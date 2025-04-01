package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/sessions"
	"gorm.io/gorm"
)

// SessionLock represents a lock record in the database
type SessionLock struct {
	Key       string `gorm:"primaryKey"`
	ExpiresAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Lock struct {
	db              *gorm.DB
	key             string
	tableNamePrefix string
}

// NewLock creates a new PostgreSQL lock instance
func NewLock(db *gorm.DB, key string, tableNamePrefix string) sessions.Lock {
	return &Lock{
		db:              db,
		key:             key,
		tableNamePrefix: tableNamePrefix,
	}
}

// Obtain obtains a lock by inserting a record into the session_lock table.
// If a lock already exists and hasn't expired, it will return ErrLockNotObtained.
func (l *Lock) Obtain(ctx context.Context, expiration time.Duration) error {
	// Verify database connection is healthy
	if err := l.verifyConnection(ctx); err != nil {
		return fmt.Errorf("database connection error: %w", err)
	}

	// Start a transaction to ensure atomicity
	tx := l.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("error starting transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Clean up expired locks
	if err := tx.Table(l.tableNamePrefix+"_session_locks").Where("expires_at < ?", time.Now()).Delete(&SessionLock{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("error cleaning up expired locks: %w", err)
	}

	// Check if lock exists and is valid
	var existingLock SessionLock
	err := tx.Table(l.tableNamePrefix+"_session_locks").Where("key = ?", l.key).First(&existingLock).Error
	if err == nil {
		// Lock exists and hasn't expired
		tx.Rollback()
		return sessions.ErrLockNotObtained
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		tx.Rollback()
		return fmt.Errorf("error checking existing lock: %w", err)
	}

	// Create new lock
	lock := &SessionLock{
		Key:       l.key,
		ExpiresAt: time.Now().Add(expiration),
	}
	if err := tx.Table(l.tableNamePrefix + "_session_locks").Create(lock).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("error creating lock: %w", err)
	}

	return tx.Commit().Error
}

// Peek checks if the lock is still held by checking if it exists and hasn't expired
func (l *Lock) Peek(ctx context.Context) (bool, error) {
	// Verify database connection is healthy
	if err := l.verifyConnection(ctx); err != nil {
		return false, fmt.Errorf("database connection error: %w", err)
	}

	// Clean up expired locks
	if err := l.db.WithContext(ctx).Table(l.tableNamePrefix+"_session_locks").Where("expires_at < ?", time.Now()).Delete(&SessionLock{}).Error; err != nil {
		return false, fmt.Errorf("error cleaning up expired locks: %w", err)
	}

	// Check if lock exists and is valid
	var lock SessionLock
	err := l.db.WithContext(ctx).Table(l.tableNamePrefix+"_session_locks").Where("key = ?", l.key).First(&lock).Error
	if err == nil {
		return true, nil
	}
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	return false, fmt.Errorf("error checking lock: %w", err)
}

// Refresh refreshes the lock by updating its expiration time
func (l *Lock) Refresh(ctx context.Context, expiration time.Duration) error {
	// Verify database connection is healthy
	if err := l.verifyConnection(ctx); err != nil {
		return fmt.Errorf("database connection error: %w", err)
	}

	// Start a transaction to ensure atomicity
	tx := l.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("error starting transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// First verify we hold the lock
	var existingLock SessionLock
	err := tx.Table(l.tableNamePrefix+"_session_locks").Where("key = ? AND expires_at > ?", l.key, time.Now()).First(&existingLock).Error
	if err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return sessions.ErrNotLocked
		}
		return fmt.Errorf("error checking existing lock: %w", err)
	}

	// Update lock expiration
	result := tx.Table(l.tableNamePrefix+"_session_locks").Model(&SessionLock{}).
		Where("key = ?", l.key).
		Update("expires_at", time.Now().Add(expiration))
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("error refreshing lock: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return sessions.ErrNotLocked
	}

	return tx.Commit().Error
}

// Release releases the lock by deleting the record from the session_lock table
func (l *Lock) Release(ctx context.Context) error {
	// Verify database connection is healthy
	if err := l.verifyConnection(ctx); err != nil {
		return fmt.Errorf("database connection error: %w", err)
	}

	// Start a transaction to ensure atomicity
	tx := l.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("error starting transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Clean up expired locks
	if err := tx.Table(l.tableNamePrefix+"_session_locks").Where("expires_at < ?", time.Now()).Delete(&SessionLock{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("error cleaning up expired locks: %w", err)
	}

	// Delete the lock
	result := tx.Table(l.tableNamePrefix+"_session_locks").Where("key = ?", l.key).Delete(&SessionLock{})
	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("error releasing lock: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		tx.Rollback()
		return sessions.ErrNotLocked
	}

	return tx.Commit().Error
}

// verifyConnection checks if the database connection is healthy
func (l *Lock) verifyConnection(ctx context.Context) error {
	sqlDB, err := l.db.DB()
	if err != nil {
		return fmt.Errorf("error getting database instance: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("error pinging database: %w", err)
	}
	return nil
}
