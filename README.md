## Overview

* Full-Featured ORM (almost)
* Associations (Has One, Has Many, Belongs To, Many To Many, Polymorphism)
* Callbacks (Before/After Create/Save/Update/Delete/Find)
* Preloading (eager loading)
* Transactions
* Composite Primary Key
* SQL Builder
* Auto Migrations
* Logger
* Extendable, write Plugins based on GORM callbacks
* Every feature comes with tests
* Developer Friendly

## Getting Started

* GORM Guides [jinzhu.github.com/gorm](http://jinzhu.github.io/gorm)

# Comments and thoughts 
- As a general idea on golang projects : "fail fast" type of logic is the best approach
- When a function is called everytime, best idea is to allow golang to inline it
- regexp.MustCompile is slow inside functions (10 times slower)

# Last merge
- 05.12.2016 - "Add gorm:association:source for association operations for plugins to extend GORM"
- Note : what's not included before that merge, it's because I don't think it's good to do so
 
# Breaking changes
- DB struct - renamed to DBCon, since that is what it represents.
    However, you can do the following, to use the old gorm.DB:
    `dbcon, err := gorm.Open("mysql", dbstr+"?parseTime=true")`
    `db = &gorm.DB{*dbcon}`
- Removed MSSQL support - out of my concerns with this project

# Changes log (29.10.2016-present)

# Todo
- [ ] Documentation for tests and build examples
- [ ] Generated SQL let's the SQL engine cast : SELECT * FROM aTable WHERE id = '1' (id being int). I think it's a bad practice and it should be fixed
- [ ] convertInterfaceToMap in utils it's terrible : simplify

## 28.01.2017
- [x] update IsZero as in last original gorm commit

## 21.12.2016
- [ ] replace fields access (where possible) via modelstruct instead of scope fields
- [x] Scope rType property holds reflect.Type of the Value (on clone) 

## 20.12.2016
- [x] SetZero, IsZero in utils
- [x] Scope has getColumnAsArray instead of generic getColumnAsArray in utils
- [x] added field not found error
- [x] removed errors.New() - replaced with fmt.Errorf

## 19.12.2016
- [x] removed Scope NewScope - inlined into utils
- [x] Scope holds rValue (prepare removal of IndirectValue calls)
- [x] unified DBCon NewScope with newScope
- [x] replace IndirectValue calls
- [x] removed GetType from utils, renamed GetTType to GetType

## 18.12.2016
- [x] Quote fields are now cached and DBCon is holding them
- [x] Removed Search struct TableName() method
- [x] Replaced, where possible, GetSliceSetting with GetForeignFieldNames, GetAssociationForeignFieldNames, GetForeignDBNames, GetAssociationDBNames
- [x] Fix for Association.Count when there are no where conditions
- [x] Fixed some tests (duplicate entries errors or misspellings)
- [x] removed Scope's attrs interface{} - since we can pass it as argument to postUpdate()
- [x] Scope TableName fix : if implements tabler or dbtabler should override the search table name
- [x] Fix for Update after save, e.g. TestDB.Save(&animal).Update("name", "Francis") - scope table name is empty, do not execute query
- [x] getScannerValue removed from utils
- [x] toQueryMarks optimisation (concat instead of string joins)

## 17.12.2016
- [x] When error occurs, print the SQL that was executed (via AddErr of the Scope, we're passing SQL and SQLVars)
- [x] DBCon has modelsStructMap property which keeps the map that was a "global" variable (separate of concerns and encapsulation)
- [x] convertInterfaceToMap in utils - created a scope without connection (should be forbidden)
- [x] SetJoinTableHandler in DBCon - created a scope without connection (should be forbidden)
- [x] safeModelStructsMap made private
- [x] DBCon exposes modelsStructMap via KnownModelStructs()
- [x] DBCon has namesMap property which keeps the map that was a "global" variable (separate of concerns and encapsulation) same as above
- [x] DBCon KnownNames(name string) for checking against namesMap
- [x] had to modify ModelStruct FieldByName signature to FieldByName(column string, con *DBCon) to give access to namesMap

## 16.12.2016
- [x] Replaced - where possible - conditions for relation checking (readability)
- [x] fieldsMap struct - all methods are private
- [x] moved errors from utils_relations and performed some cleanups
- [x] Search moved flags in types and renamed to be private
- [x] extracted strings from dialects

## 15.12.2016
- [x] Bug fix for ManyToManyWithCustomizedForeignKeys2
- [x] Fix for calling AutoMigrate with JoinTableHandlerInterface
- [x] refactored createJoinTable in utils_migrations
- [x] Fix DoJoinTable test : SetJoinTableHandler should be used only if AutoMigrate creates the tables first
```go 
TestDB.AutoMigrate(&Person{},&PersonAddress{})
TestDB.SetJoinTableHandler(&Person{}, "Addresses", &PersonAddress{})
```
   instead of 
```go 
TestDB.AutoMigrate(&Person{})
TestDB.SetJoinTableHandler(&Person{}, "Addresses", &PersonAddress{})
```

## 14.12.2016
- [x] Bug fix in StructField ParseFieldStructForDialect : slice of bytes is wrong (probably all kind of slices are)
- [x] Fix for ManyToManyWithMultiPrimaryKeys test : toQueryMarks optimisation in utils was breaking it
- [x] Cleanup in StructField after using COLUMN tag setting 
- [x] GetHandlerStruct of JoinTableHandler and interface - for debugging

## 13.12.2016
- [x] removed ORDER_BY_PK_SETTING and logic from dbCon First and Last (order it's kept in search)
- [x] removed QUERY_DEST_SETTING and logic : Scope's postQuery accepts a parameter which is destination for dbCon Scan          
- [x] created QueryOption test for "gorm:query_option"
- [x] Association Count IS failing in mysql tests, because for some reason we get field and/or scope nil
- [x] removed IGNORE_PROTEC_SETTING : set in dbCon Updates, but never used

## 11.12.2016
- [x] CallMethod - extract string constants
- [x] reduced Scope callbacks
- [x] more string concat instead of string slices

## 08.12.2016
- [x] removed STARTED_TX_SETTING : Scope's Begin returns also a bool, CommitOrRollback accepts a bool param
- [x] removed BLANK_COLS_DEFAULT_SETTING : Scope's beforeCreateCallback returns also a string, forceReloadAfterCreateCallback accepts that string
- [x] string concat instead of string slices where possible (cheaper)
- [x] DBCon SetLogMode(mode int) and LOG_OFF, LOG_VERBOSE, LOG_DEBUG constants
- [x] removed UPDATE_ATTRS_SETTING : instead, Scope has now updateMaps map[string]interface{} which holds that data
- [x] got rid of InstanceSet and InstanceGet from Scope, instanceID (uint64) property was removed
- [x] callCallbacks gets called only we have registered functions, so we won't call function for nothing

## 07.12.2016
- [x] "Add gorm:association:source for association operations for plugins to extend GORM" from original commit 
- [x] removed SelectWithArrayInput test
- [x] changed dbcon signature for Select - it doesn't accept slices of strings anymore
- [x] Skip order sql when quering with distinct commit
- [x] Search Select is using only first select clause - now it's overriding existing select clauses 
- [x] Scope createCallback was adding BLANK_COLS_DEFAULT_SETTING inside a for loop (since it was a map, was not malfunctioning, but was bad logic)
- [x] BLANK_COLS_DEFAULT_SETTING is now storing a string, not a StrSlice
- [x] Add gorm:association:source for association operations for plugins to extend GORM commit

## 05.12.2016
- [x] removed getTableOptions from Scope : was used in creation operations, so we don't need it there
- [x] after processing relations, we cleanup tag setting's ASSOCIATIONFOREIGNKEY and FOREIGNKEY for allocation sake
- [x] added sort for test results
- [x] autoIndex in tables_operations calls directly addIndex (instead of going through dbcon)

## 04.12.2016
- [x] StrSlices for association and foreign keys gets created on parseTagSettings of StructField
- [x] tag settings map changed from map[uint8]string to map[uint8]interface{}
- [x] Relationship : PolymorphicType, PolymorphicDBName, PolymorphicValue were removed
- [x] Relationship : Moved relationship kind to tag settings
- [x] Relationship : removed JoinTableHandler (holded by tag settings now)
- [x] Relationship removed.
- [x] seems JoinTableHandler did nothing with sources parameter of Delete method : in association Replace method was passing relationship

## 03.12.2016
- [x] Warning for relationship
- [x] Unset field flag, so we can use HasRelations() instead of checking for relationship == nil (HAS_RELATIONS)
- [x] test/types.go
- [x] checking field.HasRelations() - instead of checking for relationship == nil 

## 02.12.2016
- [x] tag settings are kept only if they have values - flags are set in parent StructField
- [x] tag settings is concurrent map
- [x] StructField Set method invalid logic : after setting it to zero, we don't verify is blank, we just set the flag
- [x] Rearranged tests, with a form of benchmarking
- [x] Removed SetJoinTableFK from StructField (unused) 

## 30.11.2016
- [x] Simplify reflections
- [x] Removed the way internal callbacks were called - gorm can't be broken by un-registering internal callbacks
- [x] removed Struct property of StructField - added StructName property
- [x] removed handleBelongsToPreload from Search

## 29.11.2016
- [x] Stringer for Relationship, ModelStruct and StructField

## 28.11.2016
- [x] Benchmark Quote with regexp, runes, runes conversion and byte shifting
- [x] Optimized Quote in utils : uses "prepared" regexp 
- [x] changed the Dialect interface : Quote(key string) string is now GetQuoter() string - which returns the quote string(rune)

## 27.11.2016
- [x] created Search struct methods for Query, QueryRow and Exec
- [x] created Search struct RowsAffected field (to replace DBCon's one)
- [x] argsToInterface in utils
- [x] updatedAttrsWithValues in utils
- [x] getValueFromFields in utils
- [x] initialize moved from Scope to Search
- [x] implement Warnings (like logs, but always)
- [x] All create, migrate and alter functions should be moved from the scope inside a separate file (since we're automigrating just at startup)
- [x] put back PrimaryKey() of Scope shadow of PKName() 
- [x] put back PrimaryField() of Scope shadow of PK() 
- [x] put back PrimaryFields() of Scope shadow of PKs()

## 26.11.2016
- [x] polished Scope methods
- [x] removed inlineConditions from Search
- [x] rearranged Search combinedConditionSql to have fewer calls
- [x] rearranged Search prepareQuerySQL to have fewer calls
- [x] Scope's instanceID is now uint64 and holds the pointer address of that Scope 
- [x] replace current method of keeping data Set / SetInstance / Get
- [x] switch back to some inline function : getColumnAsArray, generatePreloadDBWithConditions, toQueryValues, toQueryMarks, toQueryCondition, QuoteIfPossible, Quote
- [x] moved Value from DBCon struct to Search struct (temporary)

## 25.11.2016
- [x] utils, removed toSearchableMap
- [x] utils, convertInterfaceToMap moved to Scope
- [x] DBConFunc func(*DBCon) *DBCon
- [x] benchmarks organized
- [x] DBCon, Scope have Warn
- [x] Scope related() fail fast logic

## 24.11.2016
- [x] moved SQL related functions from Scope to Search struct
- [x] Search struct replaced scope.AddToVars with addToVars , Scope AddToVars() method removed 
- [x] slimmer Search struct : group, limit, offset gone 

## 23.11.2016
- [x] file for expr struct - removed, replaced with SqlPair
- [x] removed Scope SelectAttrs() method and Scope selectAttrs property
- [x] removed Scope OmitAttrs() []string
- [x] because above removals, Scope changeableField() method simplified
- [x] Search struct - added more flags instead of using length for conditions
- [x] moved SQL and SQLVars from Scope to Search
- [x] removed Search "con" property : DBCon struct has now clear unscoped methods (stores in search property)
- [x] removed Search getInterfaceAsSQL since it was used by Group, which takes string parameter

## 22.11.2016
- [x] slimmer Search struct - preload gone
- [x] slimmer Search struct - selects gone
- [x] slimmer Search struct - order gone
- [x] Search true clone
- [x] Search conditions renamed to Conditions, sqlConditions struct renamed to SqlConditions,  so search_test.go could be moved in tests

## 21.11.2016
- [x] slimmer Search struct - notConditions are gone
- [x] slimmer Search struct - flags instead of booleans
- [x] slimmer Search struct - initAttrs gone + method GetInitAttr()        
- [x] slimmer Search struct - assignAttrs gone + method GetAssignAttr()

## 16.11.2016
- [x] slimmer search struct - whereConditions, orConditions, havingConditions, joinConditions are gone

## 15.11.2016
- [x] minor modification on ModelStruct Create() : moved HasRelations flag setters into StructField
- [x] moved getValueFromFields from utils.go to string_slice.go (even if it don't belong there)

## 14.11.2016
- [x] StructField - optimized creation
- [x] StructField - optimized makeSlice()
- [x] StructField - method PtrToValue() called in Scope (scan)
- [x] integrate Omit duplicates and zero-value ids in preload queries. Resolves #854 and #1054.
 
## 13.11.2016
- [x] ModelStruct - removed properties PrimaryFields and StructFields - they are kept in fieldsMap struct
- [x] ModelStruct - fieldsMap struct has method PrimaryFields() which are cached into cachedPrimaryFields
- [x] extracted string "id" into a const (Scope and ModelStruct)
- [x] ModelStruct - method for number of primary fields
- [x] Added flag for IS_AUTOINCREMENT (and logic for it)
- [x] renamed PrimaryFields to PKs, PrimaryField to PK
- [x] polished relationship.go methods
- [x] added errors on relationship.go when fields not found, but they break the tests (TODO : investigate why)
- [x] got rid of checkInterfaces() method of ModelStruct (simplification)
- [x] make StructField be able to provide a value's interface (Interface() method)
- [x] make ModelStruct be able to provide a value's interface (Interface() method)
- [x] cleanup reflect.New(blah... blah) - replaced with Interface() call (WIP)

## 12.11.2016
- [x] switched bitflag from uint64 to uint16 (we really don't need more than 16 at the time)
- [x] make ModelStruct map it's fields : fieldsMap struct
- [x] make ModelStruct map it's fields : logic modification in fieldByName() - if mapped not found, looking into NamesMap
- [x] make ModelStruct map it's fields : ModelStruct has addField(field) method
- [x] make ModelStruct map it's fields : ModelStruct has addPK(field) method (primary keys)
- [x] make ModelStruct map it's fields : ModelStruct has HasColumn(name) method 
- [x] make ModelStruct map it's fields : removed Scope method HasColumn(name)
- [x] refactored Scope Fields() method - calls a cloneStructFields method of ModelStruct
- [x] simplified further the GetModelStruct() of Scope to cleanup the fields
- [x] renamed DB() of Scope to Con()
- [x] renamed NewDB() of Scope to NewCon()
- [x] make Relationship have some methods so we can move code from ModelStruct (part 1 - ModelStruct sliceRelationships() removed)
- [x] make Relationship have some methods so we can move code from ModelStruct (part 2 - ModelStruct structRelationships() removed)

## 11.11.2016
- [x] instead of having this bunch of flags in StructField - bitflag
- [x] removed joinTableHandlers property from DBCon (was probably leftover of work in progress)
- [x] simplified Setup(relationship *Relationship, source reflect.Type, destination reflect.Type) of JoinTableHandlerInterface
- [x] added SetTable(name string) to JoinTableHandlerInterface
- [x] renamed property "db" of DBCon to "sqli"
- [x] renamed interface sqlCommon to sqlInterf
- [x] renamed property "db" of Scope to "con"
- [x] renamed property "db" of search struct to "con"
- [x] search struct has collectAttrs() method which loads the cached selectAttrs of the Scope

## 10.11.2016
- [x] StructField has field UnderlyingType (should keep reflect.Type so we won't use reflection everywhere)
- [x] finally got rid of defer inside loop of Scope's GetModelStruct method (ModelStruct's processRelations method has a loop in which calls relationship processors)
- [x] introduced a HasRelations and IsTime in StructField

## 09.11.2016
- [x] Collector - a helper to avoid multiple calls on fmt.Sprintf : stores values and string
- [x] replaced some statements with switch
- [x] GetModelStruct refactoring
- [x] GromErrors change and fix (from original gorm commits)

## 08.11.2016
- [x] adopted skip association tag from https://github.com/slockij/gorm (`gorm:"save_associations:false"`)
- [x] adopted db.Raw().First() makes wrong sql fix #1214 #1243
- [x] registerGORMDefaultCallbacks() calls reorder at the end of registration
- [x] Scope toQueryCondition() from utils.go
- [x] Moved callbacks into Scope (needs closure functions)
- [x] Removed some postgres specific functions from utils.go

## 07.11.2016
- [x] have NOT integrate original-gorm pull request #1252 (prevent delete/update if conditions are not met, thus preventing delete-all, update-all) tests fail
- [x] have looked upon all original-gorm pull requests - and decided to skip them 
- [x] have NOT integrate original-gorm pull request #1251 - can be done with Scanner and Valuer
- [x] have NOT integrate original-gorm pull request #1242 - can be done simplier
- [x] ParseFieldStructForDialect() moved to struct_field.go from utils.go
- [x] makeSlice() moved to struct_field.go from utils.go
- [x] indirect() from utils.go, swallowed where needed (shows logic better when dereferencing pointer)
- [x] file for expr struct (will add more functionality)
- [x] cloneWithValue() method in db.go 

## 06.11.2016
- [x] got rid of parseTagSetting method from utils.go
- [x] moved ToDBName into safe_map.go - renamed smap to NamesMap
- [x] StrSlice is used in Relationship
- [x] more cleanups
- [x] more renaming

## 05.11.2016
- [x] DefaultCallback removed from types - it's made under open and registers all callbacks there
- [x] callback.go has now a method named registerDefaults
- [x] scope's GetModelStruct refactored and fixed a few lint problems

## 02.11.2016
- [x] avoid unnecessary calls in CallbackProcessors reorder method (lengths zero)
- [x] Refactored sortProcessors not to be recursive, but have a method called sortCallbackProcessor inside CallbackProcessor
- [x] Concurent slice and map in utils (so far, unused)
- [x] type CallbackProcessors []*CallbackProcessor for readability
- [x] callback_processors.go file which holds methods for type CallbackProcessors (add, len)
- [x] moved sortProcessors from utils.go to callback_processors.go as method
- [x] created type ScopedFunc  func(*Scope)
- [x] created type ScopedFuncs []*ScopedFunc
- [x] replaced types ScopedFunc and ScopedFuncs to be more readable  

## 01.11.2016
- [x] TestCloneSearch could not be moved
- [x] Exposed some methods on Callback for tests to run (GetCreates, GetUpdates, GetQueries, GetDeletes)
- [x] Moved tests to tests folder (helps IDE)
- [x] Extracted strings from dialect_common.go
- [x] Extracted strings from dialect_mysql.go
- [x] Modified some variable names to comply to linter ("collides with imported package name")
- [x] Remove explicit variable name on returns
- [x] Removed method newDialect from utils.go (moved it into Open() method)
- [x] Removed MSSQL support - out of my concerns with this project
- [x] Fix (chore) in StructField Set method : if implements Scanner don't attempt to convert, just pass it over
- [x] Test named TestNot
- [x] CallbackProcessor kind field changed from string to uint8

## 30.10.2016 (Others)
- [x] DB struct - renamed to DBCon, since that is what it represents

## 30.10.2016 (Operation Field -> StructField)

- [x] StructFields has it's own file to get rid of append() everywhere in the code
- [x] TagSettings map[uint8]string in StructField will become a struct by itself, to support has(), get(), set(), clone(), loadFromTags()
- [x] TagSettings should be private in StructField
- [x] replace everywhere []*StructField with type StructFields
- [x] create StructFields type []*StructField for code readability
- [x] NewStructField method to create StructField from reflect.StructField
- [x] Field struct, "Field" property renamed to "Value", since it is a reflect.Value
- [x] StructField should swallow Field model field definition
- [x] created cloneWithValue(value reflect.Value) on StructField -> calls setIsBlank()
- [x] moved isBlank(fieldValue) from utils to StructField named setIsBlank()
- [x] remove getForeignField from utils.go -> ModelStruct has a method called getForeignField(fieldName string)

## 29.10.2016

- [x] Moved code around
- [x] Numbered tests - so I can track what fails
- [x] Replaced some string constants like "many_to_many" and refactor accordingly
- [x] StructField is parsing by it's own gorm and sql tags with method ParseTagSettings
- [x] Replaced string constants for the tags and created a map string-to-uint8
- [x] Removed field Name from StructField since Struct property of it exposes Name
- [x] Created method GetName() for StructField to return that name
- [x] Created method GetTag() for StructField to return Struct property Tag (seems unused)