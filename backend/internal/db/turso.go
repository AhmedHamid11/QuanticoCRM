package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// DBConn is the interface that both *sql.DB and *TursoDB satisfy
// This allows repos to work with either a regular DB or a reconnecting TursoDB
type DBConn interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// TursoDB wraps database connections to Turso with automatic reconnection
// Uses fresh connections per-operation to avoid libsql driver connection issues
type TursoDB struct {
	connStr   string
	mu        sync.Mutex
	currentDB *sql.DB
}

// NewTursoDB creates a new TursoDB connection
func NewTursoDB(url, authToken string) (*TursoDB, error) {
	connStr := url
	if authToken != "" {
		connStr = url + "?authToken=" + authToken
	}

	t := &TursoDB{
		connStr: connStr,
	}

	log.Printf("[TURSO-DB] Creating initial connection to: %s", maskConnectionString(url))

	// Create initial connection to verify credentials work
	db, err := t.newConnection()
	if err != nil {
		log.Printf("[TURSO-DB] Initial connection FAILED: %v", err)
		return nil, err
	}
	t.currentDB = db

	log.Println("[TURSO-DB] Initial connection ESTABLISHED successfully")
	return t, nil
}

// maskConnectionString masks sensitive parts of connection string for logging
func maskConnectionString(url string) string {
	if len(url) > 50 {
		return url[:30] + "..." + url[len(url)-15:]
	}
	return url
}

// newConnection creates a fresh database connection
func (t *TursoDB) newConnection() (*sql.DB, error) {
	log.Println("[TURSO-DB] Opening new database connection...")
	db, err := sql.Open("libsql", t.connStr)
	if err != nil {
		log.Printf("[TURSO-DB] sql.Open FAILED: %v", err)
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}

	// libsql uses HTTP - single connection mode
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Second) // Short lifetime to force fresh connections
	db.SetConnMaxIdleTime(10 * time.Second)

	// Verify connection works
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("[TURSO-DB] Ping FAILED: %v", err)
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("[TURSO-DB] Connection opened and verified successfully")
	return db, nil
}

// getConnection returns a working connection, creating a new one if needed
// IMPORTANT: This method does NOT close old connections synchronously to avoid race conditions
// where one goroutine closes a connection another goroutine is still using.
// Old connections will be garbage collected and their resources released via ConnMaxIdleTime.
func (t *TursoDB) getConnection(ctx context.Context) (*sql.DB, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Try current connection first
	if t.currentDB != nil {
		if err := t.currentDB.PingContext(ctx); err == nil {
			log.Println("[TURSO-DB] Reusing existing healthy connection")
			return t.currentDB, nil
		}
		// Connection is dead - DON'T close it here as other goroutines might still be using it
		// Just create a new one and let the old one be garbage collected
		log.Println("[TURSO-DB] Current connection STALE (ping failed), creating replacement...")
	} else {
		log.Println("[TURSO-DB] No current connection, creating new one...")
	}

	// Create new connection
	db, err := t.newConnection()
	if err != nil {
		log.Printf("[TURSO-DB] Failed to create replacement connection: %v", err)
		return nil, err
	}
	t.currentDB = db
	log.Println("[TURSO-DB] Replacement connection ESTABLISHED")
	return db, nil
}

// GetDB returns the underlying sql.DB (for compatibility)
func (t *TursoDB) GetDB() *sql.DB {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentDB
}

// DB returns a valid database connection, reconnecting if necessary
// This is the preferred method to get a connection as it ensures validity
func (t *TursoDB) DB() (*sql.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return t.getConnection(ctx)
}

// Close closes all database connections
func (t *TursoDB) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.currentDB != nil {
		err := t.currentDB.Close()
		t.currentDB = nil
		return err
	}
	return nil
}

// isConnectionError checks if the error indicates a closed/stale connection
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "stream is closed") ||
		strings.Contains(errStr, "database is closed") ||
		strings.Contains(errStr, "bad connection") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset")
}

// executeWithRetry executes a database operation with retry logic
// On connection errors, it marks the connection as stale without closing it
// to avoid race conditions with other goroutines using the same connection
func (t *TursoDB) executeWithRetry(ctx context.Context, operation func(*sql.DB) error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		// Get a fresh connection for each attempt
		db, err := t.getConnection(ctx)
		if err != nil {
			lastErr = err
			log.Printf("TursoDB: Failed to get connection (attempt %d): %v", attempt+1, err)
			time.Sleep(time.Duration(attempt*50) * time.Millisecond)
			continue
		}

		err = operation(db)
		if err == nil {
			return nil
		}

		lastErr = err
		if isConnectionError(err) {
			log.Printf("TursoDB: Connection error (attempt %d): %v", attempt+1, err)
			// Mark connection as stale - DON'T close it as other goroutines might be using it
			// Just nil it out so next getConnection creates a new one
			t.mu.Lock()
			t.currentDB = nil
			t.mu.Unlock()
			time.Sleep(time.Duration(attempt*50) * time.Millisecond)
			continue
		}
		// Non-connection error, return immediately
		return err
	}
	return fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

// QueryContext executes a query with automatic reconnection and retry
func (t *TursoDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var result *sql.Rows
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.QueryContext(ctx, query, args...)
		return err
	})
	return result, err
}

// QueryRowContext executes a query that returns a single row with retry logic
// Note: This implementation uses QueryContext internally to enable retry on connection errors
func (t *TursoDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	var result *sql.Row
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		db, err := t.getConnection(ctx)
		if err != nil {
			lastErr = err
			log.Printf("TursoDB: QueryRowContext failed to get connection (attempt %d): %v", attempt+1, err)
			time.Sleep(time.Duration(attempt*50) * time.Millisecond)
			continue
		}

		result = db.QueryRowContext(ctx, query, args...)
		// We can't know if there's an error until Scan() is called
		// But we can at least ensure we have a valid connection
		return result
	}

	log.Printf("TursoDB: QueryRowContext failed after 3 attempts: %v", lastErr)
	// Last resort: try currentDB even though getConnection failed.
	// *sql.DB.QueryRowContext never returns nil - it defers errors to Scan().
	// This avoids nil pointer panics in callers.
	t.mu.Lock()
	fallbackDB := t.currentDB
	t.mu.Unlock()
	if fallbackDB != nil {
		return fallbackDB.QueryRowContext(ctx, query, args...)
	}
	// Truly no connection available - this should be extremely rare
	log.Printf("TursoDB: CRITICAL - no connection available at all, returning nil Row")
	return nil
}

// ExecContext executes a statement with automatic reconnection and retry
func (t *TursoDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}

// Ping checks if the database connection is alive
func (t *TursoDB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := t.getConnection(ctx)
	return err
}

// PrepareContext prepares a statement with automatic reconnection
func (t *TursoDB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	var result *sql.Stmt
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.PrepareContext(ctx, query)
		return err
	})
	return result, err
}

// BeginTx starts a transaction with automatic reconnection
func (t *TursoDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	var result *sql.Tx
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.BeginTx(ctx, opts)
		return err
	})
	return result, err
}

// Ensure TursoDB implements DBConn interface
var _ DBConn = (*TursoDB)(nil)

// ============================================================================
// TenantDB - Wrapper for tenant database connections with retry logic
// ============================================================================

// TenantDB wraps a tenant database connection with automatic retry logic
// This provides the same retry behavior as TursoDB for tenant-specific databases
type TenantDB struct {
	db            *sql.DB
	orgID         string
	mu            sync.Mutex
	connStr       string    // For reconnection
	token         string    // Auth token for reconnection
	lastReconnect time.Time // Tracks last reconnection to avoid rapid reconnects
}

// NewTenantDB creates a new TenantDB wrapper around an existing connection
func NewTenantDB(db *sql.DB, orgID, connStr, token string) *TenantDB {
	return &TenantDB{
		db:      db,
		orgID:   orgID,
		connStr: connStr,
		token:   token,
	}
}

// GetDB returns the underlying sql.DB (for compatibility with code expecting raw *sql.DB)
func (t *TenantDB) GetDB() *sql.DB {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.db
}

// reconnect attempts to create a new database connection
// IMPORTANT: This method does NOT close old connections to avoid race conditions
// where one goroutine closes a connection another goroutine is still using.
// Old connections will be garbage collected when no longer referenced.
func (t *TenantDB) reconnect() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Avoid rapid reconnects - if we reconnected within the last 500ms, skip
	// This prevents multiple goroutines from all triggering reconnects simultaneously
	if time.Since(t.lastReconnect) < 500*time.Millisecond {
		log.Printf("[TENANT-DB] Org=%s Skipping reconnect (last reconnect was %v ago)", t.orgID, time.Since(t.lastReconnect))
		return nil // Assume another goroutine already reconnected
	}

	// Check if current connection is actually working - another goroutine may have already fixed it
	if t.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := t.db.PingContext(ctx); err == nil {
			cancel()
			log.Printf("[TENANT-DB] Org=%s Current connection is healthy, skipping reconnect", t.orgID)
			return nil
		}
		cancel()
	}

	if t.connStr == "" {
		log.Printf("[TENANT-DB] Org=%s Cannot reconnect: no connection string stored", t.orgID)
		return fmt.Errorf("cannot reconnect: no connection string")
	}

	log.Printf("[TENANT-DB] Org=%s Attempting reconnection...", t.orgID)

	connStr := t.connStr
	if t.token != "" {
		connStr = t.connStr + "?authToken=" + t.token
	}

	db, err := sql.Open("libsql", connStr)
	if err != nil {
		log.Printf("[TENANT-DB] Org=%s sql.Open FAILED: %v", t.orgID, err)
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("[TENANT-DB] Org=%s Reconnection ping FAILED: %v", t.orgID, err)
		db.Close()
		return fmt.Errorf("failed to ping on reconnect: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Replace old connection - DO NOT close the old one!
	// Other goroutines may still be using it. Let it be garbage collected.
	// The old connection's resources will be released when it's no longer referenced.
	t.db = db
	t.lastReconnect = time.Now()

	log.Printf("[TENANT-DB] Org=%s Reconnection SUCCESSFUL", t.orgID)
	return nil
}

// executeWithRetry executes a database operation with retry logic
func (t *TenantDB) executeWithRetry(ctx context.Context, operation func(*sql.DB) error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		t.mu.Lock()
		db := t.db
		t.mu.Unlock()

		if db == nil {
			lastErr = fmt.Errorf("database connection is nil")
			log.Printf("[TENANT-DB] Org=%s Connection nil (attempt %d/3), reconnecting...", t.orgID, attempt+1)
			if err := t.reconnect(); err != nil {
				lastErr = err
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
				continue
			}
			t.mu.Lock()
			db = t.db
			t.mu.Unlock()
		}

		err := operation(db)
		if err == nil {
			return nil
		}

		lastErr = err
		if isConnectionError(err) {
			log.Printf("[TENANT-DB] Org=%s Connection error (attempt %d/3): %v", t.orgID, attempt+1, err)
			if err := t.reconnect(); err != nil {
				log.Printf("[TENANT-DB] Org=%s Reconnect failed: %v", t.orgID, err)
			}
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		// Non-connection error, return immediately
		return err
	}
	return fmt.Errorf("operation failed after 3 attempts: %w", lastErr)
}

// QueryContext executes a query with automatic retry
func (t *TenantDB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	var result *sql.Rows
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.QueryContext(ctx, query, args...)
		return err
	})
	return result, err
}

// QueryRowContext executes a single-row query
// Note: Errors from QueryRow are deferred until Scan(), so retry is limited
// For retry-enabled single-row queries, use QueryRowScanContext instead
func (t *TenantDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	t.mu.Lock()
	db := t.db
	t.mu.Unlock()

	// Check if connection is nil or closed, and reconnect if needed
	needsReconnect := db == nil
	if !needsReconnect && db != nil {
		// Quick ping check to detect closed connections
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := db.PingContext(pingCtx); err != nil {
			needsReconnect = true
			log.Printf("[TENANT-DB] Org=%s QueryRowContext ping failed: %v, will reconnect", t.orgID, err)
		}
		cancel()
	}

	if needsReconnect {
		if err := t.reconnect(); err != nil {
			log.Printf("[TENANT-DB] Org=%s QueryRowContext reconnect failed: %v", t.orgID, err)
		}
		t.mu.Lock()
		db = t.db
		t.mu.Unlock()
	}
	if db == nil {
		log.Printf("[TENANT-DB] Org=%s CRITICAL - QueryRowContext db is nil, no connection available", t.orgID)
		return nil
	}
	return db.QueryRowContext(ctx, query, args...)
}

// QueryRowScanContext executes a single-row query and scans into dest with automatic retry
// This is the preferred method for single-row queries as it enables proper retry on connection errors
func (t *TenantDB) QueryRowScanContext(ctx context.Context, dest []interface{}, query string, args ...interface{}) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		t.mu.Lock()
		db := t.db
		t.mu.Unlock()

		if db == nil {
			log.Printf("[TENANT-DB] Org=%s QueryRowScan connection nil (attempt %d/3), reconnecting...", t.orgID, attempt+1)
			if err := t.reconnect(); err != nil {
				lastErr = err
				time.Sleep(time.Duration(attempt*100) * time.Millisecond)
				continue
			}
			t.mu.Lock()
			db = t.db
			t.mu.Unlock()
		}

		err := db.QueryRowContext(ctx, query, args...).Scan(dest...)
		if err == nil {
			return nil
		}

		lastErr = err
		if isConnectionError(err) {
			log.Printf("[TENANT-DB] Org=%s QueryRowScan error (attempt %d/3): %v", t.orgID, attempt+1, err)
			if reconnErr := t.reconnect(); reconnErr != nil {
				log.Printf("[TENANT-DB] Org=%s Reconnect failed: %v", t.orgID, reconnErr)
			}
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		// Non-connection error (including sql.ErrNoRows), return immediately
		return err
	}
	return fmt.Errorf("query row scan failed after 3 attempts: %w", lastErr)
}

// ExecContext executes a statement with automatic retry
func (t *TenantDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	var result sql.Result
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.ExecContext(ctx, query, args...)
		return err
	})
	return result, err
}

// PrepareContext prepares a statement with automatic retry
func (t *TenantDB) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	var result *sql.Stmt
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.PrepareContext(ctx, query)
		return err
	})
	return result, err
}

// BeginTx starts a transaction with automatic retry
func (t *TenantDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	var result *sql.Tx
	err := t.executeWithRetry(ctx, func(db *sql.DB) error {
		var err error
		result, err = db.BeginTx(ctx, opts)
		return err
	})
	return result, err
}

// Close closes the underlying database connection
// Sets t.db to nil so subsequent queries will trigger reconnection
func (t *TenantDB) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.db != nil {
		err := t.db.Close()
		t.db = nil // Set to nil so reconnect() will be triggered on next query
		return err
	}
	return nil
}

// Ping checks if the connection is alive
func (t *TenantDB) Ping() error {
	t.mu.Lock()
	db := t.db
	t.mu.Unlock()
	if db == nil {
		return fmt.Errorf("connection is nil")
	}
	return db.Ping()
}

// Ensure TenantDB implements DBConn interface
var _ DBConn = (*TenantDB)(nil)

// GetRawDB extracts the underlying *sql.DB from a DBConn interface
// This is useful for legacy code that requires *sql.DB directly
// Note: If the DBConn is a TenantDB or TursoDB wrapper, this returns the underlying connection
func GetRawDB(conn DBConn) *sql.DB {
	if conn == nil {
		return nil
	}
	switch c := conn.(type) {
	case *sql.DB:
		return c
	case *TenantDB:
		return c.GetDB()
	case *TursoDB:
		return c.GetDB()
	default:
		// Try type assertion as last resort
		if rawDB, ok := conn.(*sql.DB); ok {
			return rawDB
		}
		return nil
	}
}

// QueryRowScan executes a single-row query and scans into dest with automatic retry
// This is a helper function for DBConn that enables retry for QueryRow operations
// For TenantDB connections, it uses the built-in retry logic
// For raw *sql.DB connections, it implements retry directly
func QueryRowScan(ctx context.Context, conn DBConn, dest []interface{}, query string, args ...interface{}) error {
	// If it's a TenantDB, use its retry-enabled method
	if tenantDB, ok := conn.(*TenantDB); ok {
		return tenantDB.QueryRowScanContext(ctx, dest, query, args...)
	}

	// For raw *sql.DB or other DBConn implementations, implement retry here
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		err := conn.QueryRowContext(ctx, query, args...).Scan(dest...)
		if err == nil {
			return nil
		}

		lastErr = err
		if isConnectionError(err) {
			log.Printf("[DB] QueryRowScan error (attempt %d/3): %v", attempt+1, err)
			time.Sleep(time.Duration(attempt*100) * time.Millisecond)
			continue
		}
		// Non-connection error (including sql.ErrNoRows), return immediately
		return err
	}
	return fmt.Errorf("query row scan failed after 3 attempts: %w", lastErr)
}
