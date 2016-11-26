package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unsafe"
)

// IndirectValue return scope's reflect value's indirect value
func (scope *Scope) IndirectValue() reflect.Value {
	reflectValue := reflect.ValueOf(scope.Value)
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

// New create a new Scope without search information
func (scope *Scope) NewScope(value interface{}) *Scope {
	return &Scope{
		con:    newCon(scope.con),
		Search: &Search{Conditions: make(SqlConditions)},
		Value:  value}
}

////////////////////////////////////////////////////////////////////////////////
// Setter-Getters
////////////////////////////////////////////////////////////////////////////////

// Set set value by name
func (scope *Scope) Set(settingType uint64, value interface{}) *Scope {
	scope.con.instanceSet(settingType, value)
	return scope
}

// Get get setting by name
func (scope *Scope) Get(settingType uint64) (interface{}, bool) {
	return scope.con.get(settingType)
}

// InstanceSet set instance setting for current operation,
// but not for operations in callbacks,
// like saving associations callback
func (scope *Scope) InstanceSet(settingType uint64, value interface{}) *Scope {
	if scope.instanceID <= 0 {
		//gets the pointer of self and convert it to uint64 - it's unique enough, since no two scopes can share same address
		scope.instanceID = *(*uint64)(unsafe.Pointer(&scope))
	}
	return scope.Set(scope.instanceID+settingType, value)
}

// InstanceGet get instance setting from current operation
func (scope *Scope) InstanceGet(settingType uint64) (interface{}, bool) {
	if scope.instanceID <= 0 {
		//gets the pointer of self and convert it to uint64 - it's unique enough, since no two scopes can share same address
		scope.instanceID = *(*uint64)(unsafe.Pointer(&scope))
	}
	return scope.Get(scope.instanceID + settingType)
}

////////////////////////////////////////////////////////////////////////////////
// Scope DB
////////////////////////////////////////////////////////////////////////////////
// Err add error to Scope
func (scope *Scope) Err(err error) error {
	if err != nil {
		scope.con.AddError(err)
	}
	return err
}

// HasError check if there are any error
func (scope *Scope) HasError() bool {
	return scope.con.Error != nil
}

// Log print log message
func (scope *Scope) Log(v ...interface{}) {
	scope.con.log(v...)
}

func (scope *Scope) Warn(v ...interface{}) {
	scope.con.warnLog(v...)
}

// SkipLeft skip remaining callbacks
func (scope *Scope) SkipLeft() {
	scope.skipLeft = true
}

// Fields get value's fields from ModelStruct
func (scope *Scope) Fields() StructFields {
	if scope.fields == nil {
		scope.fields = scope.GetModelStruct().cloneFieldsToScope(scope.IndirectValue())
	}
	return *scope.fields
}

// was PrimaryFields() : PKs() return scope's primary fields
func (scope *Scope) PKs() StructFields {
	var fields StructFields
	for _, field := range scope.Fields() {
		if field.IsPrimaryKey() {
			fields.add(field)
		}
	}
	return fields
}

// was PrimaryField() - PK() return scope's main primary field, if defined more that one primary fields, will return the one having column name `id` or the first one
func (scope *Scope) PK() *StructField {
	primaryFieldsLen := scope.GetModelStruct().noOfPKs()
	if primaryFieldsLen > 0 {
		if primaryFieldsLen > 1 {
			if field, ok := scope.FieldByName(DEFAULT_ID_NAME); ok {
				return field
			}
		}
		//and return the first one
		return scope.PKs()[0]
	}

	//TODO : @Badu - investigate where this is called and gets the nil
	return nil
}

// was PrimaryKey() - PKName() get main primary field's db name
func (scope *Scope) PKName() string {
	if field := scope.PK(); field != nil {
		return field.DBName
	}
	//TODO : @Badu - investigate where this is called and gets the empty string
	return ""
}

// PrimaryKeyZero check main primary field's value is blank or not
func (scope *Scope) PrimaryKeyZero() bool {
	field := scope.PK()
	return field == nil || field.IsBlank()
}

// PrimaryKeyValue get the primary key's value
func (scope *Scope) PrimaryKeyValue() interface{} {
	if field := scope.PK(); field != nil && field.Value.IsValid() {
		return field.Value.Interface()
	}
	return 0
}

// FieldByName find `gorm.StructField` with field name or db name
func (scope *Scope) FieldByName(name string) (*StructField, bool) {
	var (
		dbName           = NamesMap.ToDBName(name)
		mostMatchedField *StructField
	)

	for _, field := range scope.Fields() {
		if field.GetStructName() == name || field.DBName == name {
			return field, true
		}
		if field.DBName == dbName {
			mostMatchedField = field
		}
	}
	return mostMatchedField, mostMatchedField != nil
}

// SetColumn to set the column's value, column could be field or field's name/dbname
func (scope *Scope) SetColumn(column interface{}, value interface{}) error {
	var updateAttrs = map[string]interface{}{}
	if attrs, ok := scope.InstanceGet(UPDATE_ATTRS_SETTING); ok {
		updateAttrs = attrs.(map[string]interface{})
		defer scope.InstanceSet(UPDATE_ATTRS_SETTING, updateAttrs)
	}
	//TODO : @Badu - make switch .(type)
	if field, ok := column.(*StructField); ok {
		updateAttrs[field.DBName] = value
		return field.Set(value)
	} else if name, ok := column.(string); ok {
		//TODO : @Badu - looks like Scope.FieldByName
		var (
			dbName           = NamesMap.ToDBName(name)
			mostMatchedField *StructField
		)
		for _, field := range scope.Fields() {
			if field.DBName == value {
				updateAttrs[field.DBName] = value
				return field.Set(value)
			}
			if (field.DBName == dbName) || (field.GetStructName() == name && mostMatchedField == nil) {
				mostMatchedField = field
			}
		}

		if mostMatchedField != nil {
			updateAttrs[mostMatchedField.DBName] = value
			return mostMatchedField.Set(value)
		}
	}
	//TODO : @Badu - make this error more explicit : what's column name
	return errors.New("SCOPE : could not convert column to field")
}

// CallMethod call scope value's method, if it is a slice, will call its element's method one by one
func (scope *Scope) CallMethod(methodName string) {
	if scope.Value == nil {
		return
	}

	if indirectScopeValue := scope.IndirectValue(); indirectScopeValue.Kind() == reflect.Slice {
		for i := 0; i < indirectScopeValue.Len(); i++ {
			scope.callMethod(methodName, indirectScopeValue.Index(i))
		}
	} else {
		scope.callMethod(methodName, indirectScopeValue)
	}
}

// TableName return table name
func (scope *Scope) TableName() string {
	if scope.Search != nil && scope.Search.tableName != "" {
		return scope.Search.tableName
	}
	switch tabler := scope.Value.(type) {
	case tabler:
		return tabler.TableName()
	case dbTabler:
		return tabler.TableName(scope.con)
	}
	return scope.GetModelStruct().TableName(scope.con.Model(scope.Value))
}

// QuotedTableName return quoted table name
func (scope *Scope) QuotedTableName() string {
	if scope.Search != nil && scope.Search.tableName != "" {
		if strings.Index(scope.Search.tableName, " ") != -1 {
			return scope.Search.tableName
		}
		return Quote(scope.Search.tableName, scope.con.parent.dialect)
	}

	return Quote(scope.TableName(), scope.con.parent.dialect)
}

// Raw set raw sql
func (scope *Scope) Raw(sql string) *Scope {
	scope.Search.SQL = strings.Replace(sql, "$$", "?", -1)
	return scope
}

// Exec perform generated SQL
func (scope *Scope) Exec() *Scope {
	//avoid call if we don't need to
	if scope.con.logMode == 2 {
		defer scope.trace(NowFunc())
	}
	if !scope.HasError() {
		result, err := scope.con.sqli.Exec(scope.Search.SQL, scope.Search.SQLVars...)
		if scope.Err(err) == nil {
			count, err := result.RowsAffected()
			if scope.Err(err) == nil {
				scope.con.RowsAffected = count
			}
		}
	}
	return scope
}

// Begin start a transaction
func (scope *Scope) Begin() *Scope {

	if db, ok := scope.con.sqli.(sqlDb); ok {
		//parent db implements Begin() -> call Begin()
		if tx, err := db.Begin(); err == nil {
			//TODO : @Badu - maybe the parent should do so, since it's owner of db.db
			//parent db.db implements Exec(), Prepare(), Query() and QueryRow()
			scope.con.sqli = interface{}(tx).(sqlInterf)
			scope.InstanceSet(STARTED_TX_SETTING, true)
		}
	}
	return scope
}

// CommitOrRollback commit current transaction if no error happened, otherwise will rollback it
func (scope *Scope) CommitOrRollback() *Scope {
	if _, ok := scope.InstanceGet(STARTED_TX_SETTING); ok {
		if db, ok := scope.con.sqli.(sqlTx); ok {
			if scope.HasError() {
				//orm.db implements Commit() and Rollback() -> call Rollback()
				db.Rollback()
			} else {
				//orm.db implements Commit() and Rollback() -> call Commit()
				scope.Err(db.Commit())
			}
			scope.con.sqli = scope.con.parent.sqli
		}
	}
	return scope
}

// GetModelStruct get value's model struct, relationships based on struct and tag definition
func (scope *Scope) GetModelStruct() *ModelStruct {
	var modelStruct ModelStruct
	// Scope value can't be nil
	//TODO : @Badu - why can't be nil and why we are not returning an warning/error?
	if scope.Value == nil {
		return &modelStruct
	}

	reflectType := reflect.ValueOf(scope.Value).Type()
	for reflectType.Kind() == reflect.Slice || reflectType.Kind() == reflect.Ptr {
		//dereference
		reflectType = reflectType.Elem()
	}

	if reflectType.Kind() != reflect.Struct {
		//TODO : @Badu - why we are not returning an error?
		// Scope value need to be a struct
		return &modelStruct
	}

	// Get Cached model struct
	if value := modelStructsMap.Get(reflectType); value != nil {
		return value
	}

	modelStruct.Create(reflectType, scope)

	//set cached ModelStruc
	modelStructsMap.Set(reflectType, &modelStruct)
	// ATTN : first we add it to cache map, otherwise will infinite cycle
	// build relationships
	modelStruct.processRelations(scope)

	return &modelStruct
}

////////////////////////////////////////////////////////////////////////////////
// Private Methods For *gorm.Scope
////////////////////////////////////////////////////////////////////////////////

func (scope *Scope) callMethod(methodName string, reflectValue reflect.Value) {
	// Only get address from non-pointer
	if reflectValue.CanAddr() && reflectValue.Kind() != reflect.Ptr {
		reflectValue = reflectValue.Addr()
	}

	if methodValue := reflectValue.MethodByName(methodName); methodValue.IsValid() {
		switch method := methodValue.Interface().(type) {
		case func():
			method()
		case func(*Scope):
			method(scope)
		case func(*DBCon):
			newCon := newCon(scope.con)
			method(newCon)
			scope.Err(newCon.Error)
		case func() error:
			scope.Err(method())
		case func(*Scope) error:
			scope.Err(method(scope))
		case func(*DBCon) error:
			newCon := newCon(scope.con)
			scope.Err(method(newCon))
			scope.Err(newCon.Error)
		default:
			scope.Err(fmt.Errorf("unsupported function %v", methodName))
		}
	}
}

func (scope *Scope) scan(rows *sql.Rows, columns []string, fields StructFields) {
	var (
		ignored            interface{}
		values             = make([]interface{}, len(columns))
		selectFields       StructFields
		selectedColumnsMap = map[string]int{}
		resetFields        = map[int]*StructField{}
	)

	for index, column := range columns {
		values[index] = &ignored

		selectFields = fields
		if idx, ok := selectedColumnsMap[column]; ok {
			selectFields = selectFields[idx+1:]
		}

		for fieldIndex, field := range selectFields {
			if field.DBName == column {
				if field.Value.Kind() == reflect.Ptr {
					values[index] = field.Value.Addr().Interface()
				} else {
					reflectValue := field.PtrToValue()
					reflectValue.Elem().Set(field.Value.Addr())
					values[index] = reflectValue.Interface()
					resetFields[index] = field
				}

				selectedColumnsMap[column] = fieldIndex
				//TODO :@Badu - why if it's normal we break last ? shouldn't be first?
				if field.IsNormal() {
					break
				}
			}
		}
	}

	scope.Err(rows.Scan(values...))

	for index, field := range resetFields {
		if v := reflect.ValueOf(values[index]).Elem().Elem(); v.IsValid() {
			field.Value.Set(v)
		}
	}
}

func (scope *Scope) callCallbacks(funcs ScopedFuncs) *Scope {
	for _, f := range funcs {
		//was (*f)(scope) - but IDE went balistic
		rf := *f
		rf(scope)
		if scope.skipLeft {
			break
		}
	}
	return scope
}

func (scope *Scope) updatedAttrsWithValues(value interface{}) (map[string]interface{}, bool) {
	hasUpdate := false
	if scope.IndirectValue().Kind() != reflect.Struct {
		return convertInterfaceToMap(value, false), true
	}

	results := map[string]interface{}{}

	for key, value := range convertInterfaceToMap(value, true) {
		if field, ok := scope.FieldByName(key); ok && scope.changeableField(field) {
			if _, ok := value.(*SqlPair); ok {
				hasUpdate = true
				results[field.DBName] = value
			} else {
				err := field.Set(value)
				if field.IsNormal() {
					hasUpdate = true
					if err == ErrUnaddressable {
						results[field.DBName] = value
					} else {
						results[field.DBName] = field.Value.Interface()
					}
				}
			}
		}
	}
	return results, hasUpdate
}

func (scope *Scope) row() *sql.Row {
	//avoid call if we don't need to
	if scope.con.logMode == 2 {
		defer scope.trace(NowFunc())
	}
	scope.callCallbacks(scope.con.parent.callback.rowQueries)
	scope.Search.prepareQuerySQL(scope)
	return scope.con.sqli.QueryRow(scope.Search.SQL, scope.Search.SQLVars...)
}

func (scope *Scope) rows() (*sql.Rows, error) {
	//avoid call if we don't need to
	if scope.con.logMode == 2 {
		defer scope.trace(NowFunc())
	}
	scope.callCallbacks(scope.con.parent.callback.rowQueries)
	scope.Search.prepareQuerySQL(scope)
	return scope.con.sqli.Query(scope.Search.SQL, scope.Search.SQLVars...)
}

func (scope *Scope) initialize() *Scope {
	for _, pair := range scope.Search.Conditions[Where_query] {
		scope.updatedAttrsWithValues(pair.expression)
	}
	initArgs := scope.Search.getFirst(Init_attrs)
	if initArgs != nil {
		scope.updatedAttrsWithValues(initArgs.args)
	}
	args := scope.Search.getFirst(assign_attrs)
	if args != nil {
		scope.updatedAttrsWithValues(args.args)
	}
	return scope
}

func (scope *Scope) pluck(column string, value interface{}) *Scope {
	dest := reflect.Indirect(reflect.ValueOf(value))
	scope.Search.Select(column)
	if dest.Kind() != reflect.Slice {
		scope.Err(fmt.Errorf("results should be a slice, not %s", dest.Kind()))
		return scope
	}

	rows, err := scope.rows()
	if scope.Err(err) == nil {
		defer rows.Close()
		for rows.Next() {
			elem := reflect.New(dest.Type().Elem()).Interface()
			scope.Err(rows.Scan(elem))
			dest.Set(reflect.Append(dest, reflect.ValueOf(elem).Elem()))
		}
	}
	return scope
}

func (scope *Scope) count(value interface{}) *Scope {
	if !scope.Search.hasSelect() {
		scope.Search.Select("count(*)")
	} else {
		sqlPair := scope.Search.getFirst(Select_query)
		if sqlPair == nil {
			scope.con.toLog("ERROR : search select_query should have exaclty one count")
			//error has occured in getting first item in slice
			return scope
		}
		if !regexp.MustCompile("(?i)^count(.+)$").MatchString(fmt.Sprint(sqlPair.expression)) {
			scope.Search.Select("count(*)")
		}
	}
	scope.Search.setCounting()
	scope.Err(scope.row().Scan(value))
	return scope
}

func (scope *Scope) typeName() string {
	typ := scope.IndirectValue().Type()

	for typ.Kind() == reflect.Slice || typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return typ.Name()
}

// trace print sql log
func (scope *Scope) trace(t time.Time) {
	if scope.Search.SQL != "" {
		scope.con.slog(scope.Search.SQL, t, scope.Search.SQLVars...)
	}
}

func (scope *Scope) changeableField(field *StructField) bool {
	if scope.Search.hasSelect() {
		if scope.Search.checkFieldIncluded(field) {
			return true
		}
		return false
	}

	if scope.Search.checkFieldOmitted(field) {
		return false
	}

	return true
}

func (scope *Scope) shouldSaveAssociations() bool {
	if saveAssociations, ok := scope.Get(SAVE_ASSOC_SETTING); ok {
		if v, ok := saveAssociations.(bool); ok && !v {
			return false
		}
		if v, ok := saveAssociations.(string); ok && (v == "skip" || v == "false") {
			return false
		}
	}
	return !scope.HasError()
}

func (scope *Scope) related(value interface{}, foreignKeys ...string) *Scope {
	toScope := scope.con.NewScope(value)
	//TODO : @Badu - boilerplate string
	allKeys := append(foreignKeys, toScope.typeName()+"Id", scope.typeName()+"Id")

	//because we're using it in a for, we're getting it once
	dialect := scope.con.parent.dialect

	for _, foreignKey := range allKeys {
		fromField, _ := scope.FieldByName(foreignKey)
		//fail fast - from field is nil
		if fromField == nil {
			toField, _ := toScope.FieldByName(foreignKey)
			if toField == nil {
				//fail fast - continue : both fields are nil
				continue
			}
			aStr := fmt.Sprintf("%v = ?", Quote(toField.DBName, dialect))
			scope.Err(toScope.con.Where(aStr, scope.PrimaryKeyValue()).Find(value).Error)
			return scope
		}

		relationship := fromField.Relationship
		//fail fast - relationship is nil
		if relationship == nil {
			aStr := fmt.Sprintf("%v = ?", Quote(toScope.PKName(), dialect))
			scope.Err(toScope.con.Where(aStr, fromField.Value.Interface()).Find(value).Error)
			return scope
		}
		//now, fail fast is over
		switch relationship.Kind {
		case MANY_TO_MANY:
			scope.Err(relationship.JoinTableHandler.JoinWith(relationship.JoinTableHandler, toScope.con, scope.Value).Find(value).Error)
		case BELONGS_TO:
			query := toScope.con
			for idx, foreignKey := range relationship.ForeignDBNames {
				if field, ok := scope.FieldByName(foreignKey); ok {
					query = query.Where(fmt.Sprintf("%v = ?", Quote(relationship.AssociationForeignDBNames[idx], dialect)), field.Value.Interface())
				}
			}
			scope.Err(query.Find(value).Error)
		case HAS_MANY, HAS_ONE:
			query := toScope.con
			for idx, foreignKey := range relationship.ForeignDBNames {
				if field, ok := scope.FieldByName(relationship.AssociationForeignDBNames[idx]); ok {
					query = query.Where(fmt.Sprintf("%v = ?", Quote(foreignKey, dialect)), field.Value.Interface())
				}
			}

			if relationship.PolymorphicType != "" {
				query = query.Where(fmt.Sprintf("%v = ?", Quote(relationship.PolymorphicDBName, dialect)), relationship.PolymorphicValue)
			}
			scope.Err(query.Find(value).Error)
		}
		return scope

	}

	scope.Err(fmt.Errorf("invalid association %v", foreignKeys))
	return scope
}

// getTableOptions return the table options string or an empty string if the table options does not exist
func (scope *Scope) getTableOptions() string {
	tableOptions, ok := scope.Get(TABLE_OPT_SETTING)
	if !ok {
		return ""
	}
	return tableOptions.(string)
}

func (scope *Scope) createJoinTable(field *StructField) {
	if relationship := field.Relationship; relationship != nil && relationship.JoinTableHandler != nil {
		joinTableHandler := relationship.JoinTableHandler
		joinTable := joinTableHandler.Table(scope.con)
		//because we're using it in a for, we're getting it once
		dialect := scope.con.parent.dialect

		if !dialect.HasTable(joinTable) {
			toScope := &Scope{Value: field.Interface()}

			var sqlTypes, primaryKeys []string
			for idx, fieldName := range relationship.ForeignFieldNames {
				if field, ok := scope.FieldByName(fieldName); ok {
					clonedField := field.clone()
					clonedField.unsetFlag(IS_PRIMARYKEY)
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					clonedField.SetJoinTableFK("true")
					clonedField.UnsetIsAutoIncrement()
					sqlTypes = append(sqlTypes, Quote(relationship.ForeignDBNames[idx], dialect)+" "+dialect.DataTypeOf(clonedField))
					primaryKeys = append(primaryKeys, Quote(relationship.ForeignDBNames[idx], dialect))
				}
			}

			for idx, fieldName := range relationship.AssociationForeignFieldNames {
				if field, ok := toScope.FieldByName(fieldName); ok {
					clonedField := field.clone()
					clonedField.unsetFlag(IS_PRIMARYKEY)
					//TODO : @Badu - document that you cannot use IS_JOINTABLE_FOREIGNKEY in conjunction with AUTO_INCREMENT
					clonedField.SetJoinTableFK("true")
					clonedField.UnsetIsAutoIncrement()
					sqlTypes = append(sqlTypes, Quote(relationship.AssociationForeignDBNames[idx], dialect)+" "+dialect.DataTypeOf(clonedField))
					primaryKeys = append(primaryKeys, Quote(relationship.AssociationForeignDBNames[idx], dialect))
				}
			}

			scope.Err(newCon(scope.con).Exec(fmt.Sprintf("CREATE TABLE %v (%v, PRIMARY KEY (%v)) %s", Quote(joinTable, dialect), strings.Join(sqlTypes, ","), strings.Join(primaryKeys, ","), scope.getTableOptions())).Error)
		}
		newCon(scope.con).Table(joinTable).AutoMigrate(joinTableHandler)
	}
}

func (scope *Scope) createTable() *Scope {
	var tags []string
	var primaryKeys []string
	var primaryKeyInColumnType = false
	//because we're using it in a for, we're getting it once
	dialect := scope.con.parent.dialect
	for _, field := range scope.GetModelStruct().StructFields() {
		if field.IsNormal() {
			sqlTag := dialect.DataTypeOf(field)

			// Check if the primary key constraint was specified as
			// part of the column type. If so, we can only support
			// one column as the primary key.
			//TODO : @Badu - boiler plate string
			if strings.Contains(strings.ToLower(sqlTag), "primary key") {
				primaryKeyInColumnType = true
			}

			tags = append(tags, Quote(field.DBName, dialect)+" "+sqlTag)
		}

		if field.IsPrimaryKey() {
			primaryKeys = append(primaryKeys, Quote(field.DBName, dialect))
		}
		scope.createJoinTable(field)
	}

	var primaryKeyStr string
	if len(primaryKeys) > 0 && !primaryKeyInColumnType {
		primaryKeyStr = fmt.Sprintf(", PRIMARY KEY (%v)", strings.Join(primaryKeys, ","))
	}

	scope.Raw(fmt.Sprintf("CREATE TABLE %v (%v %v) %s", scope.QuotedTableName(), strings.Join(tags, ","), primaryKeyStr, scope.getTableOptions())).Exec()

	scope.autoIndex()
	return scope
}

func (scope *Scope) dropTable() *Scope {
	scope.Raw(fmt.Sprintf("DROP TABLE %v", scope.QuotedTableName())).Exec()
	return scope
}

func (scope *Scope) modifyColumn(column string, typ string) {
	scope.Raw(fmt.Sprintf("ALTER TABLE %v MODIFY %v %v", scope.QuotedTableName(), Quote(column, scope.con.parent.dialect), typ)).Exec()
}

func (scope *Scope) dropColumn(column string) {
	scope.Raw(fmt.Sprintf("ALTER TABLE %v DROP COLUMN %v", scope.QuotedTableName(), Quote(column, scope.con.parent.dialect))).Exec()
}

func (scope *Scope) addIndex(unique bool, indexName string, column ...string) {
	dialect := scope.con.parent.dialect
	if dialect.HasIndex(scope.TableName(), indexName) {
		return
	}

	var columns []string
	for _, name := range column {
		columns = append(columns, QuoteIfPossible(name, dialect))
	}

	sqlCreate := "CREATE INDEX"
	if unique {
		sqlCreate = "CREATE UNIQUE INDEX"
	}

	scope.Raw(fmt.Sprintf("%s %v ON %v(%v) %v", sqlCreate, indexName, scope.QuotedTableName(), strings.Join(columns, ", "), scope.Search.whereSQL(scope))).Exec()
}

func (scope *Scope) addForeignKey(field string, dest string, onDelete string, onUpdate string) {
	dialect := scope.con.parent.dialect
	keyName := dialect.BuildForeignKeyName(scope.TableName(), field, dest)

	if dialect.HasForeignKey(scope.TableName(), keyName) {
		return
	}
	var query = `ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s ON DELETE %s ON UPDATE %s;`
	scope.Raw(fmt.Sprintf(query, scope.QuotedTableName(), QuoteIfPossible(keyName, dialect), QuoteIfPossible(field, dialect), dest, onDelete, onUpdate)).Exec()
}

func (scope *Scope) removeIndex(indexName string) {
	scope.con.parent.dialect.RemoveIndex(scope.TableName(), indexName)
}

func (scope *Scope) autoMigrate() *Scope {
	tableName := scope.TableName()
	quotedTableName := scope.QuotedTableName()
	//because we're using it in a for, we're getting it once
	dialect := scope.con.parent.dialect

	if !dialect.HasTable(tableName) {
		scope.createTable()
	} else {

		for _, field := range scope.GetModelStruct().StructFields() {
			if !dialect.HasColumn(tableName, field.DBName) {
				if field.IsNormal() {
					sqlTag := dialect.DataTypeOf(field)
					scope.Raw(
						fmt.Sprintf(
							"ALTER TABLE %v ADD %v %v;",
							quotedTableName,
							Quote(field.DBName, dialect),
							sqlTag)).Exec()
				}
			}
			scope.createJoinTable(field)
		}
		scope.autoIndex()
	}
	return scope
}

func (scope *Scope) autoIndex() *Scope {
	var indexes = map[string][]string{}
	var uniqueIndexes = map[string][]string{}

	for _, field := range scope.GetModelStruct().StructFields() {
		if name := field.GetSetting(INDEX); name != "" {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "INDEX" || name == "" {
					name = fmt.Sprintf("idx_%v_%v", scope.TableName(), field.DBName)
				}
				indexes[name] = append(indexes[name], field.DBName)
			}
		}

		if name := field.GetSetting(UNIQUE_INDEX); name != "" {
			names := strings.Split(name, ",")

			for _, name := range names {
				if name == "UNIQUE_INDEX" || name == "" {
					name = fmt.Sprintf("uix_%v_%v", scope.TableName(), field.DBName)
				}
				uniqueIndexes[name] = append(uniqueIndexes[name], field.DBName)
			}
		}
	}

	for name, columns := range indexes {
		newCon(scope.con).Model(scope.Value).AddIndex(name, columns...)
	}

	for name, columns := range uniqueIndexes {
		newCon(scope.con).Model(scope.Value).AddUniqueIndex(name, columns...)
	}

	return scope
}

func (scope *Scope) saveFieldAsAssociation(field *StructField) (bool, *Relationship) {
	if scope.changeableField(field) && !field.IsBlank() && !field.IsIgnored() {
		//TODO : @Badu - make field WillSaveAssociations FLAG
		if field.HasSetting(SAVE_ASSOCIATIONS) {
			set := field.GetSetting(SAVE_ASSOCIATIONS)
			if set == "false" || set == "skip" {
				return false, nil
			}
		}
		if relationship := field.Relationship; relationship != nil {
			return true, relationship
		}
	}
	return false, nil
}

////////////////////////////////////////////////////////////////////////////////
// moved here from callback_query_preload.go
////////////////////////////////////////////////////////////////////////////////
// handleHasOnePreload used to preload has one associations
func (scope *Scope) handleHasOnePreload(field *StructField, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(newCon(scope.con), conditions)
	dialect := scope.con.parent.dialect
	// find relations
	query := fmt.Sprintf(
		"%v IN (%v)",
		toQueryCondition(relation.ForeignDBNames, dialect),
		toQueryMarks(primaryKeys))
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", Quote(relation.PolymorphicDBName, dialect))
		values = append(values, relation.PolymorphicValue)
	}

	results := field.makeSlice()
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
	)

	resultsValue := reflect.ValueOf(results)
	for resultsValue.Kind() == reflect.Ptr {
		resultsValue = resultsValue.Elem()
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			for i := 0; i < resultsValue.Len(); i++ {
				result := resultsValue.Index(i)
				foreignValues := relation.ForeignFieldNames.getValueFromFields(result)
				indirectValue := indirectScopeValue.Index(j)
				for indirectValue.Kind() == reflect.Ptr {
					indirectValue = indirectValue.Elem()
				}
				if equalAsString(relation.AssociationForeignFieldNames.getValueFromFields(indirectValue), foreignValues) {
					indirectValue.FieldByName(field.GetStructName()).Set(result)
					break
				}
			}
		}
	} else {
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			scope.Err(field.Set(result))
		}
	}
}

// handleHasManyPreload used to preload has many associations
func (scope *Scope) handleHasManyPreload(field *StructField, conditions []interface{}) {
	relation := field.Relationship

	// get relations's primary keys
	primaryKeys := getColumnAsArray(relation.AssociationForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(newCon(scope.con), conditions)
	dialect := scope.con.parent.dialect
	// find relations
	query := fmt.Sprintf(
		"%v IN (%v)",
		toQueryCondition(relation.ForeignDBNames, dialect),
		toQueryMarks(primaryKeys),
	)
	values := toQueryValues(primaryKeys)
	if relation.PolymorphicType != "" {
		query += fmt.Sprintf(" AND %v = ?", Quote(relation.PolymorphicDBName, dialect))
		values = append(values, relation.PolymorphicValue)
	}

	results := field.makeSlice()
	scope.Err(preloadDB.Where(query, values...).Find(results, preloadConditions...).Error)

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
	)

	resultsValue := reflect.ValueOf(results)
	for resultsValue.Kind() == reflect.Ptr {
		resultsValue = resultsValue.Elem()
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		preloadMap := make(map[string][]reflect.Value)
		for i := 0; i < resultsValue.Len(); i++ {
			result := resultsValue.Index(i)
			foreignValues := relation.ForeignFieldNames.getValueFromFields(result)
			preloadMap[toString(foreignValues)] = append(preloadMap[toString(foreignValues)], result)
		}

		for j := 0; j < indirectScopeValue.Len(); j++ {
			reflectValue := indirectScopeValue.Index(j)
			for reflectValue.Kind() == reflect.Ptr {
				reflectValue = reflectValue.Elem()
			}
			objectRealValue := relation.AssociationForeignFieldNames.getValueFromFields(reflectValue)
			f := reflectValue.FieldByName(field.GetStructName())
			if results, ok := preloadMap[toString(objectRealValue)]; ok {
				f.Set(reflect.Append(f, results...))
			} else {
				f.Set(reflect.MakeSlice(f.Type(), 0, 0))
			}
		}
	} else {
		scope.Err(field.Set(resultsValue))
	}
}

// handleBelongsToPreload used to preload belongs to associations
func (scope *Scope) handleBelongsToPreload(field *StructField, conditions []interface{}) {
	indirectScopeValue := scope.IndirectValue()
	relation := field.Relationship

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(newCon(scope.con), conditions)

	// get relations's primary keys
	primaryKeys := getColumnAsArray(relation.ForeignFieldNames, scope.Value)
	if len(primaryKeys) == 0 {
		return
	}
	dialect := scope.con.parent.dialect
	// find relations
	results := field.makeSlice()
	scope.Err(
		preloadDB.Where(
			fmt.Sprintf(
				"%v IN (%v)",
				toQueryCondition(relation.AssociationForeignDBNames, dialect),
				toQueryMarks(primaryKeys)),
			toQueryValues(primaryKeys)...,
		).
			Find(results, preloadConditions...).
			Error)

	// assign find results

	resultsValue := reflect.ValueOf(results)
	for resultsValue.Kind() == reflect.Ptr {
		resultsValue = resultsValue.Elem()
	}

	for i := 0; i < resultsValue.Len(); i++ {
		result := resultsValue.Index(i)
		if indirectScopeValue.Kind() == reflect.Slice {
			value := relation.AssociationForeignFieldNames.getValueFromFields(result)
			for j := 0; j < indirectScopeValue.Len(); j++ {
				reflectValue := indirectScopeValue.Index(j)
				for reflectValue.Kind() == reflect.Ptr {
					reflectValue = reflectValue.Elem()
				}
				if equalAsString(relation.ForeignFieldNames.getValueFromFields(reflectValue), value) {
					reflectValue.FieldByName(field.GetStructName()).Set(result)
				}
			}
		} else {
			scope.Err(field.Set(result))
		}
	}
}

// handleManyToManyPreload used to preload many to many associations
func (scope *Scope) handleManyToManyPreload(field *StructField, conditions []interface{}) {
	var (
		relation         = field.Relationship
		joinTableHandler = relation.JoinTableHandler
		fieldType, isPtr = field.Type, field.hasFlag(IS_POINTER)
		foreignKeyValue  interface{}
		foreignKeyType   = reflect.ValueOf(&foreignKeyValue).Type()
		linkHash         = map[string][]reflect.Value{}
	)

	var sourceKeys = []string{}
	for _, key := range joinTableHandler.SourceForeignKeys() {
		sourceKeys = append(sourceKeys, key.DBName)
	}

	// preload conditions
	preloadDB, preloadConditions := generatePreloadDBWithConditions(newCon(scope.con), conditions)

	// generate query with join table
	newScope := scope.NewScope(reflect.New(fieldType).Interface())
	preloadDB = preloadDB.Table(newScope.TableName()).Model(newScope.Value).Select("*")
	preloadDB = joinTableHandler.JoinWith(joinTableHandler, preloadDB, scope.Value)

	// preload inline conditions
	if len(preloadConditions) > 0 {
		preloadDB = preloadDB.Where(preloadConditions[0], preloadConditions[1:]...)
	}

	rows, err := preloadDB.Rows()

	if scope.Err(err) != nil {
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	for rows.Next() {
		var (
			elem   = reflect.New(fieldType).Elem()
			fields = scope.NewScope(elem.Addr().Interface()).Fields()
		)

		// register foreign keys in join tables
		var joinTableFields StructFields
		for _, sourceKey := range sourceKeys {
			joinTableFields.add(
				&StructField{
					DBName: sourceKey,
					Value:  reflect.New(foreignKeyType).Elem(),
					flags:  0 | (1 << IS_NORMAL),
				})
		}

		scope.scan(rows, columns, append(fields, joinTableFields...))

		var foreignKeys = make([]interface{}, len(sourceKeys))
		// generate hashed forkey keys in join table
		for idx, joinTableField := range joinTableFields {
			if !joinTableField.Value.IsNil() {
				foreignKeys[idx] = joinTableField.Value.Elem().Interface()
			}
		}
		hashedSourceKeys := toString(foreignKeys)

		if isPtr {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem.Addr())
		} else {
			linkHash[hashedSourceKeys] = append(linkHash[hashedSourceKeys], elem)
		}
	}

	// assign find results
	var (
		indirectScopeValue = scope.IndirectValue()
		fieldsSourceMap    = map[string][]reflect.Value{}
		foreignFieldNames  = StrSlice{}
	)

	for _, dbName := range relation.ForeignFieldNames {
		if field, ok := scope.FieldByName(dbName); ok {
			foreignFieldNames.add(field.GetStructName())
		}
	}

	if indirectScopeValue.Kind() == reflect.Slice {
		for j := 0; j < indirectScopeValue.Len(); j++ {
			reflectValue := indirectScopeValue.Index(j)
			for reflectValue.Kind() == reflect.Ptr {
				reflectValue = reflectValue.Elem()
			}
			key := toString(foreignFieldNames.getValueFromFields(reflectValue))
			fieldsSourceMap[key] = append(fieldsSourceMap[key], reflectValue.FieldByName(field.GetStructName()))
		}
	} else if indirectScopeValue.IsValid() {
		key := toString(foreignFieldNames.getValueFromFields(indirectScopeValue))
		fieldsSourceMap[key] = append(fieldsSourceMap[key], indirectScopeValue.FieldByName(field.GetStructName()))
	}
	for source, link := range linkHash {
		for i, field := range fieldsSourceMap[source] {
			//If not 0 this means Value is a pointer and we already added preloaded models to it
			if fieldsSourceMap[source][i].Len() != 0 {
				continue
			}
			field.Set(reflect.Append(fieldsSourceMap[source][i], link...))
		}

	}
}

////////////////////////////////////////////////////////////////////////////////
// moved here from callback functions
////////////////////////////////////////////////////////////////////////////////

//============================================
//Callback create functions
//============================================
// beforeCreateCallback will invoke `BeforeSave`, `BeforeCreate` method before creating
func (scope *Scope) beforeCreateCallback() {
	if !scope.HasError() {
		scope.CallMethod("BeforeSave")
	}
	if !scope.HasError() {
		scope.CallMethod("BeforeCreate")
	}
}

// updateTimeStampForCreateCallback will set `CreatedAt`, `UpdatedAt` when creating
func (scope *Scope) updateTimeStampForCreateCallback() {
	if !scope.HasError() {
		now := NowFunc()
		scope.SetColumn("CreatedAt", now)
		scope.SetColumn("UpdatedAt", now)
	}
}

// createCallback the callback used to insert data into database
func (scope *Scope) createCallback() {
	if !scope.HasError() {
		//avoid call if we don't need to
		if scope.con.logMode == 2 {
			defer scope.trace(NowFunc())
		}
		var (
			columns, placeholders        StrSlice
			blankColumnsWithDefaultValue StrSlice
			//because we're using it in a for, we're getting it once
			dialect = scope.con.parent.dialect
		)

		for _, field := range scope.Fields() {
			if scope.changeableField(field) {
				if field.IsNormal() {
					if field.IsBlank() && field.HasDefaultValue() {
						blankColumnsWithDefaultValue.add(Quote(field.DBName, dialect))
						scope.InstanceSet(BLANK_COLS_DEFAULT_SETTING, blankColumnsWithDefaultValue)
					} else if !field.IsPrimaryKey() || !field.IsBlank() {
						columns.add(Quote(field.DBName, dialect))
						placeholders.add(scope.Search.addToVars(field.Value.Interface(), dialect))
					}
				} else if field.Relationship != nil && field.Relationship.Kind == BELONGS_TO {
					for _, foreignKey := range field.Relationship.ForeignDBNames {
						if foreignField, ok := scope.FieldByName(foreignKey); ok &&
							!scope.changeableField(foreignField) {
							columns.add(Quote(foreignField.DBName, dialect))
							placeholders.add(scope.Search.addToVars(foreignField.Value.Interface(), dialect))
						}
					}
				}
			}
		}

		var (
			returningColumn = "*"
			quotedTableName = scope.QuotedTableName()
			primaryField    = scope.PK()
			extraOption     string
		)

		if str, ok := scope.Get(INSERT_OPT_SETTING); ok {
			extraOption = fmt.Sprint(str)
		}

		if primaryField != nil {
			returningColumn = Quote(primaryField.DBName, dialect)
		}

		lastInsertIDReturningSuffix := dialect.LastInsertIDReturningSuffix(quotedTableName, returningColumn)

		if len(columns) == 0 {
			scope.Raw(fmt.Sprintf(
				"INSERT INTO %v DEFAULT VALUES%v%v",
				quotedTableName,
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		} else {
			scope.Raw(fmt.Sprintf(
				"INSERT INTO %v (%v) VALUES (%v)%v%v",
				scope.QuotedTableName(),
				strings.Join(columns, ","),
				strings.Join(placeholders, ","),
				addExtraSpaceIfExist(extraOption),
				addExtraSpaceIfExist(lastInsertIDReturningSuffix),
			))
		}

		// execute create sql
		if lastInsertIDReturningSuffix == "" || primaryField == nil {
			if result, err := scope.con.sqli.Exec(scope.Search.SQL, scope.Search.SQLVars...); scope.Err(err) == nil {
				// set rows affected count
				scope.con.RowsAffected, _ = result.RowsAffected()

				// set primary value to primary field
				if primaryField != nil && primaryField.IsBlank() {
					if primaryValue, err := result.LastInsertId(); scope.Err(err) == nil {
						scope.Err(primaryField.Set(primaryValue))
					}
				}
			}
		} else {
			if err := scope.con.sqli.QueryRow(scope.Search.SQL, scope.Search.SQLVars...).
				Scan(primaryField.Value.Addr().Interface()); scope.Err(err) == nil {
				primaryField.unsetFlag(IS_BLANK)
				scope.con.RowsAffected = 1
			}
		}
	}
}

// forceReloadAfterCreateCallback will reload columns that having default value, and set it back to current object
func (scope *Scope) forceReloadAfterCreateCallback() {
	if blankColumnsWithDefaultValue, ok := scope.InstanceGet(BLANK_COLS_DEFAULT_SETTING); ok {
		sSlice, yes := blankColumnsWithDefaultValue.(StrSlice)
		if !yes {
			fmt.Errorf("ERROR in forceReloadAfterCreateCallback : blankColumnsWithDefaultValue IS NOT StrSlice!\n")
		}
		db := newCon(scope.con).Table(scope.TableName()).Select(sSlice.slice())
		for _, field := range scope.Fields() {
			if field.IsPrimaryKey() && !field.IsBlank() {
				db = db.Where(fmt.Sprintf("%v = ?", field.DBName), field.Value.Interface())
			}
		}

		db.Scan(scope.Value)
	}
}

// afterCreateCallback will invoke `AfterCreate`, `AfterSave` method after creating
func (scope *Scope) afterCreateCallback() {
	if !scope.HasError() {
		scope.CallMethod("AfterCreate")
	}
	if !scope.HasError() {
		scope.CallMethod("AfterSave")
	}
}

//============================================
// Callback save functions
//============================================
func (scope *Scope) saveBeforeAssociationsCallback() {
	if !scope.shouldSaveAssociations() {
		return
	}
	for _, field := range scope.Fields() {
		if scope.changeableField(field) && !field.IsBlank() && !field.IsIgnored() {
			if ok, relationship := scope.saveFieldAsAssociation(field); ok && relationship.Kind == BELONGS_TO {
				fieldValue := field.Value.Addr().Interface()
				scope.Err(newCon(scope.con).Save(fieldValue).Error)
				if relationship.ForeignFieldNames.len() != 0 {
					// set value's foreign key
					for idx, fieldName := range relationship.ForeignFieldNames {
						associationForeignName := relationship.AssociationForeignDBNames[idx]
						if foreignField, ok := scope.NewScope(fieldValue).FieldByName(associationForeignName); ok {
							scope.Err(scope.SetColumn(fieldName, foreignField.Value.Interface()))
						}
					}
				}
			}
		}
	}
}

func (scope *Scope) saveAfterAssociationsCallback() {
	if !scope.shouldSaveAssociations() {
		return
	}
	for _, field := range scope.Fields() {
		if scope.changeableField(field) && !field.IsBlank() && !field.IsIgnored() {
			//Attention : relationship.Kind <= HAS_ONE
			if ok, relationship := scope.saveFieldAsAssociation(field); ok && relationship.Kind <= HAS_ONE {
				value := field.Value

				switch value.Kind() {
				case reflect.Slice:
					for i := 0; i < value.Len(); i++ {
						//TODO : @Badu - cloneCon without copy, then NewScope which clone's con - but with copy
						newCon := newCon(scope.con)
						elem := value.Index(i).Addr().Interface()
						newScope := newCon.NewScope(elem)

						if relationship.JoinTableHandler == nil && relationship.ForeignFieldNames.len() != 0 {
							for idx, fieldName := range relationship.ForeignFieldNames {
								associationForeignName := relationship.AssociationForeignDBNames[idx]
								if f, ok := scope.FieldByName(associationForeignName); ok {
									scope.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
								}
							}
						}

						if relationship.PolymorphicType != "" {
							scope.Err(
								newScope.SetColumn(
									relationship.PolymorphicType,
									relationship.PolymorphicValue))
						}

						scope.Err(newCon.Save(elem).Error)

						if joinTableHandler := relationship.JoinTableHandler; joinTableHandler != nil {
							scope.Err(joinTableHandler.Add(joinTableHandler, newCon, scope.Value, newScope.Value))
						}
					}
				default:
					elem := value.Addr().Interface()
					newScope := scope.NewScope(elem)
					if relationship.ForeignFieldNames.len() != 0 {
						for idx, fieldName := range relationship.ForeignFieldNames {
							associationForeignName := relationship.AssociationForeignDBNames[idx]
							if f, ok := scope.FieldByName(associationForeignName); ok {
								scope.Err(newScope.SetColumn(fieldName, f.Value.Interface()))
							}
						}
					}

					if relationship.PolymorphicType != "" {
						scope.Err(newScope.SetColumn(relationship.PolymorphicType, relationship.PolymorphicValue))
					}
					scope.Err(newCon(scope.con).Save(elem).Error)
				}
			}
		}
	}
}

//============================================
// Callback update functions
//============================================
// assignUpdatingAttributesCallback assign updating attributes to model
func (scope *Scope) assignUpdatingAttributesCallback() {
	if attrs, ok := scope.InstanceGet(UPDATE_INTERF_SETTING); ok {
		if updateMaps, hasUpdate := scope.updatedAttrsWithValues(attrs); hasUpdate {
			scope.InstanceSet(UPDATE_ATTRS_SETTING, updateMaps)
		} else {
			scope.SkipLeft()
		}
	}
}

// beforeUpdateCallback will invoke `BeforeSave`, `BeforeUpdate` method before updating
func (scope *Scope) beforeUpdateCallback() {
	if _, ok := scope.Get(UPDATE_COLUMN_SETTING); !ok {
		if !scope.HasError() {
			scope.CallMethod("BeforeSave")
		}
		if !scope.HasError() {
			scope.CallMethod("BeforeUpdate")
		}
	}
}

// updateTimeStampForUpdateCallback will set `UpdatedAt` when updating
func (scope *Scope) updateTimeStampForUpdateCallback() {
	if _, ok := scope.Get(UPDATE_COLUMN_SETTING); !ok {
		scope.SetColumn("UpdatedAt", NowFunc())
	}
}

// updateCallback the callback used to update data to database
func (scope *Scope) updateCallback() {
	if !scope.HasError() {
		var sqls []string

		//because we're using it in a for, we're getting it once
		scopeDialect := scope.con.parent.dialect

		if updateAttrs, ok := scope.InstanceGet(UPDATE_ATTRS_SETTING); ok {
			for column, value := range updateAttrs.(map[string]interface{}) {
				sqls = append(sqls, fmt.Sprintf("%v = %v", Quote(column, scopeDialect), scope.Search.addToVars(value, scopeDialect)))
			}
		} else {
			for _, field := range scope.Fields() {
				if scope.changeableField(field) {
					if !field.IsPrimaryKey() && field.IsNormal() {
						sqls = append(sqls,
							fmt.Sprintf(
								"%v = %v",
								Quote(field.DBName, scopeDialect),
								scope.Search.addToVars(field.Value.Interface(), scopeDialect)))
					} else if relationship := field.Relationship; relationship != nil && relationship.Kind == BELONGS_TO {
						for _, foreignKey := range relationship.ForeignDBNames {
							if foreignField, ok := scope.FieldByName(foreignKey); ok &&
								!scope.changeableField(foreignField) {
								sqls = append(sqls,
									fmt.Sprintf(
										"%v = %v",
										Quote(foreignField.DBName, scopeDialect),
										scope.Search.addToVars(foreignField.Value.Interface(), scopeDialect)))
							}
						}
					}
				}
			}
		}

		var extraOption string
		if str, ok := scope.Get(UPDATE_OPT_SETTING); ok {
			extraOption = fmt.Sprint(str)
		}

		if len(sqls) > 0 {
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET %v%v%v",
				scope.QuotedTableName(),
				strings.Join(sqls, ", "),
				addExtraSpaceIfExist(scope.Search.combinedConditionSql(scope)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
}

// afterUpdateCallback will invoke `AfterUpdate`, `AfterSave` method after updating
func (scope *Scope) afterUpdateCallback() {
	if _, ok := scope.Get(UPDATE_COLUMN_SETTING); !ok {
		if !scope.HasError() {
			scope.CallMethod("AfterUpdate")
		}
		if !scope.HasError() {
			scope.CallMethod("AfterSave")
		}
	}
}

//============================================
// Callback query functions
//============================================
// queryCallback used to query data from database
func (scope *Scope) queryCallback() {
	//avoid call if we don't need to
	if scope.con.logMode == 2 {
		defer scope.trace(NowFunc())
	}
	var (
		isSlice, isPtr bool
		resultType     reflect.Type
		results        = scope.IndirectValue()
		dialect        = scope.con.parent.dialect
	)

	if orderBy, ok := scope.Get(ORDER_BY_PK_SETTING); ok {
		if primaryField := scope.PK(); primaryField != nil {
			scope.Search.Order(fmt.Sprintf("%v.%v %v", scope.QuotedTableName(), Quote(primaryField.DBName, dialect), orderBy))
		}
	}

	if value, ok := scope.Get(QUERY_DEST_SETTING); ok {
		results = reflect.Indirect(reflect.ValueOf(value))
	}

	if kind := results.Kind(); kind == reflect.Slice {
		isSlice = true
		resultType = results.Type().Elem()
		results.Set(reflect.MakeSlice(results.Type(), 0, 0))

		if resultType.Kind() == reflect.Ptr {
			isPtr = true
			resultType = resultType.Elem()
		}
	} else if kind != reflect.Struct {
		scope.Err(errors.New("SCOPE : unsupported destination, should be slice or struct"))
		return
	}

	scope.Search.prepareQuerySQL(scope)

	if !scope.HasError() {
		scope.con.RowsAffected = 0
		if str, ok := scope.Get(QUERY_OPT_SETTING); ok {
			scope.Search.SQL += addExtraSpaceIfExist(fmt.Sprint(str))
		}

		if rows, err := scope.con.sqli.Query(scope.Search.SQL, scope.Search.SQLVars...); scope.Err(err) == nil {
			defer rows.Close()

			columns, _ := rows.Columns()
			for rows.Next() {
				scope.con.RowsAffected++

				elem := results
				if isSlice {
					elem = reflect.New(resultType).Elem()
				}

				scope.scan(rows, columns, scope.NewScope(elem.Addr().Interface()).Fields())

				if isSlice {
					if isPtr {
						results.Set(reflect.Append(results, elem.Addr()))
					} else {
						results.Set(reflect.Append(results, elem))
					}
				}
			}

			if scope.con.RowsAffected == 0 && !isSlice {
				scope.Err(ErrRecordNotFound)
			}
		}
	}
}

// afterQueryCallback will invoke `AfterFind` method after querying
func (scope *Scope) afterQueryCallback() {
	if !scope.HasError() {
		scope.CallMethod("AfterFind")
	}
}

//============================================
// Callback query preload function
//============================================
// preloadCallback used to preload associations
func (scope *Scope) preloadCallback() {
	if !scope.Search.hasPreload() || scope.HasError() {
		return
	}

	var (
		preloadedMap1 = map[string]bool{}
		fields1       = scope.Fields()
	)

	for _, sqlPair := range scope.Search.Conditions[preload_query] {
		var (
			preloadFields = strings.Split(sqlPair.strExpr(), ".")
			currentScope  = scope
			currentFields = fields1
		)

		for idx, preloadField := range preloadFields {
			var currentPreloadConditions []interface{}
			//there is no next level
			if currentScope == nil {
				continue
			}

			// if not preloaded
			if preloadKey := strings.Join(preloadFields[:idx+1], "."); !preloadedMap1[preloadKey] {

				// assign search conditions to last preload
				if idx == len(preloadFields)-1 {
					currentPreloadConditions = sqlPair.args
				}

				for _, field := range currentFields {
					if field.GetStructName() != preloadField || field.Relationship == nil {
						continue
					}

					switch field.Relationship.Kind {
					case HAS_ONE:
						currentScope.handleHasOnePreload(field, currentPreloadConditions)
					case HAS_MANY:
						currentScope.handleHasManyPreload(field, currentPreloadConditions)
					case BELONGS_TO:
						currentScope.handleBelongsToPreload(field, currentPreloadConditions)
					case MANY_TO_MANY:
						currentScope.handleManyToManyPreload(field, currentPreloadConditions)
					default:
						scope.Err(errors.New("SCOPE : unsupported relation"))
					}

					preloadedMap1[preloadKey] = true
					break
				}

				if !preloadedMap1[preloadKey] {
					scope.Err(
						fmt.Errorf("can't preload field %s for %s",
							preloadField,
							currentScope.GetModelStruct().ModelType))
					return
				}
			}

			// preload next level
			if idx < len(preloadFields)-1 {
				//if preloadField is struct or slice, we need to get it's scope
				currentScope = getColumnAsScope(preloadField, currentScope)

				if currentScope != nil {
					currentFields = currentScope.Fields()
				}
			}
		}
	}
}

//============================================
// Callback delete functions
//============================================
// beforeDeleteCallback will invoke `BeforeDelete` method before deleting
func (scope *Scope) beforeDeleteCallback() {
	if !scope.HasError() {
		scope.CallMethod("BeforeDelete")
	}
}

// deleteCallback used to delete data from database or set deleted_at to current time (when using with soft delete)
func (scope *Scope) deleteCallback() {
	if !scope.HasError() {
		var extraOption string
		if str, ok := scope.Get(DELETE_OPT_SETTING); ok {
			extraOption = fmt.Sprint(str)
		}

		if !scope.Search.isUnscoped() && scope.GetModelStruct().HasColumn("DeletedAt") {
			scope.Raw(fmt.Sprintf(
				"UPDATE %v SET deleted_at=%v%v%v",
				scope.QuotedTableName(),
				scope.Search.addToVars(NowFunc(), scope.con.parent.dialect),
				addExtraSpaceIfExist(scope.Search.combinedConditionSql(scope)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		} else {
			scope.Raw(fmt.Sprintf(
				"DELETE FROM %v%v%v",
				scope.QuotedTableName(),
				addExtraSpaceIfExist(scope.Search.combinedConditionSql(scope)),
				addExtraSpaceIfExist(extraOption),
			)).Exec()
		}
	}
}

// afterDeleteCallback will invoke `AfterDelete` method after deleting
func (scope *Scope) afterDeleteCallback() {
	if !scope.HasError() {
		scope.CallMethod("AfterDelete")
	}
}
