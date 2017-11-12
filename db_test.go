package kivik

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/flimzy/diff"
	"github.com/flimzy/testy"

	"github.com/flimzy/kivik/driver"
	"github.com/flimzy/kivik/errors"
)

func TestClient(t *testing.T) {
	client := &Client{}
	db := &DB{client: client}
	result := db.Client()
	if result != client {
		t.Errorf("Unexpected result. Expected %p, got %p", client, result)
	}
}

func TestName(t *testing.T) {
	dbName := "foo"
	db := &DB{name: dbName}
	result := db.Name()
	if result != dbName {
		t.Errorf("Unexpected result. Expected %s, got %s", dbName, result)
	}
}

type dummyDB struct {
	driver.DB
}

var _ driver.DB = &dummyDB{}

func TestFlushNotSupported(t *testing.T) {
	db := &DB{
		driverDB: &dummyDB{},
	}
	err := db.Flush(context.Background())
	if StatusCode(err) != StatusNotImplemented {
		t.Errorf("Expected NotImplemented, got %s", err)
	}
}

type errorReader struct{}

var _ io.Reader = &errorReader{}

func (r *errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("errorReader")
}

func TestNormalizeFromJSON(t *testing.T) {
	type njTest struct {
		Name     string
		Input    interface{}
		Expected interface{}
		Status   int
		Error    string
	}
	tests := []njTest{
		{
			Name:     "Interface",
			Input:    int(5),
			Expected: int(5),
		},
		{
			Name:   "InvalidJSON",
			Input:  []byte(`invalid`),
			Status: StatusBadRequest,
			Error:  "invalid character 'i' looking for beginning of value",
		},
		{
			Name:     "Bytes",
			Input:    []byte(`{"foo":"bar"}`),
			Expected: map[string]interface{}{"foo": "bar"},
		},
		{
			Name:     "RawMessage",
			Input:    json.RawMessage(`{"foo":"bar"}`),
			Expected: map[string]interface{}{"foo": "bar"},
		},
		{
			Name:     "ioReader",
			Input:    strings.NewReader(`{"foo":"bar"}`),
			Expected: map[string]interface{}{"foo": "bar"},
		},
		{
			Name:   "ErrorReader",
			Input:  &errorReader{},
			Status: StatusUnknownError,
			Error:  "errorReader",
		},
	}
	for _, test := range tests {
		func(test njTest) {
			t.Run(test.Name, func(t *testing.T) {
				result, err := normalizeFromJSON(test.Input)
				var msg string
				var status int
				if err != nil {
					msg = err.Error()
					status = StatusCode(err)
				}
				if msg != test.Error || status != test.Status {
					t.Errorf("Unexpected error: %d %s", status, msg)
				}
				if err != nil {
					return
				}
				if d := diff.Interface(test.Expected, result); d != nil {
					t.Error(d)
				}
			})
		}(test)
	}
}

type putGrabber struct {
	*dummyDB
	lastPut  interface{}
	lastOpts map[string]interface{}

	newRev string
	err    error
}

func (db *putGrabber) Put(_ context.Context, _ string, i interface{}, opts map[string]interface{}) (string, error) {
	db.lastPut = i
	db.lastOpts = opts
	return db.newRev, db.err
}

func TestPut(t *testing.T) {
	type putTest struct {
		name     string
		db       *DB
		docID    string
		input    interface{}
		options  Options
		expected interface{}
		status   int
		err      string
		newRev   string
	}
	tests := []putTest{
		{
			name:   "no docID",
			status: StatusBadRequest,
			err:    "kivik: docID required",
		},
		{
			name:   "db error",
			db:     &DB{driverDB: &putGrabber{err: errors.Status(StatusBadRequest, "db error")}},
			docID:  "foo",
			status: StatusBadRequest,
			err:    "db error",
		},
		{
			name:     "Interface",
			db:       &DB{driverDB: &putGrabber{newRev: "1-xxx"}},
			docID:    "foo",
			input:    map[string]string{"foo": "bar"},
			expected: map[string]string{"foo": "bar"},
			newRev:   "1-xxx",
		},
		{
			name:   "InvalidJSON",
			docID:  "foo",
			input:  []byte("Something bogus"),
			status: StatusBadRequest,
			err:    "invalid character 'S' looking for beginning of value",
		},
		{
			name:     "Bytes",
			db:       &DB{driverDB: &putGrabber{newRev: "1-xxx"}},
			docID:    "foo",
			input:    []byte(`{"foo":"bar"}`),
			expected: map[string]interface{}{"foo": "bar"},
			newRev:   "1-xxx",
		},
		{
			name:     "RawMessage",
			db:       &DB{driverDB: &putGrabber{newRev: "1-xxx"}},
			docID:    "foo",
			input:    json.RawMessage(`{"foo":"bar"}`),
			expected: map[string]interface{}{"foo": "bar"},
			newRev:   "1-xxx",
		},
		{
			name:     "Reader",
			db:       &DB{driverDB: &putGrabber{newRev: "1-xxx"}},
			docID:    "foo",
			input:    strings.NewReader(`{"foo":"bar"}`),
			expected: map[string]interface{}{"foo": "bar"},
			newRev:   "1-xxx",
		},
		{
			name:   "ErrorReader",
			docID:  "foo",
			input:  &errorReader{},
			status: StatusUnknownError,
			err:    "errorReader",
		},
		{
			name:     "valid",
			db:       &DB{driverDB: &putGrabber{newRev: "1-xxx"}},
			docID:    "foo",
			input:    map[string]string{"foo": "bar"},
			options:  Options{"foo": "bar"},
			expected: map[string]string{"foo": "bar"},
			newRev:   "1-xxx",
		},
	}
	for _, test := range tests {
		func(test putTest) {
			t.Run(test.name, func(t *testing.T) {
				newRev, err := test.db.Put(context.Background(), test.docID, test.input, test.options)
				testy.StatusError(t, test.err, test.status, err)
				if newRev != test.newRev {
					t.Errorf("Unexpected new rev: %s", newRev)
				}
				if rec, ok := test.db.driverDB.(*putGrabber); ok {
					if d := diff.Interface(test.expected, rec.lastPut); d != nil {
						t.Error(d)
					}
					if d := diff.Interface(map[string]interface{}(test.options), rec.lastOpts); d != nil {
						t.Error(d)
					}
				}
			})
		}(test)
	}
}

func TestExtractDocID(t *testing.T) {
	type ediTest struct {
		name     string
		i        interface{}
		id       string
		expected bool
	}
	tests := []ediTest{
		{
			name: "nil",
		},
		{
			name: "string/interface map, no id",
			i: map[string]interface{}{
				"value": "foo",
			},
		},
		{
			name: "string/interface map, with id",
			i: map[string]interface{}{
				"_id": "foo",
			},
			id:       "foo",
			expected: true,
		},
		{
			name: "string/string map, with id",
			i: map[string]string{
				"_id": "foo",
			},
			id:       "foo",
			expected: true,
		},
		{
			name: "invalid JSON",
			i:    make(chan int),
		},
		{
			name: "valid JSON",
			i: struct {
				ID string `json:"_id"`
			}{ID: "oink"},
			id:       "oink",
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			id, ok := extractDocID(test.i)
			if ok != test.expected || test.id != id {
				t.Errorf("Expected %t/%s, got %t/%s", test.expected, test.id, ok, id)
			}
		})
	}
}

type createDocGrabber struct {
	*dummyDB
	lastDoc  interface{}
	lastOpts map[string]interface{}

	id, rev string
	err     error
}

func (db *createDocGrabber) CreateDoc(_ context.Context, doc interface{}, opts map[string]interface{}) (string, string, error) {
	db.lastDoc = doc
	db.lastOpts = opts
	return db.id, db.rev, db.err
}

func TestCreateDoc(t *testing.T) {
	tests := []struct {
		name       string
		db         *DB
		doc        interface{}
		options    Options
		docID, rev string
		status     int
		err        string
	}{
		{
			name:   "error",
			db:     &DB{driverDB: &createDocGrabber{err: errors.Status(StatusBadRequest, "create error")}},
			status: StatusBadRequest,
			err:    "create error",
		},
		{
			name:    "success",
			db:      &DB{driverDB: &createDocGrabber{id: "foo", rev: "1-xxx"}},
			doc:     map[string]string{"type": "test"},
			options: Options{"foo": "bar"},
			docID:   "foo",
			rev:     "1-xxx",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			docID, rev, err := test.db.CreateDoc(context.Background(), test.doc, test.options)
			testy.StatusError(t, test.err, test.status, err)
			if docID != test.docID || test.rev != test.rev {
				t.Errorf("Unexpected result: %s / %s", docID, rev)
			}
			if grabber, ok := test.db.driverDB.(*createDocGrabber); ok {
				if d := diff.Interface(test.doc, grabber.lastDoc); d != nil {
					t.Error(d)
				}
				if d := diff.Interface(map[string]interface{}(test.options), grabber.lastOpts); d != nil {
					t.Error(d)
				}
			}
		})
	}
}

type deleteRecorder struct {
	*dummyDB
	lastID, lastRev string
	lastOpts        map[string]interface{}

	newRev string
	err    error
}

func (db *deleteRecorder) Delete(_ context.Context, docID, rev string, opts map[string]interface{}) (string, error) {
	db.lastID, db.lastRev = docID, rev
	return db.newRev, db.err
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name       string
		db         *DB
		docID, rev string
		options    Options
		newRev     string
		status     int
		err        string
	}{
		{
			name:   "error",
			db:     &DB{driverDB: &deleteRecorder{err: errors.Status(StatusBadRequest, "delete error")}},
			status: StatusBadRequest,
			err:    "delete error",
		},
		{
			name:   "success",
			db:     &DB{driverDB: &deleteRecorder{newRev: "2-xxx"}},
			docID:  "foo",
			rev:    "1-xxx",
			newRev: "2-xxx",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newRev, err := test.db.Delete(context.Background(), test.docID, test.rev, test.options)
			testy.StatusError(t, test.err, test.status, err)
			if newRev != test.newRev {
				t.Errorf("Unexpected newRev: %s", newRev)
			}
			if rec, ok := test.db.driverDB.(*deleteRecorder); ok {
				if rec.lastID != test.docID {
					t.Errorf("Unexpected docID: %s", rec.lastID)
				}
				if rec.lastRev != test.rev {
					t.Errorf("Unexpected rev: %s", rec.lastRev)
				}
				if d := diff.Interface(map[string]interface{}(test.options), rec.lastOpts); d != nil {
					t.Error(d)
				}
			}
		})
	}
}

type putAttRecorder struct {
	*dummyDB
	lastID, lastRev, lastFilename, lastType string
	lastBody                                []byte
	lastOpts                                map[string]interface{}

	newRev string
	err    error
}

var _ driver.DB = &putAttRecorder{}

func (db *putAttRecorder) PutAttachment(_ context.Context, docID, rev, filename, contentType string, body io.Reader, opts map[string]interface{}) (string, error) {
	db.lastID, db.lastRev, db.lastFilename, db.lastType = docID, rev, filename, contentType
	var err error
	db.lastBody, err = ioutil.ReadAll(body)
	if err != nil {
		panic(err)
	}
	db.lastOpts = opts

	return db.newRev, db.err
}

func TestPutAttachment(t *testing.T) {
	tests := []struct {
		name       string
		db         *DB
		docID, rev string
		att        *Attachment
		options    Options
		newRev     string
		status     int
		err        string

		body string
	}{
		{
			name:  "db error",
			docID: "foo",
			db:    &DB{driverDB: &putAttRecorder{err: errors.Status(StatusBadRequest, "db error")}},
			att: &Attachment{
				Filename:   "foo.txt",
				ReadCloser: ioutil.NopCloser(strings.NewReader("")),
			},
			status: StatusBadRequest,
			err:    "db error",
		},
		{
			name:   "no doc id",
			status: StatusBadRequest,
			err:    "kivik: docID required",
		},
		{
			name:   "no filename",
			docID:  "foo",
			att:    &Attachment{},
			status: StatusBadRequest,
			err:    "kivik: filename required",
		},
		{
			name:  "success",
			docID: "foo",
			db:    &DB{driverDB: &putAttRecorder{newRev: "2-xxx"}},
			att: &Attachment{
				Filename:    "foo.txt",
				ContentType: "text/plain",
				ReadCloser:  ioutil.NopCloser(strings.NewReader("Test file")),
			},
			newRev: "2-xxx",
			body:   "Test file",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			newRev, err := test.db.PutAttachment(context.Background(), test.docID, test.rev, test.att, test.options)
			testy.StatusError(t, test.err, test.status, err)
			if newRev != test.newRev {
				t.Errorf("Unexpected newRev: %s", newRev)
			}
			if rec, ok := test.db.driverDB.(*putAttRecorder); ok {
				if rec.lastID != test.docID {
					t.Errorf("Unexpected docID: %s", rec.lastID)
				}
				if rec.lastFilename != test.att.Filename {
					t.Errorf("Unexpected filename: %s", rec.lastFilename)
				}
				if rec.lastType != test.att.ContentType {
					t.Errorf("Unexpected content type: %s", rec.lastType)
				}
				if d := diff.Text(test.body, string(rec.lastBody)); d != nil {
					t.Errorf("Unexpected body:\n%s\n", d)
				}
			}
		})
	}
}
