package gorm

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

const (
	Select_query  sqlConditionType = 0
	Where_query   sqlConditionType = 1
	not_query     sqlConditionType = 2
	or_query      sqlConditionType = 3
	having_query  sqlConditionType = 4
	joins_query   sqlConditionType = 5
	Init_attrs    sqlConditionType = 6
	assign_attrs  sqlConditionType = 7
	preload_query sqlConditionType = 8
	Order_query   sqlConditionType = 9
	omits_query   sqlConditionType = 10
	group_query   sqlConditionType = 11
	limit_query   sqlConditionType = 12
	offset_query  sqlConditionType = 13

	IS_UNSCOPED uint16 = 0
	IS_RAW      uint16 = 1
	IS_COUNTING uint16 = 2

	HAS_SELECT          uint16 = 3
	HAS_WHERE           uint16 = 4
	HAS_NOT             uint16 = 5
	HAS_OR              uint16 = 6
	HAS_HAVING          uint16 = 7
	HAS_JOINS           uint16 = 8
	HAS_INIT            uint16 = 9
	HAS_ASSIGN          uint16 = 10
	HAS_PRELOAD         uint16 = 11
	HAS_ORDER           uint16 = 12
	HAS_OMITS           uint16 = 13
	HAS_GROUP           uint16 = 14
	HAS_OFFSET_OR_LIMIT uint16 = 15
)

// Expr generate raw SQL expression, for example:
//     DB.Model(&product).Update("price", gorm.SqlPair("price * ? + ?", 2, 100))
func SqlExpr(expression interface{}, args ...interface{}) *SqlPair {
	return &SqlPair{expression: expression, args: args}
}

//TODO : @Badu - make expr string bytes buffer, allow args to be added, allow bytes buffer to be written into
//TODO : @Badu - before doing above, benchmark bytesbuffer versus string concat
/**
var buf bytes.Buffer
var prParams []interface{}
if p.Id > 0 {
	buf.WriteString("%q:%d,")
	prParams = append(prParams, "id")
	prParams = append(prParams, p.Id)
}
buf.WriteString("%q:%q,%q:%q,%q:%t,%q:{%v}")
prParams = append(prParams, "name")
prParams = append(prParams, p.DisplayName)
prParams = append(prParams, "states")
prParams = append(prParams, p.USStates)
prParams = append(prParams, "customerPays")
prParams = append(prParams, p.AppliesToCustomer)
prParams = append(prParams, "price")
prParams = append(prParams, p.Price)
return fmt.Sprintf(buf.String(), prParams...)
*/
//TODO : @Badu - use it to build strings with multiple fmt.Sprintf calls - making one call

func (p *SqlPair) addExpressions(values ...interface{}) {
	p.args = append(p.args, values...)
}

func (p *SqlPair) strExpr() string {
	result, ok := p.expression.(string)
	if ok {
		return result
	}
	fmt.Printf("ERROR : SqlPair expression %v expected to be string. It's not!\n", p.expression)
	return ""
}

func (s *Search) getFirst(condType sqlConditionType) *SqlPair {
	s.checkInit(condType)
	//should return the number of conditions of that type
	numConditions := len(s.Conditions[condType])
	if numConditions != 1 {
		//err := fmt.Errorf("Search getFirst : %d should have exactly one item in slice, but has %d", condType, numConditions)
		//fmt.Println(err)
		//if s.con != nil {
		//	s.con.AddError(err)
		//}
		return nil
	}
	return &s.Conditions[condType][0]
}

func (s *Search) checkInit(condType sqlConditionType) {
	//create a slice of conditions for the key of map if there isn't already one
	if _, ok := s.Conditions[condType]; !ok {
		s.Conditions[condType] = make([]SqlPair, 0, 0)
	}
}

func (s *Search) Preload(schema string, values ...interface{}) *Search {
	//Note to self : order matters here : if you attempt to replace the existing item,
	//logic will break - as in many many places
	s.checkInit(preload_query)
	//overriding sql pairs within the same schema
	for i, pair := range s.Conditions[preload_query] {
		if pair.strExpr() == schema {
			//delete from slice
			s.Conditions[preload_query] = append(s.Conditions[preload_query][:i], s.Conditions[preload_query][i+1:]...)
		}
	}
	//add preload
	newPair := SqlPair{expression: schema}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.Conditions[preload_query] = append(s.Conditions[preload_query], newPair)
	s.setFlag(HAS_PRELOAD)
	return s
}

func (s *Search) addSqlCondition(condType sqlConditionType, query interface{}, values ...interface{}) {
	//TODO : @Badu - VERY IMPORTANT : check in which condition we clone the search,
	//otherwise slice will grow indefinitely ( causing memory leak :) )
	s.checkInit(condType)
	//create a new condition pair
	newPair := SqlPair{expression: query}
	newPair.addExpressions(values...)
	//add the condition pair to the slice
	s.Conditions[condType] = append(s.Conditions[condType], newPair)
}

func (s *Search) Clone() *Search {
	clone := Search{}
	clone.flags = s.flags
	//clone conditions
	clone.Conditions = make(SqlConditions)
	for key, value := range s.Conditions {
		clone.Conditions[key] = value
	}
	clone.tableName = s.tableName
	clone.Value = s.Value
	return &clone
}

func (s *Search) Wheres(wheres ...interface{}) *Search {
	if len(wheres) > 0 {
		s.addSqlCondition(Where_query, wheres[0], wheres[1:]...)
		s.setFlag(HAS_WHERE)
	}
	return s
}

func (s *Search) initialize(scope *Scope) {
	for _, pair := range s.Conditions[Where_query] {
		updatedAttrsWithValues(scope, pair.expression)
	}
	initArgs := s.getFirst(Init_attrs)
	if initArgs != nil {
		updatedAttrsWithValues(scope, initArgs.args)
	}
	args := s.getFirst(assign_attrs)
	if args != nil {
		updatedAttrsWithValues(scope, args.args)
	}
}

func (s *Search) Where(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(Where_query, query, values...)
	s.setFlag(HAS_WHERE)
	return s
}

func (s *Search) Not(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(not_query, query, values...)
	s.setFlag(HAS_NOT)
	return s
}

func (s *Search) Or(query interface{}, values ...interface{}) *Search {
	s.addSqlCondition(or_query, query, values...)
	s.setFlag(HAS_OR)
	return s
}

func (s *Search) Having(query string, values ...interface{}) *Search {
	s.addSqlCondition(having_query, query, values...)
	s.setFlag(HAS_HAVING)
	return s
}

func (s *Search) Joins(query string, values ...interface{}) *Search {
	s.addSqlCondition(joins_query, query, values...)
	s.setFlag(HAS_JOINS)
	return s
}

func (s *Search) Select(query interface{}, args ...interface{}) *Search {
	s.addSqlCondition(Select_query, query, args...)
	s.setFlag(HAS_SELECT)
	return s
}

//TODO : @Badu - do the very same where we need only one instance (aka Singleton) - like select... (where getFirst is used)
func (s *Search) Limit(limit interface{}) *Search {
	s.Conditions[limit_query] = make([]SqlPair, 0, 0)
	newPair := SqlPair{}
	newPair.addExpressions(limit)
	s.Conditions[limit_query] = append(s.Conditions[limit_query], newPair)

	s.setFlag(HAS_OFFSET_OR_LIMIT)
	return s
}

func (s *Search) Offset(offset interface{}) *Search {
	s.Conditions[offset_query] = make([]SqlPair, 0, 0)
	newPair := SqlPair{}
	newPair.addExpressions(offset)
	s.Conditions[offset_query] = append(s.Conditions[offset_query], newPair)
	s.setFlag(HAS_OFFSET_OR_LIMIT)
	return s
}

func (s *Search) Group(query string) *Search {
	s.addSqlCondition(group_query, query, nil)
	s.setFlag(HAS_GROUP)
	return s
}

func (s *Search) Attrs(attrs ...interface{}) *Search {
	result := argsToInterface(attrs...)
	if result != nil {
		s.addSqlCondition(Init_attrs, nil, result)
		s.setFlag(HAS_INIT)
	}
	return s
}

func (s *Search) Assign(attrs ...interface{}) *Search {
	result := argsToInterface(attrs...)
	if result != nil {
		s.addSqlCondition(assign_attrs, nil, result)
		s.setFlag(HAS_ASSIGN)
	}
	return s
}

func (s *Search) Table(name string) *Search {
	s.tableName = name
	return s
}

func (s *Search) Order(value interface{}, reorder ...bool) *Search {
	if len(reorder) > 0 && reorder[0] {
		//reseting existing entry
		s.Conditions[Order_query] = make([]SqlPair, 0, 0)
	}
	if value != nil {
		s.addSqlCondition(Order_query, nil, value)
		s.setFlag(HAS_ORDER)
	}
	return s
}

func (s *Search) Omit(columns ...string) *Search {
	s.checkInit(omits_query)
	//add omit
	newPair := SqlPair{}
	//transfer slices (copy) - strings to interface
	newPair.args = make([]interface{}, len(columns))
	for i, v := range columns {
		newPair.args[i] = v
	}
	//add the condition pair to the slice
	s.Conditions[omits_query] = append(s.Conditions[omits_query], newPair)
	//fmt.Printf("Omit %d elements\n", s.numConditions(omits_query))
	s.setFlag(HAS_OMITS)
	return s
}

func (s *Search) exec(scope *Scope, sql string, values ...interface{}) string {
	newPair := SqlPair{expression: sql}
	newPair.addExpressions(values...)
	genSql := s.buildWhereCondition(newPair, scope)
	return strings.TrimSuffix(strings.TrimPrefix(genSql, "("), ")")
}

func (s *Search) Exec(scope *Scope) (sql.Result, error) {
	result, err := scope.con.sqli.Exec(s.SQL, s.SQLVars...)
	if scope.Err(err) == nil {
		count, err := result.RowsAffected()
		if scope.Err(err) == nil {
			scope.con.RowsAffected = count
		}
	}
	return result, err
}

func (s *Search) Query(scope *Scope) (*sql.Rows, error) {
	rows, err := scope.con.sqli.Query(s.SQL, s.SQLVars...)
	return rows, err
}

func (s *Search) QueryRow(scope *Scope) *sql.Row {
	return scope.con.sqli.QueryRow(s.SQL, s.SQLVars...)
}

//should remain unused
func (s Search) hasFlag(value uint16) bool {
	return s.flags&(1<<value) != 0
}

func (s *Search) setFlag(value uint16) {
	s.flags = s.flags | (1 << value)
}

//should remain unused
func (s *Search) unsetFlag(value uint16) {
	s.flags = s.flags & ^(1 << value)
}

func (s *Search) isCounting() bool {
	return s.flags&(1<<IS_COUNTING) != 0
}

func (s *Search) hasSelect() bool {
	return s.flags&(1<<HAS_SELECT) != 0
}

func (s *Search) hasJoins() bool {
	return s.flags&(1<<HAS_JOINS) != 0
}

func (s *Search) hasOrder() bool {
	return s.flags&(1<<HAS_ORDER) != 0
}

func (s *Search) hasAssign() bool {
	return s.flags&(1<<HAS_ASSIGN) != 0
}

func (s *Search) hasPreload() bool {
	return s.flags&(1<<HAS_PRELOAD) != 0
}

func (s *Search) hasHaving() bool {
	return s.flags&(1<<HAS_HAVING) != 0
}

func (s *Search) hasGroup() bool {
	return s.flags&(1<<HAS_GROUP) != 0
}

func (s *Search) hasOffsetOrLimit() bool {
	return s.flags&(1<<HAS_OFFSET_OR_LIMIT) != 0
}

func (s *Search) setCounting() *Search {
	s.flags = s.flags | (1 << IS_COUNTING)
	return s
}

func (s *Search) IsRaw() bool {
	return s.flags&(1<<IS_RAW) != 0
}

func (s *Search) SetRaw() *Search {
	s.flags = s.flags | (1 << IS_RAW)
	return s
}

func (s *Search) isUnscoped() bool {
	return s.flags&(1<<IS_UNSCOPED) != 0
}

func (s *Search) setUnscoped() *Search {
	s.flags = s.flags | (1 << IS_UNSCOPED)
	return s
}

//TODO : @Badu - make field aware of "it's include or not"
func (s *Search) checkFieldIncluded(field *StructField) bool {
	for _, pair := range s.Conditions[Select_query] {
		switch strs := pair.expression.(type) {
		case string:
			if field.GetStructName() == strs || field.DBName == strs {
				return true
			}

		case []string:
			for _, o := range strs {
				if field.GetStructName() == o || field.DBName == o {
					return true
				}
			}
		}

		for _, pairArg := range pair.args {
			if field.GetStructName() == pairArg || field.DBName == pairArg {
				return true
			}
		}
	}
	return false
}

func (s *Search) checkFieldOmitted(field *StructField) bool {
	pair := s.getFirst(omits_query)
	if pair == nil {
		return false
	}
	for _, attr := range pair.args {
		if field.GetStructName() == attr || field.DBName == attr {
			//fmt.Printf("Field %q omitted\n", attr.(string))
			return true
		}
	}
	return false
}
//TODO : @Badu - maybe it's best to split this into two function - one for sqlPair and one for value (to remove recursion)
// addToVars add value as sql's vars, used to prevent SQL injection
func (s *Search) addToVars(value interface{}, dialect Dialect) string {
	if pair, ok := value.(*SqlPair); ok {
		exp := pair.strExpr()
		for _, arg := range pair.args {
			exp = strings.Replace(exp, "?", s.addToVars(arg, dialect), 1)
		}
		return exp
	}
	s.SQLVars = append(s.SQLVars, value)
	return dialect.BindVar(len(s.SQLVars))
}

func (s *Search) whereSQL(scope *Scope) string {
	var (
		str                                            string
		dialect                                        = scope.con.parent.dialect
		quotedTableName                                = scope.QuotedTableName()
		primaryConditions, andConditions, orConditions []string
	)

	if !s.isUnscoped() && scope.GetModelStruct().HasColumn("deleted_at") {
		aStr := fmt.Sprintf("%v.deleted_at IS NULL", quotedTableName)
		primaryConditions = append(primaryConditions, aStr)
	}

	if !scope.PrimaryKeyZero() {
		for _, field := range scope.PKs() {
			aStr := fmt.Sprintf("%v.%v = %v", quotedTableName, Quote(field.DBName, dialect), s.addToVars(field.Value.Interface(), dialect))
			primaryConditions = append(primaryConditions, aStr)
		}
	}

	for _, pair := range s.Conditions[Where_query] {
		if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
			andConditions = append(andConditions, aStr)
		}
	}

	for _, pair := range s.Conditions[or_query] {
		if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
			orConditions = append(orConditions, aStr)
		}
	}

	for _, pair := range s.Conditions[not_query] {
		if aStr := s.buildNotCondition(pair, scope); aStr != "" {
			andConditions = append(andConditions, aStr)
		}
	}

	orSQL := strings.Join(orConditions, " OR ")
	combinedSQL := strings.Join(andConditions, " AND ")
	if len(combinedSQL) > 0 {
		if len(orSQL) > 0 {
			combinedSQL = combinedSQL + " OR " + orSQL
		}
	} else {
		combinedSQL = orSQL
	}

	if len(primaryConditions) > 0 {
		str = "WHERE " + strings.Join(primaryConditions, " AND ")
		if len(combinedSQL) > 0 {
			str = str + " AND (" + combinedSQL + ")"
		}
	} else if len(combinedSQL) > 0 {
		str = "WHERE " + combinedSQL
	}
	return str
}

func (s *Search) buildWhereCondition(fromPair SqlPair, scope *Scope) string {
	var (
		str             string
		quotedTableName = scope.QuotedTableName()
		dialect         = scope.con.parent.dialect
		quotedPKName    = Quote(scope.PKName(), dialect)
	)

	switch expType := fromPair.expression.(type) {
	case string:
		// if string is number
		if regExpNumberMatcher.MatchString(expType) {
			return fmt.Sprintf("(%v.%v = %v)", quotedTableName, quotedPKName, s.addToVars(expType, dialect))
		} else if expType != "" {
			str = fmt.Sprintf("(%v)", expType)
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, sql.NullInt64:
		return fmt.Sprintf("(%v.%v = %v)", quotedTableName, quotedPKName, s.addToVars(expType, dialect))
	case []int, []int8, []int16, []int32, []int64, []uint, []uint8, []uint16, []uint32, []uint64, []string, []interface{}:
		str = fmt.Sprintf("(%v.%v IN (?))", quotedTableName, quotedPKName)
		//TODO : @Badu - seems really bad "work around" (boiler plate logic)
		fromPair.args = []interface{}{expType}
	case map[string]interface{}:
		var sqls []string
		for key, value := range expType {
			if value != nil {
				sqls = append(sqls, fmt.Sprintf("(%v.%v = %v)", quotedTableName, Quote(key, dialect), s.addToVars(value, dialect)))
			} else {
				sqls = append(sqls, fmt.Sprintf("(%v.%v IS NULL)", quotedTableName, Quote(key, dialect)))
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var sqls []string
		newScope := scope.NewScope(expType)
		for _, field := range newScope.Fields() {
			if !field.IsIgnored() && !field.IsBlank() {
				sqls = append(sqls, fmt.Sprintf("(%v.%v = %v)", newScope.QuotedTableName(), Quote(field.DBName, dialect), s.addToVars(field.Value.Interface(), dialect)))
			}
		}
		return strings.Join(sqls, " AND ")
	}

	for _, arg := range fromPair.args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if bytes, ok := arg.([]byte); ok {
				str = strings.Replace(str, "?", s.addToVars(bytes, dialect), 1)
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {
				var tempMarks []string
				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, s.addToVars(values.Index(i).Interface(), dialect))
				}
				str = strings.Replace(str, "?", strings.Join(tempMarks, ","), 1)
			} else {
				str = strings.Replace(str, "?", s.addToVars(SqlExpr("NULL"), dialect), 1)
			}
		default:
			if valuer, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = valuer.Value()
			}

			str = strings.Replace(str, "?", s.addToVars(arg, dialect), 1)
		}
	}
	return str
}

func (s *Search) buildNotCondition(fromPair SqlPair, scope *Scope) string {
	var (
		str             string
		notEqualSQL     string
		primaryKey      = scope.PKName()
		quotedTableName = scope.QuotedTableName()
		dialect         = scope.con.parent.dialect
	)
	switch exprType := fromPair.expression.(type) {
	case string:
		// is number
		if regExpNumberMatcher.MatchString(exprType) {
			id, _ := strconv.Atoi(exprType)
			return fmt.Sprintf("(%v <> %v)", Quote(primaryKey, dialect), id)
		} else if regExpLikeInMatcher.MatchString(exprType) {
			str = fmt.Sprintf(" NOT (%v) ", exprType)
			notEqualSQL = fmt.Sprintf("NOT (%v)", exprType)
		} else {
			str = fmt.Sprintf("(%v.%v NOT IN (?))", quotedTableName, Quote(exprType, dialect))
			notEqualSQL = fmt.Sprintf("(%v.%v <> ?)", quotedTableName, Quote(exprType, dialect))
		}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, sql.NullInt64:
		return fmt.Sprintf("(%v.%v <> %v)", quotedTableName, Quote(primaryKey, dialect), exprType)
	case []int, []int8, []int16, []int32, []int64, []uint, []uint8, []uint16, []uint32, []uint64, []string:
		if reflect.ValueOf(exprType).Len() > 0 {
			str = fmt.Sprintf("(%v.%v NOT IN (?))", quotedTableName, Quote(primaryKey, dialect))
			//TODO : @Badu - seems really bad "work around" (boiler plate logic)
			fromPair.args = []interface{}{exprType}
		}
		return ""
	case map[string]interface{}:
		var sqls []string
		for key, value := range exprType {
			if value != nil {
				sqls = append(sqls, fmt.Sprintf("(%v.%v <> %v)", quotedTableName, Quote(key, dialect), s.addToVars(value, dialect)))
			} else {
				sqls = append(sqls, fmt.Sprintf("(%v.%v IS NOT NULL)", quotedTableName, Quote(key, dialect)))
			}
		}
		return strings.Join(sqls, " AND ")
	case interface{}:
		var sqls []string
		var newScope = scope.NewScope(exprType)
		for _, field := range newScope.Fields() {
			if !field.IsBlank() {
				sqls = append(sqls, fmt.Sprintf("(%v.%v <> %v)", newScope.QuotedTableName(), Quote(field.DBName, dialect), s.addToVars(field.Value.Interface(), dialect)))
			}
		}
		return strings.Join(sqls, " AND ")
	}

	for _, arg := range fromPair.args {
		switch reflect.ValueOf(arg).Kind() {
		case reflect.Slice: // For where("id in (?)", []int64{1,2})
			if bytes, ok := arg.([]byte); ok {
				str = strings.Replace(str, "?", s.addToVars(bytes, dialect), 1)
			} else if values := reflect.ValueOf(arg); values.Len() > 0 {
				var tempMarks []string
				for i := 0; i < values.Len(); i++ {
					tempMarks = append(tempMarks, s.addToVars(values.Index(i).Interface(), dialect))
				}
				str = strings.Replace(str, "?", strings.Join(tempMarks, ","), 1)
			} else {
				str = strings.Replace(str, "?", s.addToVars(SqlExpr("NULL"), dialect), 1)
			}
		default:
			if scanner, ok := interface{}(arg).(driver.Valuer); ok {
				arg, _ = scanner.Value()
			}
			str = strings.Replace(notEqualSQL, "?", s.addToVars(arg, dialect), 1)
		}
	}
	return str
}

// CombinedConditionSql return combined condition sql
func (s *Search) combinedConditionSql(scope *Scope) string {
	dialect := scope.con.parent.dialect
	//Attention : if we don't build joinSql first, joins will fail (it's mixing up the where clauses of the joins)
	//-= creating Joins =-
	var joinConditions []string

	for _, pair := range s.Conditions[joins_query] {
		if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
			joinConditions = append(joinConditions, strings.TrimSuffix(strings.TrimPrefix(aStr, "("), ")"))
		}
	}

	joinsSql := strings.Join(joinConditions, " ") + " "
	//-= end creating Joins =-

	whereSql := s.whereSQL(scope)
	if s.IsRaw() {
		whereSql = strings.TrimSuffix(strings.TrimPrefix(whereSql, "WHERE ("), ")")
	}

	//-= creating Group =-
	groupSQL := ""
	if s.hasGroup() {
		groupSQL = " GROUP BY " + s.Conditions[group_query][0].expression.(string)
	}
	//-= end creating Group =-

	//-= creating Having =-
	havingSQL := ""
	if s.hasHaving() {
		var andConditions []string
		for _, pair := range s.Conditions[having_query] {
			if aStr := s.buildWhereCondition(pair, scope); aStr != "" {
				andConditions = append(andConditions, aStr)
			}
		}
		combinedSQL := strings.Join(andConditions, " AND ")
		if len(combinedSQL) > 0 {
			havingSQL = " HAVING " + combinedSQL
		}
	}
	//-= end creating Having =-

	//-= creating Order =-
	orderSQL := ""
	if s.hasOrder() && !s.isCounting() {
		var orders []string
		for _, orderPair := range s.Conditions[Order_query] {
			if str, ok := orderPair.args[0].(string); ok {
				orders = append(orders, QuoteIfPossible(str, dialect))
			} else if pair, ok := orderPair.args[0].(*SqlPair); ok {
				exp := pair.strExpr()
				for _, arg := range pair.args {
					exp = strings.Replace(exp, "?", s.addToVars(arg, dialect), 1)
				}
				orders = append(orders, exp)
			}
		}
		orderSQL = " ORDER BY " + strings.Join(orders, ",")
	}
	//-= end creating Order =-

	if s.hasOffsetOrLimit() {
		limitValue := -1
		offsetValue := -1

		if len(s.Conditions[limit_query]) > 0 {
			limitValue = s.Conditions[limit_query][0].args[0].(int)
		}

		if len(s.Conditions[offset_query]) > 0 {
			offsetValue = s.Conditions[offset_query][0].args[0].(int)

		}
		limitAndOffsetSQL := dialect.LimitAndOffsetSQL(limitValue, offsetValue)
		return joinsSql + whereSql + groupSQL + havingSQL + orderSQL + limitAndOffsetSQL
	}
	return joinsSql + whereSql + groupSQL + havingSQL + orderSQL

}

func (s *Search) prepareQuerySQL(scope *Scope) {
	if s.IsRaw() {
		scope.Raw(s.combinedConditionSql(scope))
	} else {
		selectSQL := ""
		if s.hasSelect() {
			fromPair := s.getFirst(Select_query)
			if fromPair == nil {
				//error has occurred in getting first item in slice
				scope.Warn(fmt.Errorf("Error : error has occurred in getting first item in slice for SELECT"))
				selectSQL = ""
			} else {
				switch value := fromPair.expression.(type) {
				case string:
					selectSQL = value
				case []string:
					selectSQL = strings.Join(value, ", ")
				}
				for _, arg := range fromPair.args {
					switch reflect.ValueOf(arg).Kind() {
					case reflect.Slice:
						values := reflect.ValueOf(arg)
						var tempMarks []string
						for i := 0; i < values.Len(); i++ {
							tempMarks = append(tempMarks, s.addToVars(values.Index(i).Interface(), scope.con.parent.dialect))
						}
						selectSQL = strings.Replace(selectSQL, "?", strings.Join(tempMarks, ","), 1)
					default:
						if valuer, ok := interface{}(arg).(driver.Valuer); ok {
							arg, _ = valuer.Value()
						}
						selectSQL = strings.Replace(selectSQL, "?", s.addToVars(arg, scope.con.parent.dialect), 1)
					}
				}
			}
		} else if s.hasJoins() {
			selectSQL = fmt.Sprintf("%v.*", scope.QuotedTableName())
		} else {
			selectSQL = "*"
		}

		scope.Raw(fmt.Sprintf("SELECT %v FROM %v %v", selectSQL, scope.QuotedTableName(), s.combinedConditionSql(scope)))
	}
}
