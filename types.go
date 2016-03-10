package igor

import (
	"database/sql"
	"log"
)

// DBModel is the interface implemented by every struct that is a relation on the DB
type DBModel interface {
	//TableName returns the associated table name
	TableName() string
}

// TxDB Interface to wrap methods common to *sql.Tx and *sql.DB
type TxDB interface {
	Prepare(query string) (*sql.Stmt, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// Database is IGOR
type Database struct {
	connection         TxDB
	db                 TxDB
	rawRows            *sql.Rows
	tables             []string
	joinTables         []string
	models             []DBModel
	logger             *log.Logger
	selectValues       []interface{}
	selectFields       string
	updateCreateValues []interface{}
	updateCreateFields []string
	whereValues        []interface{}
	whereFields        []string
	order              string
	limit              int
	offset             int
	varCount           int
}
