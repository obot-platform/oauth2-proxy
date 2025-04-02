package postgres

import (
	"context"
	"os"
	"time"

	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/options"
	sessionsapi "github.com/oauth2-proxy/oauth2-proxy/v7/pkg/apis/sessions"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/sessions/persistence"
	"github.com/oauth2-proxy/oauth2-proxy/v7/pkg/sessions/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("PostgreSQL SessionStore Tests", func() {
	var ss sessionsapi.SessionStore
	var db *gorm.DB
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
		// Get DSN from environment or use default
		dsn := os.Getenv("POSTGRES_DSN")
		if dsn == "" {
			dsn = "postgres://postgres:postgres@localhost:5432/oauth2_proxy_test?sslmode=disable"
		}

		// Create test database connection
		var err error
		db, err = NewPostgresClient(options.PostgresStoreOptions{
			ConnectionDSN: dsn,
		})
		Expect(err).ToNot(HaveOccurred())

		err = db.AutoMigrate(&Session{}, &SessionLock{})
		Expect(err).ToNot(HaveOccurred())

		// Clean up any existing sessions
		err = db.Exec("TRUNCATE TABLE sessions, session_locks").Error
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		if db != nil {
			sqlDB, err := db.DB()
			Expect(err).ToNot(HaveOccurred())
			err = sqlDB.Close()
			Expect(err).ToNot(HaveOccurred())
		}
	})

	JustAfterEach(func() {
		// Release any connections immediately after the test ends
		if postgresManager, ok := ss.(*persistence.Manager); ok {
			if postgresManager.Store.(*SessionStore).db != nil {
				sqlDB, err := postgresManager.Store.(*SessionStore).db.DB()
				Expect(err).ToNot(HaveOccurred())
				Expect(sqlDB.Close()).To(Succeed())
			}
		}
	})

	tests.RunSessionStoreTests(
		func(opts *options.SessionOptions, cookieOpts *options.Cookie) (sessionsapi.SessionStore, error) {
			// Set the connection URL from environment or default
			dsn := os.Getenv("POSTGRES_DSN")
			if dsn == "" {
				dsn = "postgres://postgres:postgres@localhost:5432/oauth2_proxy_test?sslmode=disable"
			}
			opts.Type = options.PostgresSessionStoreType
			opts.Postgres.ConnectionDSN = dsn
			opts.Postgres.TableNamePrefix = "oauth2_proxy_test_"

			// Create new session store
			var err error
			ss, err = NewPostgresSessionStore(opts, cookieOpts)
			return ss, err
		},
		func(d time.Duration) error {
			// Fast forward the database time by updating session and lock expiration times
			if err := db.Exec("UPDATE oauth2_proxy_test_sessions SET expires_at = expires_at - $1::interval", d).Error; err != nil {
				return err
			}
			return db.Exec("UPDATE oauth2_proxy_test_session_locks SET expires_at = expires_at - $1::interval", d).Error
		},
	)

	Context("when using locks", func() {
		var store *SessionStore

		BeforeEach(func() {
			store = &SessionStore{db: db, tableNamePrefix: "oauth2_proxy_test_"}
			db.Table("oauth2_proxy_test_sessions").AutoMigrate(&Session{})
			db.Table("oauth2_proxy_test_session_locks").AutoMigrate(&SessionLock{})
		})

		It("obtains and releases locks", func() {
			lock := store.Lock("test-key-1")

			// Obtain lock
			err := lock.Obtain(ctx, time.Minute)
			Expect(err).ToNot(HaveOccurred())

			// Verify lock is held
			isLocked, err := lock.Peek(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(isLocked).To(BeTrue())

			// Release lock
			err = lock.Release(ctx)
			Expect(err).ToNot(HaveOccurred())

			// Verify lock is released
			isLocked, err = lock.Peek(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(isLocked).To(BeFalse())
		})

		It("handles concurrent locks", func() {
			lock := store.Lock("test-key-2")

			// First lock
			err := lock.Obtain(ctx, time.Minute)
			Expect(err).ToNot(HaveOccurred())

			// Try to obtain same lock with different instance
			lock2 := store.Lock("test-key-2")
			err = lock2.Obtain(ctx, time.Minute)
			Expect(err).To(Equal(sessionsapi.ErrLockNotObtained))

			// Release first lock
			err = lock.Release(ctx)
			Expect(err).ToNot(HaveOccurred())

			// Now second lock should work
			err = lock2.Obtain(ctx, time.Minute)
			Expect(err).ToNot(HaveOccurred())
		})

		It("handles lock refresh", func() {
			lock := store.Lock("test-key-3")

			// Obtain lock
			err := lock.Obtain(ctx, time.Minute)
			Expect(err).ToNot(HaveOccurred())

			// Refresh lock
			err = lock.Refresh(ctx, 2*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			// Verify lock is still held
			isLocked, err := lock.Peek(ctx)
			Expect(err).ToNot(HaveOccurred())
			Expect(isLocked).To(BeTrue())
		})
	})
})
