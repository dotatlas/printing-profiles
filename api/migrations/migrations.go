// Package migrations holds the database migrations as SQL files inside the binary. The migrate command and the test databases use them.
package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// files holds every *.sql migration. goose applies them in the order of their numbers.
//
//go:embed *.sql
var files embed.FS

// NewProvider returns a goose provider for these migrations on db. The migrate command and the tests call Up or DownTo on it.
func NewProvider(db *sql.DB) (*goose.Provider, error) {
	// The lock stops two migrate runs from changing the database at the same time.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, db, files, goose.WithSessionLocker(locker))
}
