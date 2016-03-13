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
	"errors"
	"github.com/lib/pq"
	"strings"
	"time"
)

// Listen executes `LISTEN channel`. Uses f to handle received notifications on chanel.
// On error logs error messages (if a logs exists)
func (db *Database) Listen(channel string, f func(interface{})) error {
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil && db.logger != nil {
			db.printLog(err.Error())
		}
	}

	listener := pq.NewListener(db.connectionString, 10*time.Second, time.Minute, reportProblem)
	if listener == nil {
		return errors.New("Unable to create a new listener")
	}

	return db.Exec("LISTEN ?", handleIdentifier(channel))
}

// Unlisten executes `UNLISTEN channel`. Unregister function f, that was registred with Listen(chanenel ,f).
func (db *Database) Unlisten(channel string) error {
	return db.Exec("UNLISTEN ?", handleIdentifier(channel))
}

// Notify sends a notification on channel, optional payloads are joined together and comma separated
func (db *Database) Notify(channel string, payload ...string) error {
	pl := strings.Join(payload, ",")
	if len(pl) > 0 {
		return db.Exec("SELECT pg_notify(?::text, ?::text)", handleIdentifier(channel), pl)
	}
	return db.Exec("NOTIFY ?::text", handleIdentifier(channel))
}
