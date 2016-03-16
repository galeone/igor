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

// Thanks to: https://coussej.github.io/2016/02/16/Handling-JSONB-in-Go-Structs/

package igor

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON is the Go type used to handle JSON PostgreSQL type
type JSON map[string]interface{}

// Value implements driver.Valuer interface
func (js JSON) Value() (driver.Value, error) {
	return json.Marshal(js)
}

// Scan implements sql.Scanner interface
func (js *JSON) Scan(src interface{}) error {
	if src == nil {
		*js = make(JSON)
		return nil
	}
	source, ok := src.([]byte)
	if !ok {
		return errors.New("Type assertion .([]byte) failed.")
	}

	if err := json.Unmarshal(source, js); err != nil {
		return err
	}
	return nil
}
