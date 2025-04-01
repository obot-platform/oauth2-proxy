package options

// SessionOptions contains configuration options for the SessionStore providers.
type SessionOptions struct {
	Type     string               `flag:"session-store-type" cfg:"session_store_type"`
	Cookie   CookieStoreOptions   `cfg:",squash"`
	Redis    RedisStoreOptions    `cfg:",squash"`
	Postgres PostgresStoreOptions `cfg:",squash"`
}

// CookieSessionStoreType is used to indicate the CookieSessionStore should be
// used for storing sessions.
var CookieSessionStoreType = "cookie"

// RedisSessionStoreType is used to indicate the RedisSessionStore should be
// used for storing sessions.
var RedisSessionStoreType = "redis"

// PostgresSessionStoreType is used to indicate the PostgresSessionStore should be
// used for storing sessions.
var PostgresSessionStoreType = "postgres"

// CookieStoreOptions contains configuration options for the CookieSessionStore.
type CookieStoreOptions struct {
	Minimal bool `flag:"session-cookie-minimal" cfg:"session_cookie_minimal"`
}

// RedisStoreOptions contains configuration options for the RedisSessionStore.
type RedisStoreOptions struct {
	ConnectionURL          string   `flag:"redis-connection-url" cfg:"redis_connection_url"`
	Username               string   `flag:"redis-username" cfg:"redis_username"`
	Password               string   `flag:"redis-password" cfg:"redis_password"`
	UseSentinel            bool     `flag:"redis-use-sentinel" cfg:"redis_use_sentinel"`
	SentinelPassword       string   `flag:"redis-sentinel-password" cfg:"redis_sentinel_password"`
	SentinelMasterName     string   `flag:"redis-sentinel-master-name" cfg:"redis_sentinel_master_name"`
	SentinelConnectionURLs []string `flag:"redis-sentinel-connection-urls" cfg:"redis_sentinel_connection_urls"`
	UseCluster             bool     `flag:"redis-use-cluster" cfg:"redis_use_cluster"`
	ClusterConnectionURLs  []string `flag:"redis-cluster-connection-urls" cfg:"redis_cluster_connection_urls"`
	CAPath                 string   `flag:"redis-ca-path" cfg:"redis_ca_path"`
	InsecureSkipTLSVerify  bool     `flag:"redis-insecure-skip-tls-verify" cfg:"redis_insecure_skip_tls_verify"`
	IdleTimeout            int      `flag:"redis-connection-idle-timeout" cfg:"redis_connection_idle_timeout"`
}

// PostgresStoreOptions contains configuration options for the PostgresSessionStore.
type PostgresStoreOptions struct {
	ConnectionDSN   string `flag:"postgres-connection-dsn" cfg:"postgres_connection_dsn"`
	MaxIdleConns    int    `flag:"postgres-max-idle-conns" cfg:"postgres_max_idle_conns"`
	MaxOpenConns    int    `flag:"postgres-max-open-conns" cfg:"postgres_max_open_conns"`
	ConnMaxLifetime int    `flag:"postgres-conn-max-lifetime" cfg:"postgres_conn_max_lifetime"`
	TableNamePrefix string `flag:"postgres-table-name-prefix" cfg:"postgres_table_name_prefix"`
}

func sessionOptionsDefaults() SessionOptions {
	return SessionOptions{
		Type: CookieSessionStoreType,
		Cookie: CookieStoreOptions{
			Minimal: false,
		},
	}
}
