package igor

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"
	// Blank import required to get PostgreSQL working
	_ "github.com/lib/pq"
)

// Connect opens the connection to PostgreSQL using connectionString
func Connect(connectionString string) (*Database, error) {
	var e error
	db := new(Database)
	if db.db, e = sql.Open("postgres", connectionString); e != nil {
		return nil, e
	}
	db.clear()
	return db, nil
}

// Log sets the query logger
func (db *Database) Log(logger *log.Logger) *Database {
	db.logger = logger
	return db
}

// Model sets the table name for the current query
func (db *Database) Model(model DBModel) *Database {
	db.tables = append(db.tables, handleIdentifier(model.TableName()))
	db.models = append(db.models, model)
	return db
}

// Joins append the join string to the current model
func (db *Database) Joins(joins string) *Database {
	db.joinTables = append(db.joinTables, joins)
	// we can't infer model from the join string (can contain everything)
	return db
}

// Table appends the table string to FROM. It has the same behavior of Model, but
// passing the tablename directly as a string
func (db *Database) Table(table string) *Database {
	return db.Joins(table)
}

// Select sets the fields to retrieve. Appends fields to SELECT
func (db *Database) Select(fields string, args ...interface{}) *Database {
	db.selectFields = db.replaceMarks(fields)
	db.selectValues = args
	return db
}

// Delete executes DELETE FROM value.TableName where .Where()
// Calling .Where is mandatory. You can pass a nil pointer to value if you just setted
// the tablename with Model.
func (db *Database) Delete(value DBModel) error {
	defer db.clear()
	// if Model has been called, skip table name inference procedure
	if len(db.tables) == 0 {
		db.tables = append(db.tables, handleIdentifier(value.TableName()))
		db.models = append(db.models, value)
	}

	// If where is empty, try to infer a primary key by value
	// otherwise buildDelete panics (where is mandatory)
	db.Where(value)

	// Compile query
	var stmt *sql.Stmt
	var err error
	if stmt, err = db.db.Prepare(db.buildDelete()); err != nil {
		return err
	}

	// Pass query parameters and executes the query
	if _, err = stmt.Exec(db.whereValues...); err != nil {
		return err
	}

	// Clear fields of value after delete, because the object no more exists
	value = reflect.Zero(reflect.ValueOf(value).Type()).Interface().(DBModel)

	return nil
}

// Updates looks for non blank fields in value, extract its value and generate the
// UPDATE value.TableName() SET <field_name> = <value> query part.
// It handles default values when the field is empty.
func (db *Database) Updates(value DBModel) error {
	// Build where condition for update
	err := db.Where(value).commonCreateUpdate(value, db.buildUpdate)
	return err
}

// Create creates a new row into the Database, of type value and with its fields
func (db *Database) Create(value DBModel) error {
	err := db.commonCreateUpdate(value, db.buildCreate)
	return err
}

// Pluck fills the slice with the query result.
// *Executes the query* (calls Scan internally).
// Panics if slice is not a slice or the query is not well formulated
func (db *Database) Pluck(column string, slice interface{}) error {
	dest := reflect.Indirect(reflect.ValueOf(slice))
	if dest.Kind() != reflect.Slice {
		db.panicLog(fmt.Sprintf("slice should be a slice, not %s\n", dest.Kind()))
	}

	db = db.Select(column)
	return db.Scan(slice)
}

// Count sets the query result to be count(*) and scan the result into value.
// *It executes the query* (calls Scan internally).
// It panics if the query is not well formulated.
func (db *Database) Count(value *uint8) error {
	db = db.Select("count(*)")
	return db.Scan(value)
}

// First Scans the result of the selection query of type model using the specified id
// Panics if key is not compatible with the primary key filed type or if the query formulation fails
func (db *Database) First(dest DBModel, key interface{}) error {
	modelKey, _ := primaryKey(dest)
	in := reflect.Indirect(reflect.ValueOf(dest))
	in.FieldByName(modelKey).Set(reflect.Indirect(reflect.ValueOf(key)))
	return db.Model(dest).Where(in.Interface()).Scan(dest)
}

// Scan build the SELECT query and scans the query result query into dest.
// Panics if scan fails or the query fail
func (db *Database) Scan(dest ...interface{}) error {
	ld := len(dest)
	if ld == 0 {
		return errors.New("Required at least one parameter to Scan method")
	}

	var err error
	var rows *sql.Rows
	destIndirect := reflect.Indirect(reflect.ValueOf(dest[0]))
	if db.rawRows == nil {
		// Compile query
		var stmt *sql.Stmt
		// If the destionation is a struct (or a slice of struct)
		// select should select only exported sql fields in the order declared in the struct
		// This thing should go only if the user does not selected with .Select the fields to export
		// Thus only if db.selectFields == ""
		if db.selectFields == "" {
			if ld == 1 { // if is a struct or a slice of struct
				switch destIndirect.Kind() {
				case reflect.Struct:
					db.selectFields = strings.Join(getSQLFields(destIndirect.Interface().(DBModel)), ",")
				case reflect.Slice:
					// hanlde slice of structs and slice of pointers to struct
					sliceType := destIndirect.Type().Elem()
					if sliceType.Kind() == reflect.Ptr {
						return errors.New("Do not use a slice of pointers. Use a slice of real values. E.g. use []int instead of []*int")
					}
					if sliceType.Kind() == reflect.Struct {
						db.selectFields = strings.Join(getSQLFields(reflect.Indirect(reflect.New(sliceType)).Interface().(DBModel)), ",")
					} else {
						panic(sliceType)
					}
				}
			}
		}

		if stmt, err = db.db.Prepare(db.buildSelect()); err != nil {
			return err
		}

		// Pass query parameters and execute it
		if rows, err = stmt.Query(append(db.selectValues, db.whereValues...)...); err != nil {
			return err
		}
	} else {
		rows = db.rawRows
	}

	defer func() {
		rows.Close()
		db.clear()
	}()

	if ld == 1 {
		// if is a slice, find first element to decide how to use scan
		// oterwhise use destIndirect
		var defaultElem reflect.Value
		var slicePtr reflect.Value
		switch destIndirect.Kind() {
		// slice
		case reflect.Slice:
			// create a new element, because slice usually is empty. Thus we have to dinamically create it
			defaultElem = reflect.Indirect(reflect.New(destIndirect.Type().Elem()))
			// Create a pointer to a slice value and set it to the slice
			realSlice := reflect.ValueOf(dest[0])
			slicePtr = reflect.New(realSlice.Type())
		default:
			defaultElem = destIndirect
		}

		// if defaultElem is a struct, extract its fields, pass it to scan (extracts the address)
		var interfaces []interface{}
		if defaultElem.Kind() == reflect.Struct {
			fields := getFields(defaultElem.Interface())
			for _, field := range fields {
				interfaces = append(interfaces, reflect.Indirect(defaultElem.FieldByName(field.Name)).Addr().Interface())
			}
		} else {
			// else convert defaultElem into interfaces, use the address
			interfaces = append(interfaces, defaultElem.Addr().Interface())
		}

		for rows.Next() {
			// defaultElem fields are filled by Scan (scan result into fields as variadic arguments)
			if err = rows.Scan(interfaces...); err != nil {
				return err
			}
			// append result to dest (if the destination is a slice)
			if slicePtr.IsValid() {
				destIndirect.Set(reflect.Append(destIndirect, reflect.Indirect(defaultElem)))
				//x := reflect.Indirect(slicePtr.Elem())
				//x.Set(reflect.Append(x, reflect.ValueOf(defaultElem.Interface())))
				//defaultElem = reflect.Zero(destIndirect.Type().Elem())
			}
		}
	} else {
		// Scan(field1, field2, ...)
		for rows.Next() {
			if err = rows.Scan(dest...); err != nil {
				return err
			}
		}
	}
	return nil
}

// Raw executes a raw query, replacing placeholders (?) with the one supported by PostgreSQL
// Prepare the statement only. Call Scan to execute itOA
func (db *Database) Raw(query string, args ...interface{}) *Database {
	// Replace ? with $n
	query = db.replaceMarks(query)
	// Append args content to current values
	db.whereValues = append(db.whereValues, args...)

	// Compile query
	var stmt *sql.Stmt
	var err error
	if stmt, err = db.db.Prepare(query); err != nil {
		db.panicLog(err.Error())
	}

	// Pass query parameters and executes the query
	if db.rawRows, err = stmt.Query(db.whereValues...); err != nil {
		db.panicLog(err.Error())
	}
	return db
}

// Where builds the WHERE clause. If a primary key is present in the struct
// only that field is used. Otherwise, every non empty field is ANDed
// s can be a struct, in that case args are ignored
// or it can be a string, in that case args are the query parameters. Use ? placeholder
// If a where condition can't be generated it panics
func (db *Database) Where(s interface{}, args ...interface{}) *Database {
	if reflect.TypeOf(s).Kind() == reflect.String {
		whereClause := reflect.ValueOf(s).String()
		// replace question marks with $n
		// handle cases like .Where("a = ? and b in (?)", 1, []int{1,2,4,6})
		// this must become: a = $1 and b in ($2, $3, $4, $5)
		var slicePos []int

		// since I'm looping through args, I'll build the whereFileds with expanded slices if present
		var whereArgsExtended []interface{}
		for i := 0; i < len(args); i++ {
			if reflect.TypeOf(args[i]).Kind() == reflect.Slice {
				slicePos = append(slicePos, i)
				slice := reflect.Indirect(reflect.ValueOf(args[i]))
				for k := 0; k < slice.Len(); k++ {
					whereArgsExtended = append(whereArgsExtended, reflect.Indirect(slice.Index(k)).Interface())
				}
			} else {
				whereArgsExtended = append(whereArgsExtended, args[i])
			}
		}

		if len(slicePos) > 0 {
			var buffer bytes.Buffer
			// build new where clause, using old where clause until we don't reach the ? associated with the
			// slice. Then replace that ? with len(slice) question marks.
			markCount := 0
			slicePosLen := len(slicePos)
			for _, c := range whereClause {
				if c == '?' {
					s := sort.SearchInts(slicePos, markCount)
					// if found a ? associated with a slice
					if s < slicePosLen && slicePos[s] == markCount {
						sliceLen := reflect.Indirect(reflect.ValueOf(args[markCount])).Len()
						for i := 0; i < sliceLen; i++ {
							buffer.WriteRune('?')
							if i != sliceLen-1 {
								buffer.WriteRune(',')
							}
						}
					} else {
						// if the ? is not associated with a ?, write it as is
						buffer.WriteRune(c)
					}
					markCount++
				} else {
					buffer.WriteRune(c)
				}
			}
			// build the new where clause and pass it to replaceMarks
			fmt.Println(buffer.String())
			db.whereFields = append(db.whereFields, db.replaceMarks(buffer.String()))
			db.whereValues = append(db.whereValues, whereArgsExtended...)
		} else {
			db.whereFields = append(db.whereFields, db.replaceMarks(whereClause))
			db.whereValues = append(db.whereValues, args...)
		}
	} else {
		// must be a struct
		in := getStruct(s)
		key, value := primaryKey(s)

		// if a model has not been setted, set the model as s.TableName()
		if len(db.tables) == 0 {
			db.Model(s.(DBModel))
		}

		escapedTableName := handleIdentifier(s.(DBModel).TableName())

		if key != "" && !isBlank(reflect.ValueOf(value)) {
			db.whereFields = append(db.whereFields, escapedTableName+"."+handleIdentifier(key))
			db.whereValues = append(db.whereValues, value)
		} else {
			for i := 0; i < in.NumField(); i++ {
				fieldValue := in.Field(i)
				fieldType := in.Type().Field(i)
				if !isBlank(fieldValue) {
					db.whereFields = append(db.whereFields, escapedTableName+"."+getColumnName(fieldType))
					db.whereValues = append(db.whereValues, fieldValue.Interface())
				}
			}
		}
	}
	return db
}

// Limit sets the LIMIT value to the query
func (db *Database) Limit(limit int) *Database {
	db.limit = limit
	return db
}

// Offset sets the OFFSET value to the query
func (db *Database) Offset(offset int) *Database {
	db.offset = offset
	return db
}

// Order sets the ORDER BY value to the query
func (db *Database) Order(value string) *Database {
	db.order = handleIdentifier(value)
	return db
}

// DB returns the current `*sql.DB`
// panics if called during a transaction
func (db *Database) DB() *sql.DB {
	return db.db.(*sql.DB)
}

// Transactions

// Begin initialize a transaction
// panics if begin has been already called
func (db *Database) Begin() *Database {
	// Initialize transaction
	var tx *sql.Tx
	var err error
	if tx, err = db.db.(*sql.DB).Begin(); err != nil {
		db.printLog(err.Error())
		return nil
	}
	// backup db.db into db.connection
	db.connection = db.db.(*sql.DB)
	// replace db.db with the transaction
	db.db = tx
	return db
}

// Commit commits the transaction.
// Panics if the transaction is not started (you have to call Begin before)
func (db *Database) Commit() error {
	err := db.db.(*sql.Tx).Commit()
	// restore connection
	db.db = db.connection
	db.clear()
	return err
}

// Rollback rollbacks the transaction
// Panics if the transaction is not started (you have to call Begin before)
func (db *Database) Rollback() error {
	err := db.db.(*sql.Tx).Rollback()
	// restore connection
	db.db = db.connection
	db.clear()
	return err
}
