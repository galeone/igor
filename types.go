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
	"database/sql"
	"log"

	"github.com/lib/pq"
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
	cte                string
	cteSelectValues    []interface{}
	selectFields       string
	updateCreateValues []interface{}
	updateCreateFields []string
	whereValues        []interface{}
	whereFields        []string
	order              string
	limit              int
	offset             int
	varCount           int
	connectionString   string
	listener           *pq.Listener
	listenerCallbacks  map[string]func(...string)
}
