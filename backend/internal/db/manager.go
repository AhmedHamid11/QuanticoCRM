package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
)

// TenantConnection holds a database connection and metadata
type TenantConnection struct {
	DB         *sql.DB   // Raw connection for backward compatibility
	TenantDB   *TenantDB // Wrapped connection with retry logic
	OrgID      string
	LastUsed   time.Time
	CreatedAt  time.Time
}

// Manager manages database connections for multi-tenant architecture
type Manager struct {
	masterDB    *sql.DB   // For local SQLite mode
	masterTurso *TursoDB  // For Turso mode with auto-reconnect
	connections map[string]*TenantConnection
	mu          sync.RWMutex

	// Configuration
	maxIdleTime     time.Duration
	cleanupInterval time.Duration
	maxConnsPerTenant int

	// For local development without Turso
	localMode   bool
	localDBPath string

	// Stop channel for cleanup goroutine
	stopCleanup chan struct{}
}

// ManagerConfig holds configuration for the connection manager
type ManagerConfig struct {
	MaxIdleTime       time.Duration
	CleanupInterval   time.Duration
	MaxConnsPerTenant int
	LocalMode         bool
	LocalDBPath       string
}

// DefaultManagerConfig returns sensible defaults
func DefaultManagerConfig() ManagerConfig {
	// Local mode if no Turso API credentials for tenant provisioning
	// Check both TURSO_API_TOKEN and TURSO_AUTH_TOKEN
	apiToken := os.Getenv("TURSO_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("TURSO_AUTH_TOKEN")
	}
	localMode := apiToken == "" || os.Getenv("TURSO_ORG") == ""
	return ManagerConfig{
		MaxIdleTime:       10 * time.Minute,
		CleanupInterval:   1 * time.Minute,
		MaxConnsPerTenant: 10,
		LocalMode:         localMode,
		LocalDBPath:       os.Getenv("DATABASE_PATH"),
	}
}

// NewManager creates a new tenant database manager
func NewManager(masterDB *sql.DB, config ManagerConfig) *Manager {
	m := &Manager{
		masterDB:          masterDB,
		connections:       make(map[string]*TenantConnection),
		maxIdleTime:       config.MaxIdleTime,
		cleanupInterval:   config.CleanupInterval,
		maxConnsPerTenant: config.MaxConnsPerTenant,
		localMode:         config.LocalMode,
		localDBPath:       config.LocalDBPath,
		stopCleanup:       make(chan struct{}),
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m
}

// NewManagerWithTurso creates a tenant database manager using TursoDB for master connection
// TursoDB provides automatic reconnection for Turso/libsql HTTP connections
func NewManagerWithTurso(turso *TursoDB, config ManagerConfig) *Manager {
	m := &Manager{
		masterTurso:       turso,
		connections:       make(map[string]*TenantConnection),
		maxIdleTime:       config.MaxIdleTime,
		cleanupInterval:   config.CleanupInterval,
		maxConnsPerTenant: config.MaxConnsPerTenant,
		localMode:         config.LocalMode,
		localDBPath:       config.LocalDBPath,
		stopCleanup:       make(chan struct{}),
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m
}

// GetMasterDB returns the master database connection
// When using TursoDB, this ensures the connection is valid and reconnects if needed
func (m *Manager) GetMasterDB() *sql.DB {
	// If using TursoDB, get fresh/valid connection
	if m.masterTurso != nil {
		db, err := m.masterTurso.DB()
		if err != nil {
			log.Printf("ERROR: Failed to get master DB from TursoDB: %v", err)
			return nil
		}
		return db
	}
	// Local mode - return stored connection
	return m.masterDB
}

// GetTenantDB gets or creates a database connection for a tenant
// This function handles connection health checks and automatic reconnection
// DEPRECATED: Use GetTenantDBConn for retry-enabled connections
func (m *Manager) GetTenantDB(ctx context.Context, orgID, dbURL, authToken string) (*sql.DB, error) {
	// In local mode, all orgs share the same database
	if m.localMode {
		return m.getLocalDB(orgID)
	}

	// Use write lock to avoid race conditions between health check and reconnection
	m.mu.Lock()
	conn, exists := m.connections[orgID]
	if exists {
		conn.LastUsed = time.Now()
		// Verify connection is still healthy before returning
		if err := conn.DB.PingContext(ctx); err != nil {
			// Connection is dead, close it and remove from cache
			log.Printf("[TENANT-DB] Org=%s Recreating dead connection: %v", orgID, err)
			conn.DB.Close()
			delete(m.connections, orgID)
			m.mu.Unlock()
			// Create new connection (will acquire its own lock)
			return m.createConnection(ctx, orgID, dbURL, authToken)
		}
		m.mu.Unlock()
		return conn.DB, nil
	}
	m.mu.Unlock()

	// Create new connection
	return m.createConnection(ctx, orgID, dbURL, authToken)
}

// GetTenantDBConn gets or creates a retry-enabled database connection for a tenant
// This returns a DBConn interface with automatic retry logic on connection errors
func (m *Manager) GetTenantDBConn(ctx context.Context, orgID, dbURL, authToken string) (DBConn, error) {
	// In local mode, return raw *sql.DB (which implements DBConn)
	if m.localMode {
		return m.getLocalDB(orgID)
	}

	m.mu.Lock()
	conn, exists := m.connections[orgID]
	if exists {
		conn.LastUsed = time.Now()
		// If TenantDB wrapper exists, use it
		if conn.TenantDB != nil {
			m.mu.Unlock()
			return conn.TenantDB, nil
		}
		// Fall back to raw DB check
		if err := conn.DB.PingContext(ctx); err != nil {
			log.Printf("[TENANT-DB] Org=%s Recreating dead connection: %v", orgID, err)
			conn.DB.Close()
			delete(m.connections, orgID)
			m.mu.Unlock()
			return m.createConnectionWithWrapper(ctx, orgID, dbURL, authToken)
		}
		m.mu.Unlock()
		// Create TenantDB wrapper for existing connection
		return NewTenantDB(conn.DB, orgID, dbURL, authToken), nil
	}
	m.mu.Unlock()

	// Create new connection with wrapper
	return m.createConnectionWithWrapper(ctx, orgID, dbURL, authToken)
}

// createConnectionWithWrapper creates a new tenant connection with TenantDB wrapper
func (m *Manager) createConnectionWithWrapper(ctx context.Context, orgID, dbURL, authToken string) (DBConn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := m.connections[orgID]; exists {
		conn.LastUsed = time.Now()
		if conn.TenantDB != nil {
			return conn.TenantDB, nil
		}
		return NewTenantDB(conn.DB, orgID, dbURL, authToken), nil
	}

	// Build connection string
	connStr := dbURL
	if authToken != "" {
		connStr = dbURL + "?authToken=" + authToken
	}

	db, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tenant database: %w", err)
	}

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping tenant database: %w", err)
	}

	// Configure connection pool for tenant
	db.SetMaxOpenConns(m.maxConnsPerTenant)
	db.SetMaxIdleConns(m.maxConnsPerTenant / 2)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create TenantDB wrapper
	tenantDB := NewTenantDB(db, orgID, dbURL, authToken)

	m.connections[orgID] = &TenantConnection{
		DB:        db,
		TenantDB:  tenantDB,
		OrgID:     orgID,
		LastUsed:  time.Now(),
		CreatedAt: time.Now(),
	}

	log.Printf("[TENANT-DB] Org=%s Created new connection with retry wrapper", orgID)
	return tenantDB, nil
}

// getLocalDB returns a connection to the local SQLite database
// Note: Due to SQLite driver behavior, connections may close between requests.
// This function detects closed connections and recreates them as needed.
func (m *Manager) getLocalDB(orgID string) (*sql.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections["local"]
	if exists {
		// Verify connection is still alive
		if err := conn.DB.Ping(); err != nil {
			log.Printf("Local DB connection dead, recreating: %v", err)
			conn.DB.Close()
			delete(m.connections, "local")
		} else {
			conn.LastUsed = time.Now()
			return conn.DB, nil
		}
	}

	// Create new local connection
	dbPath := m.localDBPath
	if dbPath == "" {
		dbPath = "../fastcrm.db"
	}

	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local database: %w", err)
	}

	// Configure connection pool for better SQLite handling
	db.SetMaxOpenConns(1) // SQLite works best with a single connection
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // Don't expire connections

	m.connections["local"] = &TenantConnection{
		DB:        db,
		OrgID:     "local",
		LastUsed:  time.Now(),
		CreatedAt: time.Now(),
	}

	log.Printf("Created local database connection: %s", dbPath)
	return db, nil
}

// createConnection creates a new tenant database connection
func (m *Manager) createConnection(ctx context.Context, orgID, dbURL, authToken string) (*sql.DB, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if conn, exists := m.connections[orgID]; exists {
		conn.LastUsed = time.Now()
		return conn.DB, nil
	}

	// Build connection string
	connStr := dbURL
	if authToken != "" {
		connStr = dbURL + "?authToken=" + authToken
	}

	db, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to tenant database: %w", err)
	}

	// Verify connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping tenant database: %w", err)
	}

	// Configure connection pool for tenant
	db.SetMaxOpenConns(m.maxConnsPerTenant)
	db.SetMaxIdleConns(m.maxConnsPerTenant / 2)
	db.SetConnMaxLifetime(5 * time.Minute)

	m.connections[orgID] = &TenantConnection{
		DB:        db,
		OrgID:     orgID,
		LastUsed:  time.Now(),
		CreatedAt: time.Now(),
	}

	log.Printf("Created tenant database connection for org %s", orgID)
	return db, nil
}

// cleanupLoop periodically closes idle connections
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupIdleConnections()
		case <-m.stopCleanup:
			return
		}
	}
}

// cleanupIdleConnections closes connections that have been idle too long
func (m *Manager) cleanupIdleConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	closedCount := 0
	for orgID, conn := range m.connections {
		// Don't close local connection
		if orgID == "local" {
			continue
		}

		idleTime := now.Sub(conn.LastUsed)
		if idleTime > m.maxIdleTime {
			// Check if the connection has active SQL queries before closing.
			// During long operations (e.g. large CSV imports), the connection is
			// actively used but LastUsed only updates on acquisition, not on each query.
			if conn.DB != nil {
				stats := conn.DB.Stats()
				if stats.InUse > 0 {
					// Connection is actively executing queries — update LastUsed and skip
					conn.LastUsed = now
					log.Printf("[CONN-MANAGER] Org=%s Connection appears idle (%v) but has %d active queries, keeping alive", orgID, idleTime, stats.InUse)
					continue
				}
			}

			log.Printf("[CONN-MANAGER] Org=%s Closing IDLE connection (idle for %v)", orgID, idleTime)
			// If TenantDB wrapper exists, use its Close method which handles cleanup properly
			// This ensures the wrapper knows the connection is closed and can reconnect later
			if conn.TenantDB != nil {
				conn.TenantDB.Close()
			} else if conn.DB != nil {
				conn.DB.Close()
			}
			delete(m.connections, orgID)
			closedCount++
		}
	}
	if closedCount > 0 {
		log.Printf("[CONN-MANAGER] Cleanup complete: closed %d idle connections, %d active", closedCount, len(m.connections))
	}
}

// TouchConnection updates the LastUsed timestamp for an org's connection.
// Call this during long-running operations to prevent the cleanup loop from
// closing the connection while it's in use.
func (m *Manager) TouchConnection(orgID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn, exists := m.connections[orgID]; exists {
		conn.LastUsed = time.Now()
	}
}

// CloseConnection closes a specific tenant connection
func (m *Manager) CloseConnection(orgID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, exists := m.connections[orgID]
	if !exists {
		return nil
	}

	if err := conn.DB.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	delete(m.connections, orgID)
	log.Printf("Closed connection for org %s", orgID)
	return nil
}

// Close closes all connections and stops the cleanup goroutine
func (m *Manager) Close() error {
	log.Printf("[CONN-MANAGER] Shutting down... closing %d connections", len(m.connections))
	close(m.stopCleanup)

	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	closedCount := 0
	for orgID, conn := range m.connections {
		if err := conn.DB.Close(); err != nil {
			lastErr = err
			log.Printf("[CONN-MANAGER] Org=%s Error closing connection: %v", orgID, err)
		} else {
			log.Printf("[CONN-MANAGER] Org=%s Connection CLOSED", orgID)
			closedCount++
		}
	}

	m.connections = make(map[string]*TenantConnection)
	log.Printf("[CONN-MANAGER] Shutdown complete: closed %d connections", closedCount)
	return lastErr
}

// Stats returns statistics about the connection pool
func (m *Manager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["total_connections"] = len(m.connections)
	stats["local_mode"] = m.localMode

	connStats := make([]map[string]interface{}, 0, len(m.connections))
	for orgID, conn := range m.connections {
		connStats = append(connStats, map[string]interface{}{
			"org_id":     orgID,
			"created_at": conn.CreatedAt,
			"last_used":  conn.LastUsed,
			"idle_time":  time.Since(conn.LastUsed).String(),
		})
	}
	stats["connections"] = connStats

	return stats
}

// IsLocalMode returns whether the manager is in local development mode
func (m *Manager) IsLocalMode() bool {
	return m.localMode
}
