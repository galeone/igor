package igor_test

import (
	"github.com/galeone/igor"
	"time"
	//	"testing"
)

var db *igor.Database
var e error

// Create a user igor and a db igor writeable by igor before to run tests

// Define models
type Profile struct {
	Counter        uint64 `gorm:"primary_key"`
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

type User struct {
	Counter          uint64    `gorm:"primary_key"`
	Last             time.Time `sql:"default:(now() at time zone 'utc')"`
	NotifyStory      []byte
	Private          bool
	Lang             string
	Username         string
	Email            string
	Name             string
	Surname          string
	Gender           bool
	BirthDate        time.Time `sql:"default:(now() at time zone 'utc')"`
	BoardLang        string
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
	if db, e = igor.Connect("user=igor dbname=igor sslmode=disable"); e != nil {
		panic(e.Error())
	}

	// Exec raw query to create tables and test transactions (and Exec)
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

	tx.Exec("DROP TABLE IF EXISTS profiles")
	tx.Exec(`CREATE TABLE profiles (
    counter bigint NOT NULL,
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

	if e = tx.Commit(); e != nil {
		panic(e.Error())
	}
}
