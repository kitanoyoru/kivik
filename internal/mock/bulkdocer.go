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

package mock

import (
	"context"

	"github.com/go-kivik/kivik/v4/driver"
)

// BulkDocer mocks a driver.DB and driver.BulkDocer
type BulkDocer struct {
	*DB
	BulkDocsFunc func(ctx context.Context, docs []interface{}, options map[string]interface{}) ([]driver.BulkResult, error)
}

var _ driver.BulkDocer = &BulkDocer{}

// BulkDocs calls db.BulkDocsFunc
func (db *BulkDocer) BulkDocs(ctx context.Context, docs []interface{}, options map[string]interface{}) ([]driver.BulkResult, error) {
	return db.BulkDocsFunc(ctx, docs, options)
}
