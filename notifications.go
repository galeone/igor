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
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// Listen executes `LISTEN channel`. Uses f to handle received notifications on chanel.
// On error logs error messages (if a logs exists)
func (db *Database) Listen(channel string, f func(payload ...string)) error {
	// Create a new listener only if Listen is called for the first time
	if db.listener == nil {
		db.listenerCallbacks = make(map[string]func(...string))

		reportProblem := func(ev pq.ListenerEventType, err error) {
			if err != nil && db.logger != nil {
				db.printLog(err.Error())
			}
		}
		db.listener = pq.NewListener(db.connectionString, 10*time.Second, time.Minute, reportProblem)

		if db.listener == nil {
			return errors.New("Unable to create a new listener")
		}

		// detach event handler
		go func() {
			for {
				select {
				case notification := <-db.listener.Notify:
					go db.listenerCallbacks[notification.Channel](notification.Extra)
				case <-time.After(90 * time.Second):
					go func() {
						if db.listener.Ping() != nil {
							db.printLog(fmt.Sprintf("Error checking server connection for channel %s\n", channel))
							return
						}
					}()
				}
			}
		}()
	}

	if _, alreadyIn := db.listenerCallbacks[channel]; alreadyIn {
		return errors.New("Already subscribed to channel " + channel)
	}

	db.listenerCallbacks[channel] = f

	if err := db.listener.Listen(channel); err != nil {
		return err
	}

	return nil
}

// Unlisten executes `UNLISTEN channel`. Unregister function f, that was registered with Listen(channel ,f).
func (db *Database) Unlisten(channel string) error {
	if db.listener == nil {
		return errors.New("You must create a new listener first, calling Listen(channel)")
	}

	if channel == "*" {
		return db.listener.UnlistenAll()
	}
	return db.listener.Unlisten(channel)
}

// UnlistenAll executes `UNLISTEN *`. Thus do not receive any notification from any channel
func (db *Database) UnlistenAll() error {
	return db.Unlisten("*")
}

// Notify sends a notification on channel, optional payloads are joined together and comma separated
func (db *Database) Notify(channel string, payload ...string) error {
	pl := strings.Join(payload, ",")
	if len(pl) > 0 {
		return db.Exec("SELECT pg_notify(?, ?)", channel, pl)
	}
	return db.Exec("NOTIFY " + handleIdentifier(channel))
}
