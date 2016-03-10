# igor

igor is a query builder for PostgreSQL, compatible with [GORM](https://github.com/jinzhu/gorm "The fantastic ORM library for Golang, aims to be developer friendly").

## When to use igor

You should use igor when your DBMS is PostgreSQL and you want to place an abstraction layer on top of it and do CRUD operations in a smart, easy, secure and fast way.

Thus with igor you __do not__ create a new schema. In general igor do not support DDL (you can do it with the `Raw` method, but is not officially supported).

## What igor does

- Always uses prepared statements: no sql injection and good performance.
- Supports transactions
- Uses a GORM like syntax
- Uses the same logic in insertion and update: handle default values in a coherent manner
- Uses GORM models and conventions (partially, see (Differences)[#differences])
- Exploits PostgreSQL `RETURNING` statement to update models fiels with the updated values (even when changed on db side; e.g. when having a default value)
- Automatically handle reserved keywords when used as a table name or fields. Do not quote every field (that's not recommended) but only the ones conflicting with a reserved keyword.
- 
 

## What is not igor

- An ORM
- A complete GORM replacement 

## GORM compatible

igor uses the same syntax of GORM. Thus in a great number of cases you can replace gorm with igor by only changing the import path.

__Warning__: igor is not _full_ compatible with GORM. See the [Differences](#differences).

## Methods

### Connect

### Log

### Model

### Joins

### Table

### Select

### Where

### Create

### Delete

### Updates

### Pluck

### Count

### First

### Scan

### Raw

### Where

### Limit

### Offset

### Order

### DB

### Begin

### Commit

### Rollback

## Differences

### Select and Where call order
In gorm, you can execute
```go
db.Model(User{}).Select("username")
```

```go
db.Select("username").Model(User{})
```

and achieve the same result.

In igor this is not possibile. You __must__ call `Model` before `Select`.

Thus always use: 

```go
db.Model(User{}).Select("username")
```

The reason is that igor generates queries in the form `SELECT table.field1, table.filed2 FROM table [WHERE] RETURNING  table.field1, table.filed2`.

In order to avoid ambiguities when using `Joins`, the `RETURNING` part of the query must be in the form `table.field1, table.filed2, ...`, and table is the `TableName()` result of the `DBModel` passed as `Model` argument.

### Models
Igor models are __the same__ as gorm models (you must use the `gorm` tag field and `sql` tag fields as used in gorm).

The only difference is that igor models require the implementation of the `DBModel` interface.

In GORM, you can optionally define the `TableName` method on your Model. With igor this is mandatory.

This constraint gives to igor the ability to generate conditions (like the `WHERE` or `INSERT` or `UPDATE` part of the query) that have a counter part on DB size for sure.

If a type does not implement the `DBModel` interface your program will not compile (and thus you can easily find the error and fix it). Otherwise igor could generate a wrong query and we're trying to avoid that.

### Open method
Since igor is PostgreSQL only, the `gorm.Open` method has been replaced with

```go
Connect(connectionString string) (*Database, error)
```

### Logger
There's no `db.LogMode(bool)` method in igor. If you want to log the prepared statements, you have to manually set a logger for igor.

```go
logger := log.New(os.Stdout, "query-logger: ", log.LUTC)
db.Log(logger)
```

If you want to disable the logger, set it to nil

```go
db.Log(nil)
```

Privacy: you'll __never__ see the values of the variables, but only the prepared statement and the PostgreSQL placeholders. Respect your user privacy, do not log user input (like credentials).

### Methods return value
In gorm, every method (even the ones that execute queries) returns a `*DB`.

In igor:
- methods that execute queries returns `error`
- methods that build the query returns `*Database`, thus you can chain the methods (a là gorm) and build the query.

### Scan and Find methods
In gorm, `Scan` method is used to scan query results into a struct. The `Find` method is almost the same.

In igor:
- `Scan` method executes the `SELECT` query. Thus return an error if `Scan` fails (see the previous section).
  
  `Scan` handle every type. You can scan query results in:
   - slice of struct `.Scan(&sliceOfStruct)`
   - single struct `.Scan(&singleStruct)`
   - single value `.Scan(&integerType)`
   - a comma separated list of values (because `Scan` is a variadic arguments function) `.Scan(&firstInteger, &firstString, &secondInteger, &floatDestinaton)`

- `Find` method does not exists, is completely replaced by `Scan`.

### Scan
In addiction to the previous section, there's another difference between gorm ad igor.

`Scan` method __do not__ scan selected fields into results using the selected fields name, but using the order (to increse the performance).

Thus, having:
```go
type Conversation struct {
	From   string    `json:"from"`
	Time   time.Time `json:"time"`
	ToRead bool      `json:"toRead"`
}

query := "SELECT DISTINCT otherid, MAX(times) as time, to_read " +
	"FROM (" +
	"(SELECT MAX(\"time\") AS times, \"from\" as otherid, to_read FROM pms WHERE \"to\" = ? GROUP BY \"from\", to_read)" +
	" UNION " +
	"(SELECT MAX(\"time\") AS times, \"to\" as otherid, FALSE AS to_read FROM pms WHERE \"from\" = ? GROUP BY \"to\", to_read)" +
	") AS tmp GROUP BY otherid, to_read ORDER BY to_read DESC, \"time\" DESC"

var convList []Conversation
err := Db().Raw(query, user.Counter, user.Counter).Scan(&convList)
```

Do not cause any problem, but if we change the SELECT clause, inverting the order, like

```go
query := "SELECT DISTINCT otherid, to_read, MAX(times) as time " +
...

```

Scan will fail because it will try to Scan the boolaan value in second position `to_read`, into the `time.Time` field of the Conversation structure.
