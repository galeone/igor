# igor

igor is an abstraction layer for PostgreSQL, written in Go. Igor syntax is (almost) compatible with [GORM](https://github.com/jinzhu/gorm "The fantastic ORM library for Golang, aims to be developer friendly").

## When to use igor

You should use igor when your DBMS is PostgreSQL and you want to place an abstraction layer on top of it and do CRUD operations in a smart, easy, secure and fast way.

Thus with igor you __do not__ create a new schema. In general igor do not support DDL (you can do it with the `Raw` and `Exec`, but there are not method created ad-hoc for thir purpose).

## What igor does

- Always uses prepared statements: no sql injection and good performance.
- Supports transactions
- Uses a GORM like syntax
- Uses the same logic in insertion and update: handle default values in a coherent manner
- Uses GORM models and conventions (partially, see [Differences](#differences))
- Exploits PostgreSQL `RETURNING` statement to update models fiels with the updated values (even when changed on db side; e.g. when having a default value)
- Automatically handle reserved keywords when used as a table name or fields. Do not quote every field (that's not recommended) but only the ones conflicting with a reserved keyword.
 

## What is not igor

- An ORM (and thus a complete GORM replacement):
  - Do not support associations
  - Do not support callbacks
  - Do not have any specific method for data migration and DDL operations
  - Do not support soft delete

## Install

```go
go get -u github.com/galeone/igor
```

## GORM compatible

igor uses the same syntax of GORM. Thus in a great number of cases you can replace GORM with igor by only changing the import path.

__Warning__: igor is not a complete GORM replacement. See the [Differences](#differences).

## Model definition
Models are the [same used in GORM.](http://jinzhu.me/gorm/models.html#model-definition)

The main differences are:

- Do not handle associations. Thus, if you have a field that refers to another table, disable it with the annotation `sql:"-"` (see the code below).
- Every model __must__ implement the `igor.DBTable` interface. Therefore every model must have the method `TableName() string`, that returns the table name associated with the model.
- Every model __must__ explicit the primary key field (using the tag `gorm:"primary_key"`).

Like:

```go
type User struct {
	Counter uint64 `gorm:"primary_key"`
    Username string
    Password string
    Name string
    Surname string
    Profile Profile `sql:"-"`
}

type (User) TableName() string {
    return "users"
}
```

## Methods

### Connect
```go
import "github.com/galeone/igor"

func main() {
  db, err := igor.Connect("user=galeone dbname=igor sslmode=disable")
}
```

### Log
See: [Logger](#logger).

### Model
`Model(DBModel)` sets the table name for the current query

```go
var logged bool
var counter uint64

db.Model(User{}).Select("login(?, ?) AS logged, counter", username, password).Where("LOWER(username) = ?", username).Scan(&logged, &counter);
```

generates:
```sql
SELECT login($1, $2) AS logged, counter FROM users WHERE LOWER(username) = $3 ;
```

### Joins
Joins append the join string to the current model

```go
type Post struct {
	Hpid    uint64    `gorm:"primary_key"`
	From    uint64
	To      uint64
	Pid     uint64    `sql:"default:0"`
	Message string
	Time    time.Time `sql:"default:(now() at time zone 'utc')"`
	Lang    string
	News    bool
	Closed  bool
}

type UserPost struct {
	Post
}

func (UserPost) TableName() string {
    return "posts"
}

users := new(User).TableName()
posts := new(UserPost).TableName()

var userPosts []UserPost
db.Model(UserPost{}).Order("hpid DESC").
    Joins("JOIN "+users+" ON "+users+".counter = "+posts+".to").
    Where("\"to\" = ?", user.Counter).Scan(&userPost)
```

generates:
```go
SELECT posts.hpid,posts."from",posts."to",posts.pid,posts.message,posts."time",posts.lang,posts.news,posts.closed
FROM posts
JOIN users ON users.counter = posts.to
WHERE "to" = $1
```

### Table
 Table appends the table string to FROM. It has the same behavior of Model, but passing the table name directly as a string

See example in [Joins](#joins)

### Select
Select sets the fields to retrieve. Appends fields to SELECT (See example in [Model](#model)).

When select is not specified, every field is selected in the Model order (See example in [Joins](#joins)).

### Where
Where works with `DBModel`s or strings.

When using a `DBModel`, if the primary key fields is not blank, the query will generate a where clause in the form:

Thus:

```go
db.Model(UserPost{}).Where(&UserPost{Hpid: 1, From:1, To:1})
```

generates:

```sql
SELECT posts.hpid,posts."from",posts."to",posts.pid,posts.message,posts."time",posts.lang,posts.news,posts.closed
FROM posts
WHERE posts.hpid = $1
```

Ignoring values that are not primary keys.

If the primary key field is blank, generates the where clause `AND`ing the conditions:

```go
db.Model(UserPost{}).Where(&UserPost{From:1, To:1})
```

The conditions will be:

```sql
WHERE posts.from = $1 AND posts.to = $2
```

When using a string, you can use the `?` as placeholder for parameters substitution. Thus

```go
db.Model(UserPost{}).Where("\"to\" = ?", user.Counter)
```

generates:

 ```sql
SELECT posts.hpid,posts."from",posts."to",posts.pid,posts.message,posts."time",posts.lang,posts.news,posts.closed
FROM posts
WHERE "to" = $1
```

Wheere supports slices as well:

```go
db.Model(UserPost{}).Where("\"to\" IN (?) OR \"from\" = ?", []uint64{1,2,3,4,6}, 88)
```

generates:

```sql
SELECT posts.hpid,posts."from",posts."to",posts.pid,posts.message,posts."time",posts.lang,posts.news,posts.closed
FROM posts
WHERE "to" IN ($1,$2,$3,$4,$5) OR "from" = $6
```

### Create
Create `INSERT` a new row into the table specified by the DBModel.

`Create` handles default values using the following rules:

If a field is blank and has a default value and this defualt value is the Go Zero value for that field, do not generate the query part associated with the insertion of that fields (let the DBMS handle the default value generation).

If a field is blank and has a default value that's different from the Go Zero value for fhat filed, insert the specified default value.

Create exploits the `RETURNING` clause of PostgreSQL to fetch the new row and update the DBModel passed as argument.

In that way igor always have the up-to-date fields of DBModel.

```go
post := &UserPost{
    From: 1,
    To: 1,
    Pid: 10,
    Message: "hi",
    Lang: "en",
}
db.Create(post)
```

generates:

```sql
INSERT INTO posts("from","to",pid,message,lang) VALUES ($1,$2,$3,$4,$5)  RETURNING posts.hpid,posts."from",posts."to",posts.pid,posts.message,posts."time",posts.lang,posts.news,posts.closed;
```

The resulting row (the result of `RETURNING`) is used as a source for the  `Scan` method, having the DBModel as argument.

Thus, in the example, the varialble post.Time has the `(now() at time zone 'utc')` evaluation result value.

### Delete

See [Delete](#delete-1)

### Updates

Updates uses the same logic of [Create](#create) (thus the default value handling is the same).

The only difference is that Updates `UPDATE` rows.

`Update` tries to infer the table name from the DBModel passed as argument __if__ a `Where` clause has not been specified. Oterwise uses the `Where` clause to generate the `WHERE` part and the Model to generate the `field = $n` part.

```go
var user User
db.First(&user, 1) // hanlde errors
user.Username = "username changed"

db.Updates(&user)
```

generates:

```sql
UPDATE users SET users.username = "username changed" WHERE users.counter = 1 RETURNING users.counter,users.last,users.notify_story,users.private,users.lang,users.username,users.email,users.name,users.surname,users.gender,users.birth_date,users.board_lang,users.timezone,users.viewonline,users.registration_time
```

The `RETURNING` clause is handled in the same manner of [Create](#create).

### Pluck
Pluck fills the slice with the query result.
It calls `Scan` internally, thus the slice can be a slice of structures or a slice of simple types.

It panics if slice is not a slice or the query is not well formulated.

```go
type Blacklist struct {
	From       uint64
	To         uint64
	Motivation string
	Time       time.Time `sql:"default:(now() at time zone 'utc')"`
	Counter    uint64    `gorm:"primary_key"`
}

func (Blacklist) TableName() string {
	return "blacklist"
}

var blacklist []uint64
db.Model(Blacklist{}).Where(&Blacklist{From: user.Counter}).Pluck("\"to\"", &blacklist)
```

generates

```sql
SELECT "to" FROM blacklist WHERE blacklist."from" = $1
```

### Count

Count sets the query result to be count(*) and scan the result into value.

```go
var count int
db.Model(Blacklist{}).Where(&Blacklist{From: user.Counter}).Count(&count
```

generates:

```sql
SELECT COUNT(*) FROM blacklist WHERE blacklist."from" = $1
```

### First

See [First](#first-1)

### Scan

See [Scan and Find methods](#scan-and-find-methods)

### Raw

Prepares and executes a raw query, the results is avaiable for the Scan method.

See [Scan and Find methods](#scan-and-find-methods)

### Exec

Prepares and executes a raw query, the results is discarded. Useful when you don't need the query result or the operation have no result.

```go
tx := db.Begin()
tx.Exec("DROP TABLE IF EXISTS users")
tx.Exec(`CREATE TABLE users (
	counter bigint NOT NULL,
	last timestamp without time zone DEFAULT timezone('utc'::text, now()) NOT NULL,
	notify_story jsonb,
	private boolean DEFAULT false NOT NULL,
	lang character varying(2) DEFAULT 'en'::character varying NOT NULL,
	username character varying(90) NOT NULL,
	password character varying(60) NOT NULL,
	name character varying(60) NOT NULL,
	surname character varying(60) NOT NULL,
	email character varying(350) NOT NULL,
	gender boolean NOT NULL,
	birth_date date NOT NULL,
	board_lang character varying(2) DEFAULT 'en'::character varying NOT NULL,
	timezone character varying(35) DEFAULT 'UTC'::character varying NOT NULL,
	viewonline boolean DEFAULT true NOT NULL,
	remote_addr inet DEFAULT '127.0.0.1'::inet NOT NULL,
	http_user_agent text DEFAULT ''::text NOT NULL,
	registration_time timestamp(0) with time zone DEFAULT now() NOT NULL
	)`)
tx.Commit()
```

### Where

### Limit
Limit sets the LIMIT value to the query

### Offset
Offset sets the OFFSET value to the query

### Order
Order sets the ORDER BY value to the query

### DB
DB returns the current `*sql.DB`. It panics if called during a transaction

### Begin
Begin initialize a transaction. It panics if begin has been already called.

Il returns a `*igor.Database`, thus you can use every other `*Database` method on the returned value.

```go
tx := db.Begin()
```

### Commit
Commit commits the transaction. It panics if the transaction is not started (you have to call Begin before)

```go
tx.Create(&user)
tx.Commit()
// Now you can use the db variable again
```

### Rollback
Rollback rollbacks the transaction. It panics if the transaction is not started (you have to call Begin before

```go
if e := tx.Create(&user); e != nil {
    tx.Rollback()
} else {
    tx.Commit()
}
// Now you can use the db variable again

```

## Differences

### Select and Where call order
In GORM, you can execute
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
Igor models are __the same__ as GORM models (you must use the `gorm` tag field and `sql` tag fields as used in GORM).

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
In GORM, every method (even the ones that execute queries) returns a `*DB`.

In igor:
- methods that execute queries returns `error`
- methods that build the query returns `*Database`, thus you can chain the methods (a là GORM) and build the query.

### Scan and Find methods
In GORM, `Scan` method is used to scan query results into a struct. The `Find` method is almost the same.

In igor:
- `Scan` method executes the `SELECT` query. Thus return an error if `Scan` fails (see the previous section).
  
  `Scan` handle every type. You can scan query results in:
   - slice of struct `.Scan(&sliceOfStruct)`
   - single struct `.Scan(&singleStruct)`
   - single value `.Scan(&integerType)`
   - a comma separated list of values (because `Scan` is a variadic arguments function) `.Scan(&firstInteger, &firstString, &secondInteger, &floatDestinaton)`

- `Find` method does not exists, is completely replaced by `Scan`.

### Scan
In addiction to the previous section, there's another difference between GORM ad igor.

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


### Delete
In GORM, if you do not specify a primary key or a where clause (or if the value of the primary key is blank) the generated query will be
```
DELETE FROM <table>
```

That will delete everything from your table.

In igor this is not possible.

You __must__ specify a `Where` clause or pass to `Delete` a non empty model that will be used to build the where clause.

```go
db.Delete(&UserPost{}) // this panics

post := UserPost{
    Hpid: 10,
    From: 1,
}

db.Delete(&post)
//generates DELETE FROM posts WHERE hpid = $1, because hpid is a primary key

db.Where(&post).Delete(&UserPost{}) // ^ generates the same query

db.Delete(&UserPost{From:1,To:1})
// generates: DELETE FROM posts WHERE "from" = $1 AND "to" = $2
```

### First

In GORM `First` is used to get the first record, with or without a second parameter that is the primary key value.

In igor this is not possible. `First` works only with 2 parameter.

-  `DBModel`: that's the model you want to fill
-  `key interface{}` that's the primary key value, that __must__ be of the same type of the `DBModel` primary key.

```go
var user User
db.First(&user, uint64(1))

db.First(&user, "1") // panics, because "1" is not of the same type of user.Counter (uint64)
```

generates:

```sql
SELECT users.counter,users.last,users.notify_story,users.private,users.lang,users.username,users.email,users.name,users.surname,users.gender,users.birth_date,users.board_lang,users.timezone,users.viewonline,users.registration_time
FROM users
WHERE users.counter = $1
```

## Other

Every other GORM method is not implemented.

### Contributing

Do you want to add some new method to improve GORM compatibility or add some new method to improve igor?

Feel free to contribuite via Pull Request.

### License

Igor is relaeased under GNU Affero General Public License v3 (AGPL-3.0).

### About the author

Feel free to contact me (you can find my email address and other ways to contact me in my GitHub profile page).
