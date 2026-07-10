package postgres

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPostgres(t *testing.T) {
	if os.Getenv("POSTGRES_DSN") == "" {
		t.Skip("POSTGRES_DSN is not set")
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "PostgreSQL SessionStore Suite")
}
