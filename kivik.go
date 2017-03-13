package kivik

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/flimzy/kivik/driver"
	"github.com/flimzy/kivik/errors"
)

// Client is a client connection handle to a CouchDB-like server.
type Client struct {
	// AutoFlush turns on the AutoFlush flag for new database connections.
	AutoFlush bool

	dsn          string
	driverClient driver.Client
}

// Options is a collection of options. The keys and values are backend specific.
type Options map[string]interface{}

// New calls NewContext with a background context.
func New(driverName, dataSourceName string) (*Client, error) {
	return NewContext(context.Background(), driverName, dataSourceName)
}

// NewContext creates a new client object specified by its database driver name
// and a driver-specific data source name.
func NewContext(ctx context.Context, driverName, dataSourceName string) (*Client, error) {
	driversMu.RLock()
	driveri, ok := drivers[driverName]
	driversMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("kivik: unknown driver %q (forgotten import?)", driverName)
	}
	client, err := driveri.NewClientContext(ctx, dataSourceName)
	if err != nil {
		return nil, err
	}
	return &Client{
		dsn:          dataSourceName,
		driverClient: client,
	}, nil
}

// DSN returns the data source name used to connect this client.
func (c *Client) DSN() string {
	return c.dsn
}

// ServerInfo calls ServerInfoContext with a background context.
func (c *Client) ServerInfo() (driver.ServerInfo, error) {
	return c.ServerInfoContext(context.Background())
}

// ServerInfoContext returns version and vendor info about the backend.
func (c *Client) ServerInfoContext(ctx context.Context) (driver.ServerInfo, error) {
	return c.driverClient.ServerInfoContext(ctx)
}

// DB calls DBContext with a background context.
func (c *Client) DB(dbName string) (*DB, error) {
	return c.DBContext(context.Background(), dbName)
}

// DBContext returns a handle to the requested database.
func (c *Client) DBContext(ctx context.Context, dbName string) (*DB, error) {
	db, err := c.driverClient.DBContext(ctx, dbName)
	return &DB{
		AutoFlush: c.AutoFlush,
		driverDB:  db,
	}, err
}

// AllDBs calls AllDBsContext with a background context.
func (c *Client) AllDBs() ([]string, error) {
	return c.AllDBsContext(context.Background())
}

// AllDBsContext returns a list of all databases.
func (c *Client) AllDBsContext(ctx context.Context) ([]string, error) {
	return c.driverClient.AllDBsContext(ctx)
}

// UUIDs calls UUIDsContext with a background context.
func (c *Client) UUIDs(count int) ([]string, error) {
	return c.UUIDsContext(context.Background(), count)
}

// UUIDsContext returns one or more UUIDs as generated by the CouchDB server.
// This method may not be implemented by all backends, in which case an error
// will be returned. Generally, there are better ways to generate UUIDs.
func (c *Client) UUIDsContext(ctx context.Context, count int) ([]string, error) {
	if count < 0 {
		return nil, errors.Status(http.StatusBadRequest, "count must be a positive integer")
	}
	if uuider, ok := c.driverClient.(driver.UUIDer); ok {
		return uuider.UUIDsContext(ctx, count)
	}
	return nil, ErrNotImplemented
}

// Membership calls MembershipContext with a background context.
func (c *Client) Membership() (allNodes []string, clusterNodes []string, err error) {
	return c.MembershipContext(context.Background())
}

// MembershipContext returns the list of nodes that are part of the cluster as
// clusterNodes, and all known nodes, including cluster nodes, as allNodes.
// Not all servers or clients will support this method.
func (c *Client) MembershipContext(ctx context.Context) (allNodes []string, clusterNodes []string, err error) {
	if cluster, ok := c.driverClient.(driver.Cluster); ok {
		return cluster.MembershipContext(ctx)
	}
	return nil, nil, ErrNotImplemented
}

// Log reads the server log, if supported by the client driver. This method will
// read up to length bytes of logs from the server, ending at offset bytes from
// the end. The caller must close the ReadCloser.
func (c *Client) Log(length, offset int64) (io.ReadCloser, error) {
	return c.LogContext(context.Background(), length, offset)
}

// LogContext reads the server log, if supported by the client driver. This
// method will read up to length bytes of logs from the server, ending at offset
// bytes from the end. The provided context must be non-nil. The caller must
// close the ReadCloser.
func (c *Client) LogContext(ctx context.Context, length, offset int64) (io.ReadCloser, error) {
	if logger, ok := c.driverClient.(driver.LogReader); ok {
		return logger.LogContext(ctx, length, offset)
	}
	return nil, ErrNotImplemented
}

// DBExists calls DBExistsContext with a background context.
func (c *Client) DBExists(dbName string) (bool, error) {
	return c.DBExistsContext(context.Background(), dbName)
}

// DBExistsContext returns true if the specified database exists.
func (c *Client) DBExistsContext(ctx context.Context, dbName string) (bool, error) {
	return c.driverClient.DBExistsContext(ctx, dbName)
}

// Copied verbatim from http://docs.couchdb.org/en/2.0.0/api/database/common.html#head--db
var validDBName = regexp.MustCompile("^[a-z][a-z0-9_$()+/-]*$")

// CreateDB calls CreateDBContext with a background context.
func (c *Client) CreateDB(dbName string) error {
	return c.CreateDBContext(context.Background(), dbName)
}

// CreateDBContext creates a DB of the requested name.
func (c *Client) CreateDBContext(ctx context.Context, dbName string) error {
	if !validDBName.MatchString(dbName) {
		return errors.Status(StatusBadRequest, "invalid database name")
	}
	return c.driverClient.CreateDBContext(ctx, dbName)
}

// DestroyDB calls DestroyDBContext with a background context.
func (c *Client) DestroyDB(dbName string) error {
	return c.DestroyDBContext(context.Background(), dbName)
}

// DestroyDBContext deletes the requested DB.
func (c *Client) DestroyDBContext(ctx context.Context, dbName string) error {
	return c.driverClient.DestroyDBContext(ctx, dbName)
}

// Authenticate calls AuthenticateContext with a background context.
func (c *Client) Authenticate(a interface{}) error {
	return c.AuthenticateContext(context.Background(), a)
}

// AuthenticateContext authenticates the client with the passed authenticator, which
// is driver-specific. If the driver does not understand the authenticator, an
// error will be returned.
func (c *Client) AuthenticateContext(ctx context.Context, a interface{}) error {
	if auth, ok := c.driverClient.(driver.Authenticator); ok {
		return auth.AuthenticateContext(ctx, a)
	}
	return ErrNotImplemented
}
