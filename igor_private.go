/*
Copyright 2016 Paolo Galeone. All right reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package igor

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	// Blank import required to get PostgreSQL working
	_ "github.com/lib/pq"
)

// Reserved keyworkds that shouldn't be used as column name or other identifiers
// result of: select '"' || word::text || '",' from pg_get_keywords() where catcode = 'R' OR catdesc like '%cannot%';
// We will check if the current identifier is in that list. If it is, it will be placed between quotes
var reservedKeywords = []string{
	"all",
	"analyse",
	"analyze",
	"and",
	"any",
	"array",
	"as",
	"asc",
	"asymmetric",
	"between",
	"bigint",
	"bit",
	"boolean",
	"both",
	"case",
	"cast",
	"char",
	"character",
	"check",
	"coalesce",
	"collate",
	"column",
	"constraint",
	"create",
	"current_catalog",
	"current_date",
	"current_role",
	"current_time",
	"current_timestamp",
	"current_user",
	"dec",
	"decimal",
	"default",
	"deferrable",
	"desc",
	"distinct",
	"do",
	"else",
	"end",
	"except",
	"exists",
	"extract",
	"false",
	"fetch",
	"float",
	"for",
	"foreign",
	"from",
	"grant",
	"greatest",
	"group",
	"grouping",
	"having",
	"in",
	"initially",
	"inout",
	"int",
	"integer",
	"intersect",
	"interval",
	"into",
	"lateral",
	"leading",
	"least",
	"limit",
	"localtime",
	"localtimestamp",
	"national",
	"nchar",
	"none",
	"not",
	"null",
	"nullif",
	"numeric",
	"offset",
	"on",
	"only",
	"or",
	"order",
	"out",
	"overlay",
	"placing",
	"position",
	"precision",
	"primary",
	"real",
	"references",
	"returning",
	"row",
	"select",
	"session_user",
	"setof",
	"smallint",
	"some",
	"substring",
	"symmetric",
	"table",
	"then",
	"time",
	"timestamp",
	"to",
	"trailing",
	"treat",
	"trim",
	"true",
	"union",
	"unique",
	"user",
	"using",
	"values",
	"varchar",
	"variadic",
	"when",
	"where",
	"window",
	"with",
	"xmlattributes",
	"xmlconcat",
	"xmlelement",
	"xmlexists",
	"xmlforest",
	"xmlparse",
	"xmlpi",
	"xmlroot",
	"xmlserialize",
}

// clear is called at the end of every query, to clean up the db structure
// preserving the connection and the logger
func (db *Database) clear() {
	db.rawRows = nil
	db.tables = nil
	db.joinTables = nil
	db.models = nil
	db.cteSelectValues = nil
	db.cte = ""
	db.selectFields = ""
	db.updateCreateValues = nil
	db.updateCreateFields = nil
	db.whereValues = nil
	db.whereFields = nil
	db.order = ""
	db.limit = 0
	db.offset = 0
	db.varCount = 1
}

// printLog uses db.log to update log
func (db *Database) printLog(v interface{}) {
	if db.logger != nil {
		db.logger.Print(v)
	}
}

// panicLog uses db.log to update log and than it panics
// if db.log is nil, printLog panic using the panic method
func (db *Database) panicLog(v interface{}) {
	if db.logger != nil {
		db.logger.Panic(v)
	} else {
		panic(v)
	}
}

// handleIdentifier handle db idetifiers that are reserved
// It puts reserved keywords used as column/table name betweeen double quotes and
// rename clause into a valid database identifier, following the coventions
func handleIdentifier(clause string) string {
	lowerClause := strings.ToLower(clause)
	i := sort.SearchStrings(reservedKeywords, lowerClause)
	if i < len(reservedKeywords) && reservedKeywords[i] == lowerClause {
		return `"` + lowerClause + `"`
	}
	return namingConvention(clause)
}

// commonRawQuery executes common operations when using raw queries
// returns the prepared statement
func (db *Database) commonRawQuery(query string, args ...interface{}) *sql.Stmt {
	// Replace ? with $n
	query = db.replaceMarks(query)
	// Append args content to current values
	db.whereValues = append(db.whereValues, args...)

	db.printLog(query)
	// Compile query
	var stmt *sql.Stmt
	var err error
	if stmt, err = db.db.Prepare(query + ";"); err != nil {
		db.panicLog(err.Error())
	}
	return stmt
}

// commonCreateUpdate executs common operation in preparation of create / update statements
// because the logic is the same.
// builder is the function that build the UPDATE or CREATE query
func (db *Database) commonCreateUpdate(value DBModel, builder func() string) error {
	defer db.clear()
	// if Model has been called, skip table name inference procedure
	if len(db.tables) == 0 {
		db.tables = append(db.tables, handleIdentifier(value.TableName()))
		db.models = append(db.models, value)
	}

	in := getStruct(value)
	// getFields handle anonymous nested fields
	fields := getFields(value)

	for _, structField := range fields {
		field := in.FieldByName(structField.Name)
		if value := fieldValue(field, structField); value != nil {
			db.updateCreateFields = append(db.updateCreateFields, getColumnName(structField))
			db.updateCreateValues = append(db.updateCreateValues, value)
		}
	}

	// Compile query
	var stmt *sql.Stmt
	var err error
	if stmt, err = db.db.Prepare(builder()); err != nil {
		return err
	}

	// Pass query parameters and executes the query
	// set db.rawRows to query results (of returning) in order to make it possible to scan rows into result
	if db.rawRows, err = stmt.Query(append(db.updateCreateValues, db.whereValues...)...); err != nil {
		return err
	}
	if err = db.Scan(value); err != nil {
		panic(err)
	}
	return nil
}

// replaceMarks replace question marks (?) with the PostgreSQL variable identifier
// using the right (incremental) value
func (db *Database) replaceMarks(in string) string {
	var buffer bytes.Buffer
	for _, runeValue := range in {
		if runeValue == '?' {
			buffer.WriteString("$")
			buffer.WriteString(strconv.Itoa(db.varCount))
			db.varCount++
		} else {
			buffer.WriteRune(runeValue)
		}
	}
	return buffer.String()
}

// primaryKey returns the pair (key, value) representing the defined primary key field
// (key) and its value (value), when the `sql` struct tag field is defined and is value is not blank
// returns empty key if a key is not present and thus en empty value
// oterwise returns the key and the value (if present, i.e. is not blank)
// returned Key is the Name of the field. Not following the sql conventions but the go convention.
// If you need to change this value (and you usually do), parse key with handleIdentifier
func primaryKey(s interface{}) (key string, value interface{}) {
	val := reflect.Indirect(reflect.ValueOf(s))
	for i := 0; i < val.NumField(); i++ {
		fieldValue := val.Field(i)
		fieldType := val.Type().Field(i)
		// Handle embedded anonymous struct only
		switch fieldValue.Kind() {
		case reflect.Struct:
			if fieldType.Anonymous {
				key, value = primaryKey(fieldValue.Interface())
			}
		default:
			tag := strings.ToLower(fieldType.Tag.Get("igor"))
			tagValue := strings.Split(tag, ",")
			sort.Strings(tagValue)
			idx := sort.SearchStrings(tagValue, "primary_key")
			if idx < len(tagValue) && tagValue[idx] == "primary_key" {
				key = fieldType.Name
				value = fieldValue.Interface()
				return
			}
		}
	}
	return
}

// namingConvention returns the coversion of input name to a
// valid db entity that follows the convention
func namingConvention(name string) string {
	// first char is always upper case
	var ucActual = true
	var buffer bytes.Buffer
	buffer.WriteRune(rune(name[0]))
	for i := 1; i < len(name); i++ {
		prevChar := rune(name[i-1])
		actualChar := rune(name[i])
		ucActual = unicode.IsUpper(actualChar)
		if unicode.IsLower(prevChar) && ucActual {
			buffer.WriteByte('_')
		}
		buffer.WriteRune(actualChar)
	}

	return strings.ToLower(buffer.String())
}

// getColumnName returns the column name of the specified field of the struct
// it's the name of the field if the field has not a `igor:column` value spcified
// the field is a valid sql value (thus in case, the name is escaped using handleIdentifier)
func getColumnName(field reflect.StructField) (fieldName string) {
	ts := parseTagSetting(field.Tag.Get("igor"))
	if ts["column"] != "" {
		fieldName = ts["column"]
	} else {
		fieldName = handleIdentifier(field.Name)
	}
	return
}

// getSQLFields returns sql-compatible fields that the select query should return
// skips sql:"-".
func getSQLFields(s DBModel) (ret []string) {
	fields := getFields(s)
	table := handleIdentifier(s.TableName())
	for _, field := range fields {
		ret = append(ret, table+"."+getColumnName(field))
	}
	return
}

// getFields returns a slice of reflect.StructField that represents the exported struct Fields in s
// that are not excluded in sql generation
func getFields(s interface{}) (ret []reflect.StructField) {
	val := reflect.Indirect(reflect.ValueOf(s))
	// addIf adds filedType to ret if is not marked as `sql:"-"`
	addIf := func(fieldType reflect.StructField) {
		tag := strings.ToLower(fieldType.Tag.Get("sql"))
		tagValue := strings.Split(tag, ",")
		sort.Strings(tagValue)
		idx := sort.SearchStrings(tagValue, "-")
		if idx == len(tagValue) || tagValue[idx] != "-" {
			ret = append(ret, fieldType)
		}
	}
	for i := 0; i < val.NumField(); i++ {
		fieldValue := val.Field(i)
		fieldType := val.Type().Field(i)
		// if it's exported
		if fieldType.PkgPath == "" {
			// Handle embedded anoymous struct
			switch fieldValue.Kind() {
			case reflect.Struct:
				// if it's anonymous, embed its fields in the query
				if fieldType.Anonymous {
					ret = append(ret, getFields(fieldValue.Interface())...)
				} else { // use its name only (to work with structs like time.Time)
					addIf(fieldType)
				}
			default:
				addIf(fieldType)
			}
		}
	}

	return
}

// isBlank returns true if value is empty
func isBlank(value reflect.Value) bool {
	return reflect.DeepEqual(value.Interface(), reflect.Zero(value.Type()).Interface())
}

// getStruct panics if s is not a struct. Returns the reflect.Indirect(reflect.ValueOf(s))
func getStruct(s interface{}) reflect.Value {
	in := reflect.Indirect(reflect.ValueOf(s))
	if in.Kind() != reflect.Struct {
		panic(fmt.Sprintf("s must be a sturct, not %s\n", in.Kind()))
	}
	return in
}

// buildSelect returns the generated SQL. Panics if it can't generate a query
func (db *Database) buildSelect() string {
	if len(db.tables) == 0 {
		db.panicLog("Please set a table with Model [ + Joins ]")
	}

	var query bytes.Buffer
	query.WriteString(db.buildCTE())

	// Select
	var fields string
	query.WriteString("SELECT ")
	if len(db.selectFields) > 0 {
		fields = db.selectFields
	}

	query.WriteString(fields)
	query.WriteString(" FROM ")
	// Model && Join

	query.WriteString(strings.Join(db.tables, ","))
	query.WriteString(" ")

	// Join (optional)
	if len(db.joinTables) > 0 {
		query.WriteString(strings.Join(db.joinTables, " "))
		query.WriteString(" ")
	}

	// Where (optional)
	if len(db.whereFields) > 0 {
		query.WriteString(db.buildWhere())
	}

	// Order (optional)
	if db.order != "" {
		query.WriteString(" ORDER BY ")
		query.WriteString(db.order)
	}

	// Limit (optional)
	if db.limit != 0 {
		query.WriteString(" LIMIT ")
		query.WriteString(strconv.Itoa(db.limit))
	}

	// Offset (optional)
	if db.offset != 0 {
		query.WriteString(" OFFSET ")
		query.WriteString(strconv.Itoa(db.limit))
	}

	query.WriteString(";")
	qs := query.String()
	db.printLog(qs)
	return qs
}

// buildUpdate returns the generated SQL for the UPDATE statement. Panics if it can't generate a query
func (db *Database) buildUpdate() string {
	var query bytes.Buffer
	query.WriteString(db.buildCTE())
	query.WriteString("UPDATE ")

	// Model only
	if len(db.tables) != 1 {
		db.panicLog("Please set a table with Model to Update")
	}
	query.WriteString(db.tables[0])
	query.WriteString(" SET ")

	updateSize := len(db.updateCreateFields)
	if updateSize == 0 {
		db.panicLog("Unable to detect fields to update")
	}

	for j, field := range db.updateCreateFields {
		query.WriteString(handleIdentifier(field))
		query.WriteString(" = $")
		query.WriteString(strconv.Itoa(db.varCount))
		db.varCount++
		if j != updateSize-1 {
			query.WriteString(", ")
		}
	}

	// Where (optional)
	if len(db.whereFields) > 0 {
		query.WriteString(db.buildWhere())
	}

	query.WriteString(db.buildReturning())

	qs := query.String()
	db.printLog(qs)
	return qs
}

// buildCreate returns the generated SQL for the CREATE statement. Panics if it can't generate a query
func (db *Database) buildCreate() string {
	var query bytes.Buffer
	query.WriteString(db.buildCTE())
	query.WriteString("INSERT INTO ")

	// Model only
	if len(db.tables) != 1 {
		db.panicLog(fmt.Sprintf("Unable to infer table name for Create. Number of tables: %d", len(db.tables)))
	}
	// Table (
	query.WriteString(db.tables[0])
	query.WriteString("(")

	// field1,file2,...
	createSize := len(db.updateCreateFields)
	if createSize == 0 {
		db.panicLog("Unable to detect fields for Create")
	}

	query.WriteString(strings.Join(db.updateCreateFields, ","))
	// values($1,..$n) place holders
	query.WriteString(") VALUES (")
	for i := 0; i < createSize; i++ {
		query.WriteString("$")
		query.WriteString(strconv.Itoa(db.varCount))
		db.varCount++
		if i != createSize-1 {
			query.WriteString(",")
		}
	}

	query.WriteString(") ")
	query.WriteString(db.buildReturning())
	query.WriteString(";")

	qs := query.String()
	db.printLog(qs)
	return qs
}

// buildReturning returns the RETURNING part of the query
// it explicits every fields in the current model.
// In that way we're able to easily Scan the results
func (db *Database) buildReturning() string {
	var query bytes.Buffer
	query.WriteString(" RETURNING ")
	query.WriteString(strings.Join(getSQLFields(db.models[0]), ","))
	return query.String()
}

// buildCTE returns the CTE if defined
func (db *Database) buildCTE() string {
	// replaceMarks has been called previously, in order to prevent wrong parameter order
	return db.cte + " "
}

// buildDelete returns the generated SQL for the DELETE statement. Panics if it can't geerate a query
func (db *Database) buildDelete() string {
	var query bytes.Buffer
	query.WriteString(db.buildCTE())
	query.WriteString("DELETE FROM ")

	// Model only
	if len(db.tables) != 1 {
		db.panicLog("Unable to infer table name for Delete. Use Delete(model) or Model(model)")
	}
	query.WriteString(db.tables[0])

	// Where (mandatory)
	if len(db.whereFields) == 0 {
		db.panicLog("Where statement is mandatory in Delete")
	}

	query.WriteString(db.buildWhere())
	query.WriteString(";")

	qs := query.String()
	db.printLog(qs)
	return qs
}

// buildWhere returns the generated SQL for the WHERE clause. Panics if the Where method hasn't been called
// The generated query uses the PostgreSQL placeholders for query parameters in compiled queries
func (db *Database) buildWhere() string {
	var query bytes.Buffer
	whereSize := len(db.whereFields)
	if whereSize == 0 {
		db.panicLog("Please add a Where condition with .Where")
	}

	query.WriteString(" WHERE ")

	for j, clause := range db.whereFields {
		if strings.Contains(clause, "$") {
			query.WriteString(clause)
		} else {
			query.WriteString(handleIdentifier(clause))
			query.WriteString(" = $")
			query.WriteString(strconv.Itoa(db.varCount))
			db.varCount++
		}
		if j != whereSize-1 {
			query.WriteString(" AND ")
		}
	}
	query.WriteString(" ")
	return query.String()
}

// fieldValue returns an interface{} that's the value of fieldVal if fieldVal is not blank
// if fieldVal is blank and the field has a default value, return the defalt value
// oherwise returns nil
func fieldValue(fieldVal reflect.Value, structField reflect.StructField) (ret interface{}) {
	if isBlank(fieldVal) {
		defaultValue := strings.TrimSpace(parseTagSetting(structField.Tag.Get("sql"))["default"])
		if defaultValue != "" {
			switch fieldVal.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if numericValue, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
					if numericValue != fieldVal.Int() {
						return fieldVal.Int()
					}
					return numericValue
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if numericValue, err := strconv.ParseUint(defaultValue, 10, 64); err == nil {
					if numericValue != fieldVal.Uint() {
						return fieldVal.Int()
					}
					return numericValue
				}
			case reflect.Float32, reflect.Float64:
				if floatValue, err := strconv.ParseFloat(defaultValue, 64); err == nil {
					if floatValue != fieldVal.Float() {
						return fieldVal.Float()
					}
					return floatValue
				}
			case reflect.Bool:
				if boolValue, err := strconv.ParseBool(defaultValue); err == nil {
					if boolValue != fieldVal.Bool() {
						return fieldVal.Bool()
					}
					return boolValue
				}
			case reflect.String:
				return fieldVal.String()
			default:
				return nil
			}
		}
		return nil
	}
	return fieldVal.Interface()
}

// parseTagSetting, imported from jinzhu/gorm
func parseTagSetting(str string) map[string]string {
	tags := strings.Split(str, ";")
	setting := map[string]string{}
	for _, value := range tags {
		v := strings.Split(value, ":")
		k := strings.TrimSpace(strings.ToLower(v[0]))
		if len(v) >= 2 {
			setting[k] = strings.Join(v[1:], ":")
		} else {
			setting[k] = k
		}
	}
	return setting
}

// clone clones the current Database in order to be thread safe
func (db *Database) clone() *Database {
	clone := &Database{
		connection:       db.connection,
		db:               db.db,
		rawRows:          db.rawRows,
		logger:           db.logger,
		cte:              db.cte,
		selectFields:     db.selectFields,
		order:            db.order,
		limit:            db.limit,
		offset:           db.offset,
		varCount:         db.varCount,
		connectionString: db.connectionString,
		listener:         db.listener,
	}

	clone.tables = make([]string, len(db.tables))
	copy(clone.tables, db.tables)

	clone.joinTables = make([]string, len(db.joinTables))
	copy(clone.joinTables, db.joinTables)

	clone.models = make([]DBModel, len(db.models))
	copy(clone.models, db.models)

	clone.cteSelectValues = make([]interface{}, len(db.cteSelectValues))
	copy(clone.cteSelectValues, db.cteSelectValues)

	clone.updateCreateValues = make([]interface{}, len(db.updateCreateValues))
	copy(clone.updateCreateValues, db.updateCreateValues)

	clone.updateCreateFields = make([]string, len(db.updateCreateFields))
	copy(clone.updateCreateFields, db.updateCreateFields)

	clone.whereValues = make([]interface{}, len(db.whereValues))
	copy(clone.whereValues, db.whereValues)

	clone.whereFields = make([]string, len(db.whereFields))
	copy(clone.whereFields, db.whereFields)

	return clone
}
