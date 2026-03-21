package monitor

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	// Pure-Go MySQL driver
	_ "github.com/go-sql-driver/mysql"
	// Pure-Go PostgreSQL driver
	_ "github.com/lib/pq"
	// Microsoft SQL Server driver
	_ "github.com/microsoft/go-mssqldb"
	// Pure-Go MongoDB driver
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/xdung24/conductor/internal/models"
)

// NewDBConn opens and configures a persistent *sql.DB connection pool for
// MySQL, PostgreSQL, and MSSQL monitors. Returns nil for other monitor types.
// The caller is responsible for closing the returned *sql.DB when done.
func NewDBConn(m *models.Monitor) *sql.DB {
	var driver string
	switch m.Type {
	case models.MonitorTypeMySQL:
		driver = "mysql"
	case models.MonitorTypePostgres:
		driver = "postgres"
	case models.MonitorTypeMSSQL:
		driver = "sqlserver"
	default:
		return nil
	}
	db, err := sql.Open(driver, m.URL)
	if err != nil {
		return nil
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)
	return db
}

// ---------------------------------------------------------------------------
// MySQL / MariaDB checker
// ---------------------------------------------------------------------------

// MySQLChecker checks a MySQL or MariaDB server by opening a connection and
// running either a configured query or a lightweight "SELECT 1" ping.
//
// The monitor URL must be a valid MySQL DSN:
//
//	user:password@tcp(host:port)/dbname
//	user:password@tcp(host:port)/dbname?tls=skip-verify
type MySQLChecker struct {
	conn *sql.DB // optional cached connection pool; if nil a new connection is opened per check
}

// Check opens a MySQL connection, pings the server, and optionally executes
// the configured query. Returns UP with latency on success.
func (c *MySQLChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	db := c.conn
	if db == nil {
		var err error
		db, err = sql.Open("mysql", m.URL)
		if err != nil {
			return Result{Status: 0, Message: fmt.Sprintf("open: %v", err)}
		}
		defer db.Close() //nolint:errcheck
		db.SetMaxOpenConns(1)
		db.SetConnMaxLifetime(time.Duration(m.TimeoutSeconds) * time.Second)
	}

	if err := db.PingContext(ctx); err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("ping: %v", err)}
	}

	query := m.DBQuery
	if query == "" {
		query = "SELECT 1"
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("query: %v", err)}
	}
	rows.Close() //nolint:errcheck

	return Result{
		Status:    1,
		LatencyMs: int(time.Since(start).Milliseconds()),
		Message:   "MySQL OK",
	}
}

// ---------------------------------------------------------------------------
// PostgreSQL checker
// ---------------------------------------------------------------------------

// PostgresChecker checks a PostgreSQL server by opening a connection and
// running either a configured query or a lightweight "SELECT 1" ping.
//
// The monitor URL must be a valid libpq connection string or URL:
//
//	postgres://user:password@host:port/dbname?sslmode=disable
//	host=localhost port=5432 user=postgres password=secret dbname=mydb sslmode=disable
type PostgresChecker struct {
	conn *sql.DB // optional cached connection pool; if nil a new connection is opened per check
}

// Check opens a PostgreSQL connection, pings the server, and optionally
// executes the configured query. Returns UP with latency on success.
func (c *PostgresChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	db := c.conn
	if db == nil {
		var err error
		db, err = sql.Open("postgres", m.URL)
		if err != nil {
			return Result{Status: 0, Message: fmt.Sprintf("open: %v", err)}
		}
		defer db.Close() //nolint:errcheck
		db.SetMaxOpenConns(1)
		db.SetConnMaxLifetime(time.Duration(m.TimeoutSeconds) * time.Second)
	}

	if err := db.PingContext(ctx); err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("ping: %v", err)}
	}

	query := m.DBQuery
	if query == "" {
		query = "SELECT 1"
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("query: %v", err)}
	}
	rows.Close() //nolint:errcheck

	return Result{
		Status:    1,
		LatencyMs: int(time.Since(start).Milliseconds()),
		Message:   "PostgreSQL OK",
	}
}

// ---------------------------------------------------------------------------
// Redis checker (raw RESP protocol — no external driver needed)
// ---------------------------------------------------------------------------

// RedisChecker checks a Redis server using the raw RESP protocol.
// It sends AUTH (if a password is provided in the connection string) and then
// PING, expecting a +PONG reply.
//
// The monitor URL format:
//
//	host:port                        (no authentication)
//	:password@host:port              (password-only / ACL-less)
//	username:password@host:port      (ACL user + password, Redis 6+)
type RedisChecker struct{}

// Check connects to Redis via raw TCP and sends PING, returning UP on +PONG.
func (c *RedisChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	addr, user, pass := parseRedisURL(m.URL)

	d := dialerFor(m)
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("connect: %v", err)}
	}
	defer conn.Close() //nolint:errcheck

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline) //nolint:errcheck
	}

	r := bufio.NewReader(conn)

	// AUTH (Redis 6+ supports AUTH username password; older only AUTH password).
	if pass != "" {
		var authCmd string
		if user != "" {
			authCmd = fmt.Sprintf("*3\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
				len(user), user, len(pass), pass)
		} else {
			authCmd = fmt.Sprintf("*2\r\n$4\r\nAUTH\r\n$%d\r\n%s\r\n", len(pass), pass)
		}
		if _, err := fmt.Fprint(conn, authCmd); err != nil {
			return Result{Status: 0, Message: fmt.Sprintf("send AUTH: %v", err)}
		}
		resp, err := r.ReadString('\n')
		if err != nil {
			return Result{Status: 0, Message: fmt.Sprintf("read AUTH response: %v", err)}
		}
		resp = strings.TrimSpace(resp)
		if !strings.HasPrefix(resp, "+OK") {
			return Result{Status: 0, Message: fmt.Sprintf("AUTH failed: %s", resp)}
		}
	}

	// PING
	if _, err := fmt.Fprint(conn, "*1\r\n$4\r\nPING\r\n"); err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("send PING: %v", err)}
	}
	resp, err := r.ReadString('\n')
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("read PING response: %v", err)}
	}
	resp = strings.TrimSpace(resp)
	if resp != "+PONG" && resp != "$4" {
		return Result{Status: 0, Message: fmt.Sprintf("unexpected PING response: %s", resp)}
	}

	return Result{
		Status:    1,
		LatencyMs: int(time.Since(start).Milliseconds()),
		Message:   "Redis OK",
	}
}

// parseRedisURL parses a Redis address string into (addr, user, password).
// Supported formats:
//
//	host:port
//	:password@host:port
//	username:password@host:port
func parseRedisURL(raw string) (addr, user, pass string) {
	if idx := strings.LastIndex(raw, "@"); idx >= 0 {
		credentials := raw[:idx]
		addr = raw[idx+1:]
		if colonIdx := strings.Index(credentials, ":"); colonIdx >= 0 {
			user = credentials[:colonIdx]
			pass = credentials[colonIdx+1:]
		} else {
			pass = credentials
		}
	} else {
		addr = raw
	}
	// Default Redis port.
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "6379")
	}
	return addr, user, pass
}

// ---------------------------------------------------------------------------
// MongoDB checker
// ---------------------------------------------------------------------------

// MongoDBChecker checks a MongoDB server by connecting and sending a ping
// command. Uses the official pure-Go mongo-driver.
//
// The monitor URL must be a valid MongoDB connection string:
//
//	mongodb://user:password@host:port/dbname
//	mongodb+srv://user:password@cluster.example.com/dbname
type MongoDBChecker struct{}

// Check connects to MongoDB and issues a ping command.
func (c *MongoDBChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	timeout := time.Duration(m.TimeoutSeconds) * time.Second
	opts := options.Client().
		ApplyURI(m.URL).
		SetConnectTimeout(timeout).
		SetServerSelectionTimeout(timeout)

	client, err := mongo.Connect(opts)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("connect: %v", err)}
	}
	defer client.Disconnect(context.Background()) //nolint:errcheck

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("ping: %v", err)}
	}

	// Optionally run a custom command stored in DBQuery (e.g. {"listDatabases":1}).
	if m.DBQuery != "" {
		var cmd bson.D
		if err := bson.UnmarshalExtJSON([]byte(m.DBQuery), true, &cmd); err != nil {
			return Result{Status: 0, Message: fmt.Sprintf("invalid command JSON: %v", err)}
		}
		var result bson.M
		if err := client.Database("admin").RunCommand(ctx, cmd).Decode(&result); err != nil {
			return Result{Status: 0, Message: fmt.Sprintf("command: %v", err)}
		}
	}

	return Result{
		Status:    1,
		LatencyMs: int(time.Since(start).Milliseconds()),
		Message:   "MongoDB OK",
	}
}

// ---------------------------------------------------------------------------
// Microsoft SQL Server checker
// ---------------------------------------------------------------------------

// MSSQLChecker checks a Microsoft SQL Server by opening a connection and
// running either a configured query or a lightweight "SELECT 1" ping.
//
// The monitor URL must be a valid SQL Server DSN:
//
//	sqlserver://user:password@host:port?database=dbname
//	server=host;user id=user;password=pass;database=dbname
type MSSQLChecker struct {
	conn *sql.DB // optional cached connection pool; if nil a new connection is opened per check
}

// Check opens a SQL Server connection, pings the server, and optionally
// executes the configured query. Returns UP with latency on success.
func (c *MSSQLChecker) Check(ctx context.Context, m *models.Monitor) Result {
	start := time.Now()

	db := c.conn
	if db == nil {
		var err error
		db, err = sql.Open("sqlserver", m.URL)
		if err != nil {
			return Result{Status: 0, Message: fmt.Sprintf("open: %v", err)}
		}
		defer db.Close() //nolint:errcheck
		db.SetMaxOpenConns(1)
		db.SetConnMaxLifetime(time.Duration(m.TimeoutSeconds) * time.Second)
	}

	if err := db.PingContext(ctx); err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("ping: %v", err)}
	}

	query := m.DBQuery
	if query == "" {
		query = "SELECT 1"
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return Result{Status: 0, Message: fmt.Sprintf("query: %v", err)}
	}
	rows.Close() //nolint:errcheck

	return Result{
		Status:    1,
		LatencyMs: int(time.Since(start).Milliseconds()),
		Message:   "MSSQL OK",
	}
}
