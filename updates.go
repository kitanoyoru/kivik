// Licensed under the Apache License, Version 2.0 (the "License"); you may not
// use this file except in compliance with the License. You may obtain a copy of
// the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations under
// the License.

package kivik

import (
	"context"
	"net/http"

	"github.com/go-kivik/kivik/v4/driver"
)

// DBUpdates provides access to database updates.
type DBUpdates struct {
	*iter
}

type updatesIterator struct{ driver.DBUpdates }

var _ iterator = &updatesIterator{}

func (r *updatesIterator) Next(i interface{}) error { return r.DBUpdates.Next(i.(*driver.DBUpdate)) }

func newDBUpdates(ctx context.Context, onClose func(), updatesi driver.DBUpdates) *DBUpdates {
	return &DBUpdates{
		iter: newIterator(ctx, onClose, &updatesIterator{updatesi}, &driver.DBUpdate{}),
	}
}

// DBName returns the database name for the current update.
func (f *DBUpdates) DBName() string {
	runlock, err := f.rlock()
	if err != nil {
		return ""
	}
	defer runlock()
	return f.curVal.(*driver.DBUpdate).DBName
}

// Type returns the type of the current update.
func (f *DBUpdates) Type() string {
	runlock, err := f.rlock()
	if err != nil {
		return ""
	}
	defer runlock()
	return f.curVal.(*driver.DBUpdate).Type
}

// Seq returns the update sequence of the current update.
func (f *DBUpdates) Seq() string {
	runlock, err := f.rlock()
	if err != nil {
		return ""
	}
	defer runlock()
	return f.curVal.(*driver.DBUpdate).Seq
}

// DBUpdates begins polling for database updates.
func (c *Client) DBUpdates(ctx context.Context, options ...Options) *DBUpdates {
	updater, ok := c.driverClient.(driver.DBUpdater)
	if !ok {
		return &DBUpdates{errIterator(&Error{Status: http.StatusNotImplemented, Message: "kivik: driver does not implement DBUpdater"})}
	}

	if err := c.startQuery(); err != nil {
		return &DBUpdates{errIterator(err)}
	}

	updatesi, err := updater.DBUpdates(ctx, mergeOptions(options...))
	if err != nil {
		c.endQuery()
		return &DBUpdates{errIterator(err)}
	}
	return newDBUpdates(context.Background(), c.endQuery, updatesi)
}
