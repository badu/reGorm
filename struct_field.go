package gorm

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const (
	//bit flags - flags are uint16, which means we can use 16 flags
	IS_PRIMARYKEY     uint16 = 0
	IS_NORMAL         uint16 = 1
	IS_IGNORED        uint16 = 2
	IS_SCANNER        uint16 = 3
	IS_TIME           uint16 = 4
	HAS_DEFAULT_VALUE uint16 = 5
	IS_FOREIGNKEY     uint16 = 6
	IS_BLANK          uint16 = 7
	IS_SLICE          uint16 = 8
	IS_STRUCT         uint16 = 9
	HAS_RELATIONS     uint16 = 10
	IS_EMBED_OR_ANON  uint16 = 11
	IS_AUTOINCREMENT  uint16 = 12
	IS_POINTER        uint16 = 13
	IS_OMITTED        uint16 = 14
	IS_INCLUDED       uint16 = 15
)

//TODO : @Badu - benchmark discarding settings (except those with value) while keeping the flags
func NewStructField(fromStruct reflect.StructField) (*StructField, error) {
	result := &StructField{
		StructName: fromStruct.Name,
		Names:      []string{fromStruct.Name},
	}
	//field should process itself the tag settings
	err := result.parseTagSettings(fromStruct.Tag)

	if result.tagSettings.has(IGNORED) {
		result.setFlag(IS_IGNORED)
	}

	if result.tagSettings.has(PRIMARY_KEY) {
		result.setFlag(IS_PRIMARYKEY)
	}

	if result.tagSettings.has(DEFAULT) {
		result.setFlag(HAS_DEFAULT_VALUE)
	}

	if result.tagSettings.has(AUTO_INCREMENT) {
		result.setFlag(IS_AUTOINCREMENT)
		if !result.IsPrimaryKey() {
			result.setFlag(HAS_DEFAULT_VALUE)
		}
	}

	if result.HasSetting(EMBEDDED) || fromStruct.Anonymous {
		//result.isEmbedOrAnon = true
		result.setFlag(IS_EMBED_OR_ANON)
	}

	// Even it is ignored, also possible to decode db value into the field
	if value := result.tagSettings.get(COLUMN); value != "" {
		result.DBName = value
	} else {
		result.DBName = NamesMap.ToDBName(fromStruct.Name)
	}

	//keeping the type for later usage
	result.Type = fromStruct.Type

	//dereference it, it's a pointer
	for result.Type.Kind() == reflect.Ptr {
		result.setFlag(IS_POINTER)
		result.Type = result.Type.Elem()
	}

	//create a value of it, to be returned
	result.Value = reflect.New(result.Type)

	if !result.IsIgnored() {
		//checking implements scanner or time
		_, isScanner := result.Value.Interface().(sql.Scanner)
		_, isTime := result.Value.Interface().(*time.Time)
		if isScanner {
			// is scanner
			result.setFlag(IS_NORMAL)
			result.setFlag(IS_SCANNER)
		} else if isTime {
			// is time
			result.setFlag(IS_NORMAL)
			result.setFlag(IS_TIME)
		}
	}

	//ATTN : order matters, since it can be both slice and struct
	if result.Type.Kind() == reflect.Slice {
		//mark it as slice
		result.setFlag(IS_SLICE)

		for result.Type.Kind() == reflect.Slice || result.Type.Kind() == reflect.Ptr {
			if result.Type.Kind() == reflect.Ptr {
				result.setFlag(IS_POINTER)
			}
			//getting rid of slices and slices of pointers
			result.Type = result.Type.Elem()
		}
		//it's a slice of structs
		if result.Type.Kind() == reflect.Struct {
			//mark it as struct
			result.setFlag(IS_STRUCT)
		}
	} else if result.Type.Kind() == reflect.Struct {
		//mark it as struct
		result.setFlag(IS_STRUCT)
		if !result.IsIgnored() && result.IsScanner() {
			for i := 0; i < result.Type.NumField(); i++ {
				tag := result.Type.Field(i).Tag
				for _, str := range []string{tag.Get(TAG_SQL), tag.Get(TAG_GORM)} {
					err := result.tagSettings.loadFromTags(str)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	if !result.IsIgnored() && !result.IsScanner() && !result.IsTime() && !result.IsEmbedOrAnon() {
		if result.IsSlice() {
			result.SetHasRelations() //marker for later processing of relationships
		} else if result.IsStruct() {
			result.SetHasRelations() //marker for later processing of relationships
		} else {
			result.SetIsNormal()
		}
	}

	return result, err
}

func (field *StructField) ptrToLoad() reflect.Value {
	return reflect.New(reflect.PtrTo(field.Value.Type()))
}

func (field *StructField) makeSlice() (interface{}, reflect.Value) {
	basicType := field.Type
	if field.IsPointer() {
		basicType = reflect.PtrTo(field.Type)
	}
	sliceType := reflect.SliceOf(basicType)
	slice := reflect.New(sliceType)
	slice.Elem().Set(reflect.MakeSlice(sliceType, 0, 0))
	return slice.Interface(), IndirectValue(slice.Interface())
}

func (field *StructField) Interface() interface{} {
	return reflect.New(field.Type).Interface()
}

func (field *StructField) IsPrimaryKey() bool {
	return field.flags&(1<<IS_PRIMARYKEY) != 0
}

func (field *StructField) IsNormal() bool {
	return field.flags&(1<<IS_NORMAL) != 0
}

func (field *StructField) IsPointer() bool {
	return field.flags&(1<<IS_POINTER) != 0
}

func (field *StructField) SetIsNormal() {
	field.flags = field.flags | (1 << IS_NORMAL)
}

func (field *StructField) IsIgnored() bool {
	return field.flags&(1<<IS_IGNORED) != 0
}

func (field *StructField) IsScanner() bool {
	return field.flags&(1<<IS_SCANNER) != 0
}

func (field *StructField) IsTime() bool {
	return field.flags&(1<<IS_TIME) != 0
}

func (field *StructField) HasDefaultValue() bool {
	return field.flags&(1<<HAS_DEFAULT_VALUE) != 0
}

func (field *StructField) IsForeignKey() bool {
	return field.flags&(1<<IS_FOREIGNKEY) != 0
}

func (field *StructField) SetIsForeignKey() {
	field.flags = field.flags | (1 << IS_FOREIGNKEY)
}

func (field *StructField) IsBlank() bool {
	return field.flags&(1<<IS_BLANK) != 0
}

func (field *StructField) IsSlice() bool {
	return field.flags&(1<<IS_SLICE) != 0
}

func (field *StructField) IsStruct() bool {
	return field.flags&(1<<IS_STRUCT) != 0
}

func (field *StructField) HasRelations() bool {
	return field.flags&(1<<HAS_RELATIONS) != 0
}

func (field *StructField) SetHasRelations() {
	field.flags = field.flags | (1 << HAS_RELATIONS)
}

func (field *StructField) IsEmbedOrAnon() bool {
	return field.flags&(1<<IS_EMBED_OR_ANON) != 0
}

func (field *StructField) IsAutoIncrement() bool {
	return field.flags&(1<<IS_AUTOINCREMENT) != 0
}

//TODO : @Badu - make field aware of "it's include or not"
func (field *StructField) IsOmmited() bool {
	return field.flags&(1<<IS_OMITTED) != 0
}

//TODO : @Badu - make field aware of "it's include or not"
func (field *StructField) SetIsOmmitted() {
	field.flags = field.flags | (1 << IS_OMITTED)
}

//TODO : @Badu - make field aware of "it's include or not"
func (field *StructField) IsIncluded() bool {
	return field.flags&(1<<IS_INCLUDED) != 0
}

//TODO : @Badu - make field aware of "it's include or not"
func (field *StructField) SetIsIncluded() {
	field.flags = field.flags | (1 << IS_INCLUDED)
}

func (field *StructField) UnsetIsAutoIncrement() {
	field.unsetFlag(IS_AUTOINCREMENT)
}

// Set set a value to the field
func (field *StructField) SetIsAutoIncrement() {
	field.setFlag(IS_AUTOINCREMENT)
}

func (field *StructField) GetStructName() string {
	return field.StructName
}

//checks if has such a key (for code readability)
func (field *StructField) GetTagSetting() TagSettings {
	return field.tagSettings
}

//gets a key (for code readability)
func (field *StructField) HasSetting(named uint8) bool {
	return field.tagSettings.has(named)
}

func (field *StructField) GetSetting(named uint8) string {
	return field.tagSettings.get(named)
}

func (field *StructField) SetJoinTableFK(value string) {
	field.tagSettings.set(IS_JOINTABLE_FOREIGNKEY, value)
}

func (field *StructField) Set(value interface{}) error {
	var (
		err        error
		fieldValue = field.Value
	)
	if !field.Value.IsValid() {
		//TODO : @Badu - make errors more explicit : which field...
		return errors.New("StructField : field value not valid")
	}

	if !field.Value.CanAddr() {
		return ErrUnaddressable
	}
	//TODO : @Badu - we have this kind of information in our field ...
	//type cast to value
	reflectValue, ok := value.(reflect.Value)
	if !ok {
		//couldn't cast - reflecting it
		reflectValue = reflect.ValueOf(value)
	}

	if reflectValue.IsValid() {
		if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
			//we set it
			fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
		} else {
			if fieldValue.Kind() == reflect.Ptr {
				//it's a pointer
				if fieldValue.IsNil() {
					//and it's NIL : we have to build it
					fieldValue.Set(reflect.New(field.Type))
				}
				//we dereference it
				fieldValue = fieldValue.Elem()
			}
			//#fix (chore) : if implements scanner don't attempt to convert, just pass it over
			if scanner, ok := fieldValue.Addr().Interface().(sql.Scanner); ok {
				//implements Scanner - we pass it over
				err = scanner.Scan(reflectValue.Interface())
			} else if reflectValue.Type().ConvertibleTo(fieldValue.Type()) {
				fieldValue.Set(reflectValue.Convert(fieldValue.Type()))
			} else {
				//Oops
				//TODO : @Badu - make errors more explicit
				err = fmt.Errorf("could not convert argument of field %s from %s to %s", field.GetStructName(), reflectValue.Type(), fieldValue.Type())
			}
		}
	} else {
		//it's not valid
		field.Value.Set(reflect.Zero(field.Value.Type()))
	}
	//TODO : @Badu - seems invalid logic : above we set it ot zero if it's not valid
	//then we check if the value is blank
	//check if the value is blank
	field.setIsBlank()
	return err
}

// ParseFieldStructForDialect parse field struct for dialect
func (field *StructField) ParseFieldStructForDialect() (reflect.Value, string, int, string) {
	var (
		size       = 0
		fieldValue = reflect.Indirect(reflect.New(field.Type))
	)

	// Get scanner's real value
	fieldValue = getScannerValue(fieldValue)

	// Default Size
	if num := field.GetSetting(SIZE); num != "" {
		size, _ = strconv.Atoi(num)
	} else {
		size = 255
	}

	//TODO : @Badu - what if the settings below are empty?
	// Default type from tag setting
	additionalType := field.GetSetting(NOT_NULL) + " " + field.GetSetting(UNIQUE)
	if value := field.GetSetting(DEFAULT); value != "" {
		additionalType = additionalType + " DEFAULT " + value
	}

	return fieldValue, field.GetSetting(TYPE), size, strings.TrimSpace(additionalType)
}

//implementation of Stringer
func (field StructField) String() string {
	var collector Collector
	collector.add("%s = %q\n", "Name", field.DBName)
	for _, n := range field.Names {
		collector.add("\t%s = %q\n", "name", n)
	}

	collector.add("Flags:")
	if field.flags&(1<<IS_PRIMARYKEY) != 0 {
		collector.add(" PrimaryKey")
	}
	if field.flags&(1<<IS_NORMAL) != 0 {
		collector.add(" IsNormal")
	}
	if field.flags&(1<<IS_IGNORED) != 0 {
		collector.add(" IsIgnored")
	}
	if field.flags&(1<<IS_SCANNER) != 0 {
		collector.add(" IsScanner")
	}
	if field.flags&(1<<IS_TIME) != 0 {
		collector.add(" IsTime")
	}
	if field.flags&(1<<HAS_DEFAULT_VALUE) != 0 {
		collector.add(" HasDefaultValue")
	}
	if field.flags&(1<<IS_FOREIGNKEY) != 0 {
		collector.add(" IsForeignKey")
	}
	if field.flags&(1<<IS_BLANK) != 0 {
		collector.add(" IsBlank")
	}
	if field.flags&(1<<IS_SLICE) != 0 {
		collector.add(" IsSlice")
	}
	if field.flags&(1<<IS_STRUCT) != 0 {
		collector.add(" IsStruct")
	}
	if field.flags&(1<<HAS_RELATIONS) != 0 {
		collector.add(" HasRelations")
	}
	if field.flags&(1<<IS_EMBED_OR_ANON) != 0 {
		collector.add(" IsEmbedAnon")
	}
	if field.flags&(1<<IS_AUTOINCREMENT) != 0 {
		collector.add(" IsAutoincrement")
	}
	if field.flags&(1<<IS_POINTER) != 0 {
		collector.add(" IsPointer")
	}
	if field.flags&(1<<IS_OMITTED) != 0 {
		collector.add(" IsOmmited")
	}
	if field.flags&(1<<IS_INCLUDED) != 0 {
		collector.add(" HasIncluded")
	}
	collector.add("\n")

	if field.tagSettings.len() > 0 {
		collector.add("%s = %q\n", "Tags:", field.tagSettings)
	}
	if field.Type != nil {
		collector.add("%s = %s\n", "Type:", field.Type.String())
	}
	collector.add("%s = %s\n", "Value:", field.Value.String())
	if field.Relationship != nil {
		collector.add("%s\n%s", "Relationship:", field.Relationship)
	}
	return collector.String()
}

////////////////////////////////////////////////////////////////////////////////
// Private methods
////////////////////////////////////////////////////////////////////////////////
func (field *StructField) setFlag(value uint16) {
	field.flags = field.flags | (1 << value)
}

func (field *StructField) unsetFlag(value uint16) {
	field.flags = field.flags & ^(1 << value)
}

func (field *StructField) clone() *StructField {
	clone := &StructField{
		flags:        field.flags,
		DBName:       field.DBName,
		Names:        field.Names,
		tagSettings:  field.tagSettings.clone(),
		StructName:   field.StructName,
		Relationship: field.Relationship,
		Type:         field.Type,
	}

	return clone
}

func (field *StructField) cloneWithValue(value reflect.Value) *StructField {
	clone := &StructField{
		flags:        field.flags,
		DBName:       field.DBName,
		Names:        field.Names,
		tagSettings:  field.tagSettings.clone(),
		StructName:   field.StructName,
		Relationship: field.Relationship,
		Value:        value,
		Type:         field.Type,
	}
	//check if the value is blank
	clone.setIsBlank()
	return clone
}

//Function collects information from tags named `sql:""` and `gorm:""`
func (field *StructField) parseTagSettings(tag reflect.StructTag) error {
	field.tagSettings = TagSettings{Uint8Map: make(map[uint8]string)}
	for _, str := range []string{tag.Get(TAG_SQL), tag.Get(TAG_GORM)} {
		err := field.tagSettings.loadFromTags(str)
		if err != nil {
			return err
		}
	}
	return nil
}

//TODO : @Badu - seems expensive to be called everytime. Maybe a good solution would be to
//change isBlank = true by default and modify the code to change it to false only when we have a value
//to make this less expensive
func (field *StructField) setIsBlank() {
	if reflect.DeepEqual(field.Value.Interface(), reflect.Zero(field.Value.Type()).Interface()) {
		field.setFlag(IS_BLANK)
	} else {
		field.unsetFlag(IS_BLANK)
	}
}

func (field *StructField) getForeignKeys() StrSlice {
	var result StrSlice
	if foreignKey := field.GetSetting(FOREIGNKEY); foreignKey != "" {
		result.commaLoad(foreignKey)
	}
	return result
}

func (field *StructField) getAssocForeignKeys() StrSlice {
	var result StrSlice
	if foreignKey := field.GetSetting(ASSOCIATIONFOREIGNKEY); foreignKey != "" {
		result.commaLoad(foreignKey)
	}
	return result
}
