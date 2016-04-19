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
package igor_test

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/galeone/igor"
)

var db *igor.Database
var e error

// Create a user igor and a db igor writeable by igor before to run tests

// Define models
type Profile struct {
	Counter        uint64 `igor:"primary_key"`
	Website        string
	Quotes         string
	Biography      string
	Github         string
	Skype          string
	Jabber         string
	Yahoo          string
	Userscript     string
	Template       uint8
	MobileTemplate uint8
	Dateformat     string
	Facebook       string
	Twitter        string
	Steam          string
	Push           bool
	Pushregtime    time.Time `sql:"default:(now() at time zone 'utc')"`
	Closed         bool
}

//TableName returns the table name associated with the structure
func (Profile) TableName() string {
	return "profiles"
}

// The User type do not have every field with a counter part on the db side
// as you can see in init(). The non present fields, have a default value associated and handled by the DBMS
type User struct {
	Counter          uint64    `igor:"primary_key"`
	Last             time.Time `sql:"default:(now() at time zone 'utc')"`
	NotifyStory      igor.JSON `sql:"default:'{}'::jsonb"`
	Private          bool
	Lang             string `sql:"default:en"`
	Username         string
	Password         string
	Email            string
	Name             string
	Surname          string
	Gender           bool
	BirthDate        time.Time `sql:"default:(now() at time zone 'utc')"`
	BoardLang        string    `sql:"default:en"`
	Timezone         string
	Viewonline       bool
	RegistrationTime time.Time `sql:"default:(now() at time zone 'utc')"`
	// Relation. Manually fill the field when required
	Profile Profile `sql:"-"`
}

//TableName returns the table name associated with the structure
func (User) TableName() string {
	return "users"
}

func init() {

	if db, e = igor.Connect("user=donotexists dbname=wat sslmode=error"); e == nil {
		panic(fmt.Sprintf("Connect with a wrong connection string shoudl fail, but succeeded"))
	}

	if db, e = igor.Connect("user=igor dbname=igor sslmode=disable"); e != nil {
		panic(e.Error())
	}

	// Exec raw query to create tables and test transactions (and Exec)
	tx := db.Begin()
	e = tx.Exec("DROP TABLE IF EXISTS users CASCADE")
	if e != nil {
		panic(e.Error())
	}

	e = tx.Exec(`CREATE TABLE users (
    counter bigserial NOT NULL PRIMARY KEY,
    last timestamp without time zone DEFAULT timezone('utc'::text, now()) NOT NULL,
    notify_story jsonb DEFAULT '{}'::jsonb NOT NULL,
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
	if e != nil {
		panic(e.Error())
	}

	e = tx.Exec("DROP TABLE IF EXISTS profiles CASCADE")
	if e != nil {
		panic(e.Error())
	}

	e = tx.Exec(`CREATE TABLE profiles (
    counter bigserial NOT NULL PRIMARY KEY,
    website character varying(350) DEFAULT ''::character varying NOT NULL,
    quotes text DEFAULT ''::text NOT NULL,
    biography text DEFAULT ''::text NOT NULL,
    github character varying(350) DEFAULT ''::character varying NOT NULL,
    skype character varying(350) DEFAULT ''::character varying NOT NULL,
    jabber character varying(350) DEFAULT ''::character varying NOT NULL,
    yahoo character varying(350) DEFAULT ''::character varying NOT NULL,
    userscript character varying(128) DEFAULT ''::character varying NOT NULL,
    template smallint DEFAULT 0 NOT NULL,
    dateformat character varying(25) DEFAULT 'd/m/Y, H:i'::character varying NOT NULL,
    facebook character varying(350) DEFAULT ''::character varying NOT NULL,
    twitter character varying(350) DEFAULT ''::character varying NOT NULL,
    steam character varying(350) DEFAULT ''::character varying NOT NULL,
    push boolean DEFAULT false NOT NULL,
    pushregtime timestamp without time zone DEFAULT timezone('utc'::text, now()) NOT NULL,
    mobile_template smallint DEFAULT 1 NOT NULL,
    closed boolean DEFAULT false NOT NULL,
    template_variables jsonb DEFAULT '{}'::jsonb NOT NULL
	)`)
	if e != nil {
		panic(e.Error())
	}

	e = tx.Exec("ALTER TABLE profiles ADD CONSTRAINT profiles_users_fk FOREIGN KEY(counter) references users(counter) ON DELETE CASCADE")
	if e != nil {
		panic(e.Error())
	}

	if e = tx.Commit(); e != nil {
		panic(e.Error())
	}
}

// createUser creates a test user (since the primary key is a bigserial, each call creates a new user)
func createUser() User {
	user := User{
		Username:  "igor",
		Password:  "please store hashed password",
		Name:      "Paolo",
		Surname:   "Galeone",
		Email:     "please validate the @email . com",
		Gender:    true,
		BirthDate: time.Now(),
	}

	if e = db.Create(&user); e != nil {
		panic(fmt.Sprintf("Create(&user) filling fields having no default shoud work, but got: %s\n", e.Error()))
	}
	return user
}

// createProfile creates the profile for a test user (since the primary key is a bigserial, each call creates a new user)
func createProfile(id uint64) Profile {
	profile := Profile{Counter: id}
	if e = db.Create(&profile); e != nil {
		panic(fmt.Sprintf("Create(&profile) failed: %s\n", e.Error()))
	}
	return profile
}

func TestModelCreateUpdatesSelectDelete(t *testing.T) {
	panicNumber := 0
	defer func() {
		// catch panic of db.Model(nil)
		if r := recover(); r != nil {
			if panicNumber == 0 {
				t.Log("All right")
				panicNumber++
			} else {
				t.Error("Too many panics")
			}
		}
	}()

	// must panic
	db.Model(nil)

	if db.Create(&User{}) == nil {
		t.Error("Create an user without assign a value to fileds that have no default should fail")
	}

	user := createUser()
	user.Profile = createProfile(user.Counter)

	// Testing first
	var p Profile
	if e = db.First(&p, user.Counter); e != nil {
		t.Errorf("First failed: %s\n", e.Error())
	}

	if !reflect.DeepEqual(p, user.Profile) {
		t.Error("Fetched profile should be deep equals to the created profile")
	}

	if user.Lang != "en" {
		t.Error("Auto update of struct fields having default values on the DBMS shoud work, but failed")
	}

	//change user language
	user.Lang = "it"
	if e = db.Updates(&user); e != nil {
		t.Errorf("Updates should work but got: %s\n", e.Error())
	}

	// Scan without parameters should fail
	if e = db.Model(User{}).Select("lang").Where(user).Scan(); e == nil {
		t.Error("Scan without a parameter should fail, but succeeded")
	}

	// Select lang stored in the db
	var lang string
	if e = db.Model(User{}).Select("lang").Where(user).Scan(&lang); e != nil {
		t.Errorf("Scan failed: %s\n", e.Error())
	}

	if lang != "it" {
		t.Errorf("The fetched language (%s) is different to the expected one (%s)\n", lang, user.Lang)
	}

	if e = db.Delete(&user); e != nil {
		t.Errorf("Delete of a user (using the primary key) shoudl work, but got: %s\n", e.Error())
	}

	// Now use is empty. Thus a new .Delete(&user) should fail
	if e = db.Delete(&user); e == nil {
		t.Error("Delete of an empty object should fail, but succeeded")
	}
}

func TestJoinsTableSelectDeleteWhere(t *testing.T) {
	logger := log.New(os.Stdout, "igor-log: ", log.LUTC)
	db.Log(logger)

	// create 6 user and profiles
	var ids []uint64
	for i := 0; i < 6; i++ {
		ids = append(ids, createUser().Counter)
		createProfile(uint64(i + 1))
	}

	var fetchedIds []uint64
	if e = db.Model(User{}).Order("counter asc").Pluck("counter", &fetchedIds); e != nil {
		t.Errorf("Pluck should work but got: %s\n", e.Error())
	}

	for i := 0; i < 6; i++ {
		if ids[i] != fetchedIds[i] {
			t.Errorf("Expected %d in position %d but got: %d\n", ids[i], i, fetchedIds[i])
		}
	}

	// select $1::int, $2::int, $3::it, counter from users join profiles on user.counter = profiles.counter
	// where user.counter = $4
	var one, two, three, four int
	u := (User{}).TableName()
	p := (Profile{}).TableName()
	if e = db.Select("?::int, ?::int, ?::int, "+u+".counter", 1, 2, 3).
		Table(u).
		Joins("JOIN "+p+" ON "+u+".counter = "+p+".counter").
		Where(&User{Counter: 4}).Scan(&one, &two, &three, &four); e != nil {
		t.Error(e.Error())
	}
	db.Log(nil)

	if one != 1 || two != 2 || three != 3 || four != 4 {
		t.Errorf("problem in scanning results, expected 1,2,3,4 got: %d,%d,%d,%d", one, two, three, four)
	}

	// Count
	var count uint8
	if e = db.Model(User{}).Count(&count); e != nil {
		t.Errorf("problem counting users: %s\n", e.Error())
	}

	if count != 6 {
		t.Errorf("Problem wiht count. Expeted 6 users but counted %d", count)
	}

	if e = db.Where("counter IN (?)", ids).Delete(User{}); e != nil {
		t.Errorf("delete in range should work but got: %s\n", e.Error())
	}

	// clear slice and pluck again
	fetchedIds = nil
	db.Model(User{}).Order("counter asc").Pluck("counter", &fetchedIds)
	if len(fetchedIds) != 0 {
		t.Errorf("delete in range failed, pluck returned ids that must have been deleted")
	}
}

func TestJSON(t *testing.T) {
	user := createUser()
	var emptyJson igor.JSON = make(igor.JSON)

	if !reflect.DeepEqual(user.NotifyStory, emptyJson) {
		t.Errorf("JSON notifyStory should be empty but got: %s instead of %s\n", user.NotifyStory, emptyJson)
	}

	var ns igor.JSON = make(igor.JSON)

	ns["0"] = struct {
		From    uint64 `json:"from"`
		To      uint64 `json:"to"`
		Message string `json:"message"`
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

	if !reflect.DeepEqual(ns, nsNew) {
		t.Errorf("fetched notify story is different from the saved one\n%s vs %s", ns, nsNew)
	}

	db.Delete(&user)
}

func TestNotifications(t *testing.T) {
	logger := log.New(os.Stdout, "igor-log: ", log.LUTC)
	db.Log(logger)
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

	// wait some time to handle all notifications
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
}

func TestCTE(t *testing.T) {
	//db.Log(nil)
	createUser()
	createUser()
	createUser()
	var usernames []string
	e = db.CTE(`WITH full_users_id AS (
		SELECT counter FROM users WHERE name = ?)`, "Paolo").Table("full_users_id as fui").Select("username").Joins("JOIN users ON fui.counter = users.counter").Scan(&usernames)
	if e != nil {
		t.Fatalf(e.Error())
	}
	if len(usernames) != 3 {
		t.Fatalf("Expected 3, but got: %d\n", len(usernames))
	}
	db.Model(User{}).Where("name", "Paolo").Delete(User{})
}
