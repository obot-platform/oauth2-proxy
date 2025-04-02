package postgres

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/sessions"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/sessions/persistence"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SessionStore is an implementation of the persistence.Store
// interface that stores sessions in PostgreSQL
type SessionStore struct {
	db              *gorm.DB
	tableNamePrefix string
}

// Session represents a session record in the database
type Session struct {
	Key       string `gorm:"primaryKey"`
	Value     []byte `gorm:"type:bytea"`
	ExpiresAt time.Time
	User      []byte `gorm:"type:bytea"`
	Email     []byte `gorm:"type:bytea"`
}

// NewPostgresSessionStore initialises a new instance of the SessionStore and wraps
// it in a persistence.Manager
func NewPostgresSessionStore(opts *options.SessionOptions, cookieOpts *options.Cookie) (sessions.SessionStore, error) {
	db, err := NewPostgresClient(opts.Postgres)
	if err != nil {
		return nil, fmt.Errorf("error constructing postgres client: %v", err)
	}

	// Auto-migrate the session and lock tables
	if err := db.Table(opts.Postgres.TableNamePrefix + "sessions").AutoMigrate(&Session{}); err != nil {
		return nil, fmt.Errorf("error migrating postgres schema: %v", err)
	}
	if err := db.Table(opts.Postgres.TableNamePrefix + "session_locks").AutoMigrate(&SessionLock{}); err != nil {
		return nil, fmt.Errorf("error migrating postgres schema: %v", err)
	}

	ps := &SessionStore{
		db:              db,
		tableNamePrefix: opts.Postgres.TableNamePrefix,
	}

	return persistence.NewManager(ps, cookieOpts), nil
}

// Save takes a sessions.SessionState and stores the information from it
// to PostgreSQL, and adds a new persistence cookie on the HTTP response writer
func (store *SessionStore) Save(ctx context.Context, key string, value []byte, exp time.Duration, user, email string) error {
	// We store the username and email as hashes so that we can associate sessions with particular users.
	// This will allow us, in the future, to delete all sessions for a particular user.
	userHash := sha256.Sum256([]byte(user))
	emailHash := sha256.Sum256([]byte(email))

	session := &Session{
		Key:       key,
		Value:     value,
		ExpiresAt: time.Now().Add(exp),
		User:      userHash[:],
		Email:     emailHash[:],
	}

	result := store.db.WithContext(ctx).Table(store.tableNamePrefix + "sessions").Save(session)
	if result.Error != nil {
		return fmt.Errorf("error saving postgres session: %v", result.Error)
	}
	return nil
}

// Load reads sessions.SessionState information from a persistence
// cookie within the HTTP request object
func (store *SessionStore) Load(ctx context.Context, key string) ([]byte, error) {
	var session Session
	result := store.db.WithContext(ctx).Table(store.tableNamePrefix+"sessions").Where("key = ? AND expires_at > ?", key, time.Now()).First(&session)
	if result.Error != nil {
		return nil, fmt.Errorf("error loading postgres session: %v", result.Error)
	}
	return session.Value, nil
}

// Clear clears any saved session information for a given persistence cookie
// from PostgreSQL, and then clears the session
func (store *SessionStore) Clear(ctx context.Context, key string) error {
	result := store.db.WithContext(ctx).Table(store.tableNamePrefix+"sessions").Where("key = ?", key).Delete(&Session{})
	if result.Error != nil {
		return fmt.Errorf("error clearing the session from postgres: %v", result.Error)
	}
	return nil
}

// Lock creates a lock object for sessions.SessionState
func (store *SessionStore) Lock(key string) sessions.Lock {
	return NewLock(store.db, key, store.tableNamePrefix)
}

// VerifyConnection verifies the postgres connection is valid and the
// server is responsive
func (store *SessionStore) VerifyConnection(ctx context.Context) error {
	sqlDB, err := store.db.DB()
	if err != nil {
		return fmt.Errorf("error getting database instance: %v", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("error pinging postgres: %v", err)
	}
	return nil
}

// NewPostgresClient creates a new GORM database connection
func NewPostgresClient(opts options.PostgresStoreOptions) (*gorm.DB, error) {
	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	db, err := gorm.Open(postgres.Open(opts.ConnectionDSN), config)
	if err != nil {
		return nil, fmt.Errorf("error connecting to postgres: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error getting database instance: %v", err)
	}

	// Set connection pool settings
	if opts.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(opts.ConnMaxLifetime) * time.Second)
	}

	return db, nil
}

var _ persistence.Store = (*SessionStore)(nil)
