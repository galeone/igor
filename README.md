# igor
igor is an abstraction layer for PostgreSQL, written in Go. Igor syntax is (almost) compatible with [GORM](https://github.com/jinzhu/gorm "The fantastic ORM library for Golang, aims to be developer friendly").

[![GoDoc](https://godoc.org/github.com/galeone/igor?status.svg)](https://godoc.org/github.com/galeone/igor)
[![Build Status](https://travis-ci.org/galeone/igor.svg?branch=master)](https://travis-ci.org/galeone/igor)

## When to use igor
You should use igor when your DBMS is PostgreSQL and you want to place an abstraction layer on top of it and do CRUD operations in a smart, easy, secure and fast way.

Thus with igor you __do not__ create a new schema. In general igor does not support DDL (you can do it with the `Raw` and `Exec`, but there are not method created ad-hoc for this purpose).

## What igor does
- Always uses prepared statements: no sql injection and good performance.
- Supports transactions
- Supports PostgreSQL JSON and JSONB types with `igor.JSON`
- Supports PostgreSQL [LISTEN/NOTIFY](http://www.postgresql.org/docs/current/static/sql-notify.html)
- Uses a GORM like syntax
- Uses the same logic in insertion and update: handle default values in a coherent manner
- Uses GORM models and conventions (partially, see [Differences](#differences))
- Exploits PostgreSQL `RETURNING` statement to update models fiels with the updated values (even when changed on db side; e.g. when having a default value)
- Automatically handle reserved keywords when used as a table name or fields. Does not quote every field (that's not recommended) but only the ones conflicting with a reserved keyword.
 

## What igor is not
- An ORM (and thus a complete GORM replacement):
  - Does not support associations
  - Does not support callbacks
  - Does not have any specific method for data migration and DDL operations
  - Does not support soft delete

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

- Igor does not handle associations. Thus, if you have a field that refers to another table, disable it with the annotation `sql:"-"` (see the code below).
- Every model __must__ implement the `igor.DBTable` interface. Therefore every model must have the method `TableName() string`, that returns the table name associated with the model.
- Every model __must__ explicit the primary key field (using the tag `igor:"primary_key"`).
- Since igor does not deal with DDL, `sql:"type:<type>"` is ignored.

Like:

```go
type User struct {
	Counter uint64 `igor:"primary_key"`
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
- [Connect](#connect)
- [Log](#log)
- [Model](#model)
- [Joins](#joins)
- [Table](#table)
- [CTE](#cte)
- [Select](#select)
- [Where](#where)
- [Create](#create)
- [Delete](#delete)
- [Updates](#updates)
- [Pluck](#pluck)
- [Count](#count)
- [First](#first)
- [Scan](#scan)
- [Raw](#raw)
- [Exec](#exec)
- [Where](#where)
- [Limit](#limit)
- [Offset](#offset)
- [Order](#order)
- [DB](#db)
- [Begin](#begin)
- [Commit](#commit)
- [Rollback](#rollback)
- [Listen](#listen)
- [Unlisten](#unlisten)
- [UnlistenAll](#unlistenall)
- [Notify](#notify)

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
	Hpid    uint64    `igor:"primary_key"`
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
    Where(`"to" = ?`, user.Counter).Scan(&userPost)
```

generates:
```sql
SELECT posts.hpid,posts."from",posts."to",posts.pid,posts.message,posts."time",posts.lang,posts.news,posts.closed
FROM posts
JOIN users ON users.counter = posts.to
WHERE "to" = $1
```

### Table
Table appends the table string to FROM. It has the same behavior of Model, but passing the table name directly as a string

See example in [Joins](#joins)

### CTE
CTE allows to define a Common Table Expression that precedes the query.

__Warning__: use it with the [Table](#table) method.

```go
var usernames []string
db.CTE(`WITH full_users_id AS (
SELECT counter FROM users WHERE name = ?)`, "Paolo").
Table("full_users_id as fui").
Select("username").
Joins("JOIN users ON fui.counter = users.counter").Scan(&usernames)
```

generates:
```sql
WITH full_users_id AS (
  SELECT counter FROM users WHERE name = $1
)
SELECT username FROM full_users_id as fui JOIN users ON fui.counter = users.counter ;
```

### Select
Select sets the fields to retrieve. Appends fields to SELECT (See example in [Model](#model)).

When select is not specified, every field is selected in the Model order (See example in [Joins](#joins)).

__Warning__: calling `Select` using parameters without type is allowed only if the stored procedure on the DBMS define the type.

Eg: if we have a function on postresql that accepts two parameters like
```pgsql
login(_username text, _pass text, OUT ret boolean) RETURNS boolean
```
we can call this function in that way

```go
db.Select('login(?,?)', username, password)
```

But, if the DBMS can't infer the paramters (in every other case except the one previous mentioned), we __must__ make parameters type explicit.

This is due to the use of prepared statements.

```go
db.Select("?::int, ?::int, ?::int", 1, 2, 3)
```

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
db.Model(UserPost{}).Where(`"to" = ?`, user.Counter)
```

generates:

 ```sql
SELECT posts.hpid,posts."from",posts."to",posts.pid,posts.message,posts."time",posts.lang,posts.news,posts.closed
FROM posts
WHERE "to" = $1
```

Wheere supports slices as well:

```go
db.Model(UserPost{}).Where(`"to" IN (?) OR "from" = ?`, []uint64{1,2,3,4,6}, 88)
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

If a field is blank and has a default value and this defualt value is the Go Zero value for that field, igor does not generate the query part associated with the insertion of that fields (let the DBMS handle the default value generation).

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
	Counter    uint64    `igor:"primary_key"`
}

func (Blacklist) TableName() string {
	return "blacklist"
}

var blacklist []uint64
db.Model(Blacklist{}).Where(&Blacklist{From: user.Counter}).Pluck(`"to"`, &blacklist)
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
Where builds the WHERE clause.

If a primary key is present in the struct passed as argument only that field is used.

```go
user.Counter = 2
user.Name = "paolo"
db.Select("username").Where(&user)
```

generates:

```sql
SELECT username FROM users WHERE users.counter = $1
```

because `Counter` is the primary key.

If the primary key is blank every non empty field is and-end.

```go
user.Counter = 0 // 0 is a blank primary key
```

generates

```sql
SELECT username FROM users WHERE users.name = $1
```

You can use a string to build the where clause and pass parameters if needed.

```go
db.Model(User{}).Select("username").Where("counter IN (?) AND name ILIKE ?",[]uint64{1,2,4,5}, "nino")

```

generates:

```sql
SELECT username FROM users WHERE counter in ($1,$2,$3,$4) AND name ILIKE $5
```

If a where condition can't be generated, Where panics

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

### Listen
Listen executes `LISTEN channel`. Uses f to handle received notifications on chanel.

```go
if e := db.Listen("notification_channel", func(payload ...string) {
    if len(payload) > 0 {
        pl := strings.Join(payload, ",")
        fmt.Println("Received notification on channel notification_channel, having payload: " + pl)
    } else {
        fmt.Println("Received notification on channel notification_channel without payload")
    }
}); e != nil {
    // handle error
}
```

### Unlisten
Unlisten executes`UNLISTEN channel`. Unregister function f, that was registered with Listen(chanenel ,f).

```go
e := db.Unlisten("notification_channel")
// handle error
```

You can unlisten from every channel calling `db.Unlisten("*")` or using the method `UnlistenAll`

### UnlistenAll
UnlistenAll executes `UNLISTEN *`. Thus do not receive any notification from any channel.

### Notify
With Notify you can send a notification with or without payload on a channel.

```go
e = db.Notify("notification_channel") // empty payload
e = db.Notify("notification_channel", "payload 1", "payload 2", "test")
```

When seding a payload, the strings are joined together. Therfore the payload sent with previous call to `Notify` is: `payload 1, payload 2, test`

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
Igor models are __the same__ as GORM models (except that you have to use the `igor` tag field to define the primary key). The `sql` tag field is used to define default value and column value. Eg:

```go
type Test struct {
    ID      uint64 `igor:"primary_key column:id_column"`
    Time    time.Time `sql:"default:(now() at time zone 'utc')"`
}
```

The other main difference is that igor models require the implementation of the `DBModel` interface.

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

`Scan` method __do not__ scans the selected fields into results using the selected fields name, but uses the order (to increse the performance).

Thus, having:
```go
type Conversation struct {
	From   string    `json:"from"`
	Time   time.Time `json:"time"`
	ToRead bool      `json:"toRead"`
}

var convList []Conversation
err := Db().Raw(`SELECT DISTINCT otherid, MAX(times) as "time", to_read FROM (
    (SELECT MAX("time") AS times, "from" as otherid, to_read FROM pms WHERE "to" = ? GROUP BY "from", to_read)
    UNION
    (SELECT MAX("time") AS times, "to" as otherid, FALSE AS to_read FROM pms WHERE "from" = ? GROUP BY "to", to_read)
) AS tmp GROUP BY otherid, to_read ORDER BY to_read DESC, "time" DESC`, user.Counter, user.Counter).Scan(&convList)
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

### JSON and JSONB support
Igor supports PostgreSQL JSON and JSONB types natively.

Just define the field in the DBModel with the type `igor.JSON`.
After that, you can work with JSON in the following way:

```go
user := createUser()

var ns igor.JSON = make(igor.JSON) // use it like a map[string]interface{}

ns["0"] = struct {
    From    uint64 `json:from`
    To      uint64 `json:to`
    Message string `json:message`
}{
    From:    1,
    To:      1,
    Message: "hi bob",
}
ns["numbers"] = 1
ns["test"] = 2

user.NotifyStory = ns

if e = db.Updates(&user); e != nil {
    t.Errorf("updates should work but got: %s\n", e.Error())
}

// To use JSON with json, use:
// printableJSON, _ := json.Marshal(user.NotifyStory)
// fmt.Printf("%s\n", printableJSON)

var nsNew igor.JSON
if e = db.Model(User{}).Select("notify_story").Where(&user).Scan(&nsNew); e != nil {
    t.Errorf("Problem scanning into igor.JSON: %s\n", e.Error())
}
```

### LISTEN / NOTIFY support
PostgreSQL give us a beautiful method to avoid polling the DBMS, using a simple publish/subscribe model over database connections (read more on the [docs](http://www.postgresql.org/docs/current/static/sql-notify.html)).

Igor gives you the ability to generate notification and subscribe to notifications sent over a channel, using the methods `Listen` and `Notify`.

Bevelow there's a working example:

```go
count := 0
if e = db.Listen("notification_without_payload", func(payload ...string) {
    count++
    t.Log("Received notification on channel: notification_without_payload\n")
}); e != nil {
    t.Fatalf("Unable to listen on channel: %s\n", e.Error())
}

for i := 0; i < 4; i++ {
    if e = db.Notify("notification_without_payload"); e != nil {
        t.Fatalf("Unable to send notification: %s\n", e.Error())
    }
}

// wait some time to handle all notifications, because are asynchronous
time.Sleep(100 * time.Millisecond)
if count != 4 {
    t.Errorf("Expected to receive 4 notifications, but counted only: %d\n", count)
}

// listen on an opened channel should fail
if e = db.Listen("notification_without_payload", func(payload ...string) {}); e == nil {
    t.Errorf("Listen on an opened channel should fail, but succeeded\n")
}

// Handle payload

// listen on more channels, with payload
count = 0
if e = db.Listen("np", func(payload ...string) {
    count++
    t.Logf("channel np: received payload: %s\n", payload)
}); e != nil {
    t.Fatalf("Unable to listen on channel: %s\n", e.Error())
}

// test sending payload with notify
for i := 0; i < 4; i++ {
    if e = db.Notify("np", strconv.Itoa(i)+" payload"); e != nil {
        t.Fatalf("Unable to send notification with payload: %s\n", e.Error())
    }
}

// wait some time to handle all notifications
time.Sleep(100 * time.Millisecond)
if count != 4 {
    t.Errorf("Expected to receive 4 notifications, but counted only: %d\n", count)
}

// test unlisten
if e = db.Unlisten("notification_without_payload"); e != nil {
    t.Errorf("Unable to unlisten from notification_without_payload, got: %s\n", e.Error())
}

// test UnlistenAll
if e = db.UnlistenAll(); e != nil {
    t.Errorf("Unable to unlistenAll, got: %s\n", e.Error())
}

```

### Contributing
Do you want to add some new method to improve GORM compatibility or add some new method to improve igor?

Feel free to contribuite via Pull Request.

### Testing
To test igor, you must create a igor user on PostgreSQL and make it own the igor database.
On Archlinux, with `postgres` as the PostgreSQL superuser this can be achieved by:

```sh
createuser -U postgres igor
createdb -U postgres igor igor
```

You can run tests with the usual command:

```sh
go test
```

### License
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

### About the author

Feel free to contact me (you can find my email address and other ways to contact me in my GitHub profile page).
