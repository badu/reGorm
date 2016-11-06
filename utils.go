package gorm

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

//============================================
// Dialect functions
//============================================

// RegisterDialect register new dialect
func RegisterDialect(name string, dialect Dialect) {
	dialectsMap[name] = dialect
}

// ParseFieldStructForDialect parse field struct for dialect
func ParseFieldStructForDialect(field *StructField) (fieldValue reflect.Value, sqlType string, size int, additionalType string) {
	// Get redirected field type
	var reflectType = field.Struct.Type
	for reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Get redirected field value
	fieldValue = reflect.Indirect(reflect.New(reflectType))

	// Get scanner's real value
	var getScannerValue func(reflect.Value)
	getScannerValue = func(value reflect.Value) {
		fieldValue = value
		if _, isScanner := reflect.New(fieldValue.Type()).Interface().(sql.Scanner); isScanner && fieldValue.Kind() == reflect.Struct {
			getScannerValue(fieldValue.Field(0))
		}
	}
	getScannerValue(fieldValue)

	// Default Size
	if num := field.GetSetting(SIZE); num != "" {
		size, _ = strconv.Atoi(num)
	} else {
		size = 255
	}

	//TODO : @Badu - what if the settings below are empty?
	// Default type from tag setting
	additionalType = field.GetSetting(NOT_NULL) + " " + field.GetSetting(UNIQUE)
	if value := field.GetSetting(DEFAULT); value != "" {
		additionalType = additionalType + " DEFAULT " + value
	}

	return fieldValue, field.GetSetting(TYPE), size, strings.TrimSpace(additionalType)
}

func isByteArrayOrSlice(value reflect.Value) bool {
	return (value.Kind() == reflect.Array || value.Kind() == reflect.Slice) && value.Type().Elem() == reflect.TypeOf(uint8(0))
}

func isUUID(value reflect.Value) bool {
	if value.Kind() != reflect.Array || value.Type().Len() != 16 {
		return false
	}
	typename := value.Type().Name()
	lower := strings.ToLower(typename)
	return "uuid" == lower || "guid" == lower
}

//============================================
// Other utils functions
//============================================

func newSafeMap() *safeMap {
	return &safeMap{l: new(sync.RWMutex), m: make(map[string]string)}
}

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func convertInterfaceToMap(values interface{}, withIgnoredField bool) map[string]interface{} {
	var attrs = map[string]interface{}{}

	switch value := values.(type) {
	case map[string]interface{}:
		return value
	case []interface{}:
		for _, v := range value {
			for key, value := range convertInterfaceToMap(v, withIgnoredField) {
				attrs[key] = value
			}
		}
	case interface{}:
		reflectValue := reflect.ValueOf(values)

		switch reflectValue.Kind() {
		case reflect.Map:
			for _, key := range reflectValue.MapKeys() {
				attrs[ToDBName(key.Interface().(string))] = reflectValue.MapIndex(key).Interface()
			}
		default:
			for _, field := range (&Scope{Value: values}).Fields() {
				if !field.IsBlank && (withIgnoredField || !field.IsIgnored) {
					attrs[field.DBName] = field.Value.Interface()
				}
			}
		}
	}
	return attrs
}

//TODO : @Badu - seems to be called everywhere! Optimize, cache or get rid of it!
// ToDBName convert string to db name
func ToDBName(name string) string {
	if v := smap.Get(name); v != "" {
		return v
	}

	if name == "" {
		return ""
	}

	var (
		value                        = commonInitialismsReplacer.Replace(name)
		buf                          = bytes.NewBufferString("")
		lastCase, currCase, nextCase strCase
	)

	for i, v := range value[:len(value)-1] {
		nextCase = strCase(value[i+1] >= 'A' && value[i+1] <= 'Z')
		if i > 0 {
			if currCase == upper {
				if lastCase == upper && nextCase == upper {
					buf.WriteRune(v)
				} else {
					if value[i-1] != '_' && value[i+1] != '_' {
						buf.WriteRune('_')
					}
					buf.WriteRune(v)
				}
			} else {
				buf.WriteRune(v)
			}
		} else {
			currCase = upper
			buf.WriteRune(v)
		}
		lastCase = currCase
		currCase = nextCase
	}

	buf.WriteByte(value[len(value)-1])

	s := strings.ToLower(buf.String())
	smap.Set(name, s)
	return s
}

// Expr generate raw SQL expression, for example:
//     DB.Model(&product).Update("price", gorm.Expr("price * ? + ?", 2, 100))
func Expr(expression string, args ...interface{}) *expr {
	return &expr{expr: expression, args: args}
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func toQueryMarks(primaryValues [][]interface{}) string {
	var results []string

	for _, primaryValue := range primaryValues {
		var marks []string
		for range primaryValue {
			marks = append(marks, "?")
		}

		if len(marks) > 1 {
			results = append(results, fmt.Sprintf("(%v)", strings.Join(marks, ",")))
		} else {
			results = append(results, strings.Join(marks, ""))
		}
	}
	return strings.Join(results, ",")
}

func toQueryCondition(scope *Scope, columns []string) string {
	var newColumns []string
	for _, column := range columns {
		newColumns = append(newColumns, scope.Quote(column))
	}

	if len(columns) > 1 {
		return fmt.Sprintf("(%v)", strings.Join(newColumns, ","))
	}
	return strings.Join(newColumns, ",")
}

func toQueryValues(values [][]interface{}) (results []interface{}) {
	for _, value := range values {
		for _, v := range value {
			results = append(results, v)
		}
	}
	return
}

func fileWithLineNum() string {
	for i := 2; i < 15; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok && (!regexp.MustCompile(`jinzhu/gorm/.*.go`).MatchString(file) || regexp.MustCompile(`jinzhu/gorm/.*test.go`).MatchString(file)) {
			return fmt.Sprintf("%v:%v", file, line)
		}
	}
	return ""
}

func toSearchableMap(attrs ...interface{}) (result interface{}) {
	if len(attrs) > 1 {
		if str, ok := attrs[0].(string); ok {
			result = map[string]interface{}{str: attrs[1]}
		}
	} else if len(attrs) == 1 {
		if attr, ok := attrs[0].(map[string]interface{}); ok {
			result = attr
		}

		if attr, ok := attrs[0].(interface{}); ok {
			result = attr
		}
	}
	return
}

func equalAsString(a interface{}, b interface{}) bool {
	return toString(a) == toString(b)
}

func toString(str interface{}) string {
	if values, ok := str.([]interface{}); ok {
		var results []string
		for _, value := range values {
			results = append(results, toString(value))
		}
		return strings.Join(results, "_")
	} else if strBytes, ok := str.([]byte); ok {
		return string(strBytes)
	} else if reflectValue := reflect.Indirect(reflect.ValueOf(str)); reflectValue.IsValid() {
		return fmt.Sprintf("%v", reflectValue.Interface())
	}
	return ""
}

func makeSlice(elemType reflect.Type) interface{} {
	if elemType.Kind() == reflect.Slice {
		elemType = elemType.Elem()
	}
	sliceType := reflect.SliceOf(elemType)
	slice := reflect.New(sliceType)
	slice.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))
	return slice.Interface()
}

// getValueFromFields return given fields's value
func getValueFromFields(value reflect.Value, fieldNames []string) (results []interface{}) {
	// If value is a nil pointer, Indirect returns a zero Value!
	// Therefor we need to check for a zero value,
	// as FieldByName could panic
	if indirectValue := reflect.Indirect(value); indirectValue.IsValid() {
		for _, fieldName := range fieldNames {
			if fieldValue := indirectValue.FieldByName(fieldName); fieldValue.IsValid() {
				result := fieldValue.Interface()
				if r, ok := result.(driver.Valuer); ok {
					result, _ = r.Value()
				}
				results = append(results, result)
			}
		}
	}
	return
}

func addExtraSpaceIfExist(str string) string {
	if str != "" {
		return " " + str
	}
	return ""
}

// Open initialize a new db connection, need to import driver first, e.g:
//
//     import _ "github.com/go-sql-driver/mysql"
//     func main() {
//       db, err := gorm.Open("mysql", "user:password@/dbname?charset=utf8&parseTime=True&loc=Local")
//     }
// GORM has wrapped some drivers, for easier to remember driver's import path, so you could import the mysql driver with
//    import _ "github.com/badu/gorm/dialects/mysql"
//    // import _ "github.com/badu/gorm/dialects/postgres"
//    // import _ "github.com/badu/gorm/dialects/sqlite"
//    // import _ "github.com/badu/gorm/dialects/mssql"
func Open(dialectName string, args ...interface{}) (*DBCon, error) {
	var db DBCon
	var err error

	if len(args) == 0 {
		err = errors.New("invalid database source")
		return nil, err
	}
	var source string
	var dbSQL sqlCommon

	switch value := args[0].(type) {
	case string:
		var driverName = dialectName
		if len(args) == 1 {
			source = value
		} else if len(args) >= 2 {
			driverName = value
			source = args[1].(string)
		}
		dbSQL, err = sql.Open(driverName, source)
	case sqlCommon:
		source = reflect.Indirect(reflect.ValueOf(value)).FieldByName("dsn").String()
		dbSQL = value
	}
	//TODO : dialects map should disappear - instead of dialectName we should receive directly the Dialect
	var commontDialect Dialect
	if value, ok := dialectsMap[dialectName]; ok {
		commontDialect = reflect.New(reflect.TypeOf(value).Elem()).Interface().(Dialect)
		commontDialect.SetDB(dbSQL.(*sql.DB))
	} else {
		fmt.Printf("`%v` is not officially supported, running under compatibility mode.\n", dialectName)
		commontDialect = &commonDialect{}
		commontDialect.SetDB(dbSQL.(*sql.DB))
	}

	db = DBCon{
		dialect:  commontDialect,
		logger:   defaultLogger,
		callback: &Callback{},
		source:   source,
		values:   map[string]interface{}{},
		db:       dbSQL,
	}
	//register all default callbacks
	db.callback.registerGORMDefaultCallbacks()
	db.parent = &db

	if err == nil {
		err = db.DB().Ping() // Send a ping to make sure the database connection is alive.
		if err != nil {
			db.DB().Close()
		}
	}

	return &db, err
}
