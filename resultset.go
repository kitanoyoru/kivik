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
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	"github.com/go-kivik/kivik/v4/driver"
)

// ResultMetadata contains metadata about certain queries.
type ResultMetadata struct {
	// Offset is the starting offset where the result set started.
	Offset int64

	// TotalRows is the total number of rows in the view which would have been
	// returned if no limiting were used.
	TotalRows int64

	// UpdateSeq is the sequence id of the underlying database the view
	// reflects, if requested in the query.
	UpdateSeq string

	// Warning is a warning generated by the query, if any.
	Warning string

	// Bookmark is the paging bookmark, if one was provided with the result
	// set. This is intended for use with the Mango /_find interface, with
	// CouchDB 2.1.1 and later. Consult the official CouchDB documentation for
	// detailed usage instructions:
	// http://docs.couchdb.org/en/2.1.1/api/database/find.html#pagination
	Bookmark string
}

// ResultSet is an iterator over a multi-value query result set.
//
// Call Next() to advance the iterator to the next item in the result set.
//
// The Scan* methods are expected to be called only once per iteration, as
// they may consume data from the network, rendering them unusable a second
// time.
//
// Calling ScanDoc, ScanKey, ScanValue, ID, or Key before calling Next will
// operate on the first item in the resultset, then close the iterator
// immediately. This is for convenience in cases where only a single item is
// expected, so the extra effort of iterating is otherwise wasted.
type ResultSet interface {
	// Next prepares the next result value for reading. It returns true on
	// success or false if there are no more results or an error occurs while
	// preparing it. Err should be consulted to distinguish between the two.
	Next() bool

	// Err returns the error, if any, that was encountered during iteration.
	// Err may be called after an explicit or implicit Close.
	Err() error

	// Close closes the Rows, preventing further enumeration, and freeing any
	// resources (such as the http request body) of the underlying query. If
	// Next is called and there are no further results, Rows is closed
	// automatically and it will suffice to check the result of Err. Close is
	// idempotent and does not affect the result of Err.
	Close() error

	// Finish will consume any remaining results in the result set, then close
	// the iterator, and return any available query metadata.  Use this in
	// place of Close() if the result metadata is needed, otherwise Close()
	// may be more efficient, as Close() does not read any more data from the
	// network.
	Finish() (ResultMetadata, error)

	// ScanValue copies the data from the result value into the value pointed
	// at by dest. Think of this as a json.Unmarshal into dest.
	//
	// If the dest argument has type *[]byte, Scan stores a copy of the input
	// data. The copy is owned by the caller and can be modified and held
	// indefinitely.
	//
	// The copy can be avoided by using an argument of type *json.RawMessage
	// instead. After a ScanValue into a json.RawMessage, the slice is only
	// valid until the next call to Next or Close.
	//
	// For all other types, refer to the documentation for json.Unmarshal for
	// type conversion rules.
	ScanValue(dest interface{}) error

	// ScanDoc works the same as ScanValue, but on the doc field of the result.
	// It will return an error if the query does not include documents.
	ScanDoc(dest interface{}) error

	// ScanKey works the same as ScanValue, but on the key field of the result.
	// For simple keys, which are just strings, the Key() method may be easier
	// to use.
	ScanKey(dest interface{}) error

	// ID returns the ID of the most recent result.
	ID() string

	// Rev returns the document revision, when known. Not all resultsets (such
	// as those from views) include revision IDs, so this will be blank in such
	// cases.
	Rev() string

	// Key returns the Key of the most recent result as a raw JSON string. For
	// compound keys, the ScanKey() method may be more convenient.
	Key() string

	// QueryIndex returns the 0-based index of the query. For standard queries,
	// this is always 0. When multiple queries are passed to the view, this will
	// represent the query currently being iterated
	QueryIndex() int

	// EOQ returns true if the iterator has reached the end of a query in a
	// multi-query query. When EOQ is true, the row data will not have been
	// updated. It is common to simply `continue` in case of EOQ, unless you
	// care about the per-query metadata, such as offset, total rows, etc.
	EOQ() bool

	// Attachments returns an attachments iterator. At present, it is only set
	// by the Get() method when doing a multi-part get from CouchDB (which is
	// the default where supported). This may be extended to other cases in
	// the future.
	Attachments() *AttachmentsIterator
}

// baseRS provides no-op versions of common rows functions that aren't
// needed by every implementation, so that it can be embedded in other
// implementations
type baseRS struct{}

func (baseRS) Rev() string                       { return "" }
func (baseRS) EOQ() bool                         { return false }
func (baseRS) QueryIndex() int                   { return 0 }
func (baseRS) UpdateSeq() string                 { return "" }
func (baseRS) Attachments() *AttachmentsIterator { return nil }
func (baseRS) ScanKey(interface{}) error         { return nil }
func (baseRS) ScanValue(interface{}) error       { return nil }
func (baseRS) Key() string                       { return "" }

type rows struct {
	baseRS
	*iter
	rowsi driver.Rows
	err   error
}

var _ ResultSet = &rows{}

func (r *rows) Next() bool {
	return r.iter.Next()
}

func (r *rows) EOQ() bool {
	return r.iter.EOQ()
}

func (r *rows) Err() error {
	if r.err != nil {
		return r.err
	}
	return r.iter.Err()
}

func (r *rows) Close() error {
	return r.iter.Close()
}

func (r *rows) Finish() (ResultMetadata, error) {
	for r.Next() {
	}
	var warning, bookmark string
	if w, ok := r.rowsi.(driver.RowsWarner); ok {
		warning = w.Warning()
	}
	if b, ok := r.rowsi.(driver.Bookmarker); ok {
		bookmark = b.Bookmark()
	}
	return ResultMetadata{
		Offset:    r.rowsi.Offset(),
		TotalRows: r.rowsi.TotalRows(),
		UpdateSeq: r.rowsi.UpdateSeq(),
		Warning:   warning,
		Bookmark:  bookmark,
	}, r.Close()
}

type rowsIterator struct{ driver.Rows }

var _ iterator = &rowsIterator{}

func (r *rowsIterator) Next(i interface{}) error { return r.Rows.Next(i.(*driver.Row)) }

func newRows(ctx context.Context, rowsi driver.Rows) *rows {
	return &rows{
		iter:  newIterator(ctx, &rowsIterator{rowsi}, &driver.Row{}),
		rowsi: rowsi,
	}
}

func (r *rows) ScanValue(dest interface{}) (err error) {
	if r.err != nil {
		return r.err
	}
	runlock := r.makeReady(&err)
	defer runlock()
	row := r.curVal.(*driver.Row)
	if row.Error != nil {
		return row.Error
	}
	if row.ValueReader != nil {
		return json.NewDecoder(row.ValueReader).Decode(dest)
	}
	return json.Unmarshal(row.Value, dest)
}

func (r *rows) ScanDoc(dest interface{}) (err error) {
	if r.err != nil {
		return r.err
	}
	runlock := r.makeReady(&err)
	defer runlock()
	row := r.curVal.(*driver.Row)
	if err := row.Error; err != nil {
		return err
	}
	doc := row.Doc
	if row.DocReader != nil {
		return json.NewDecoder(row.DocReader).Decode(dest)
	}
	if doc != nil {
		return json.Unmarshal(doc, dest)
	}
	return &Error{HTTPStatus: http.StatusBadRequest, Message: "kivik: doc is nil; does the query include docs?"}
}

// ScanAllDocs loops through remaining documents in the resultset, and scans
// them into dest. Dest is expected to be a pointer to a slice or an array, any
// other type will return an error. If dest is an array, scanning will stop
// once the array is filled.  The iterator is closed by this method. It is
// possible that an error will be returned, and that one or more documents were
// successfully scanned.
func ScanAllDocs(r ResultSet, dest interface{}) error {
	return scanAll(r, dest, r.ScanDoc)
}

// ScanAllValues works like ScanAllDocs, but scans the values rather than docs.
func ScanAllValues(r ResultSet, dest interface{}) error {
	return scanAll(r, dest, r.ScanValue)
}

func scanAll(r ResultSet, dest interface{}, scan func(interface{}) error) (err error) {
	defer func() {
		closeErr := r.Close()
		if err == nil {
			err = closeErr
		}
	}()
	if err := r.Err(); err != nil {
		return err
	}

	value := reflect.ValueOf(dest)
	if value.Kind() != reflect.Ptr {
		return errors.New("must pass a pointer to ScanAllDocs")
	}
	if value.IsNil() {
		return errors.New("nil pointer passed to ScanAllDocs")
	}

	direct := reflect.Indirect(value)
	var limit int

	switch direct.Kind() {
	case reflect.Array:
		limit = direct.Len()
		if limit == 0 {
			return errors.New("0-length array passed to ScanAllDocs")
		}
	case reflect.Slice:
	default:
		return errors.New("dest must be a pointer to a slice or array")
	}

	base := value.Type()
	if base.Kind() == reflect.Ptr {
		base = base.Elem()
	}
	base = base.Elem()

	for i := 0; r.Next(); i++ {
		if limit > 0 && i >= limit {
			return nil
		}
		vp := reflect.New(base)
		err = scan(vp.Interface())
		if limit > 0 { // means this is an array
			direct.Index(i).Set(reflect.Indirect(vp))
		} else {
			direct.Set(reflect.Append(direct, reflect.Indirect(vp)))
		}
	}
	return nil
}

func (r *rows) ScanKey(dest interface{}) (err error) {
	if r.err != nil {
		return r.err
	}
	runlock := r.makeReady(&err)
	defer runlock()
	row := r.curVal.(*driver.Row)
	if err := row.Error; err != nil {
		return err
	}
	return json.Unmarshal(row.Key, dest)
}

func (r *rows) ID() string {
	runlock := r.makeReady(nil)
	defer runlock()
	return r.curVal.(*driver.Row).ID
}

func (r *rows) Key() string {
	runlock := r.makeReady(nil)
	defer runlock()
	return string(r.curVal.(*driver.Row).Key)
}

func (r *rows) QueryIndex() int {
	if qi, ok := r.rowsi.(driver.QueryIndexer); ok {
		return qi.QueryIndex()
	}
	return 0
}

// errRS is a resultset that has errored.
type errRS struct {
	baseRS
	err error
}

var _ ResultSet = &errRS{}

func (e *errRS) Err() error                      { return e.err }
func (e *errRS) Close() error                    { return e.err }
func (e *errRS) Finish() (ResultMetadata, error) { return ResultMetadata{}, e.err }
func (e *errRS) ID() string                      { return "" }
func (e *errRS) Key() string                     { return "" }
func (e *errRS) Next() bool                      { return false }
func (e *errRS) ScanAllDocs(interface{}) error   { return e.err }
func (e *errRS) ScanDoc(interface{}) error       { return e.err }
func (e *errRS) ScanKey(interface{}) error       { return e.err }
func (e *errRS) ScanValue(interface{}) error     { return e.err }
