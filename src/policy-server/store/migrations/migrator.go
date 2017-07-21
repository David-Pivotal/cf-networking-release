package migrations

import (
	"database/sql"
	"fmt"

	"github.com/cf-container-networking/sql-migrate"
)

//go:generate counterfeiter -o fakes/migrate_adapter.go --fake-name MigrateAdapter . migrateAdapter
type migrateAdapter interface {
	ExecMax(db MigrationDb, dialect string, m migrate.MigrationSource, dir migrate.MigrationDirection, maxNumMigrations int) (int, error)
}

//go:generate counterfeiter -o fakes/migration_db.go --fake-name MigrationDb . MigrationDb
type MigrationDb interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	DriverName() string
}

type Migrator struct {
	MigrateAdapter migrateAdapter
}

func (m *Migrator) PerformMigrations(driverName string, migrationDb MigrationDb, maxNumMigrations int) (int, error) {
	if !migrationsToPerform.supportsDriver(driverName) {
		return 0, fmt.Errorf("unsupported driver: %s", driverName)
	}

	numMigrations, err := m.MigrateAdapter.ExecMax(
		migrationDb,
		driverName,
		migrate.MemoryMigrationSource{
			Migrations: migrationsToPerform.forDriver(driverName),
		},
		migrate.Up,
		maxNumMigrations,
	)

	if err != nil {
		return numMigrations, fmt.Errorf("executing migration: %s", err)
	}
	return numMigrations, nil
}


type policyServerMigrations []policyServerMigration

func (s policyServerMigrations) forDriver(driverName string) []*migrate.Migration {
	migrationMapped := []*migrate.Migration{}

	for _, migration := range s {
		migrationMapped = append(migrationMapped, migration.forDriver(driverName))
	}
	return migrationMapped
}

func (s policyServerMigrations) supportsDriver(driverName string) bool {
	for _, migration := range s {
		if !migration.supportsDriver(driverName) {
			return false
		}
	}
	return true
}

type policyServerMigration struct {
	Id   string
	Up   map[string][]string
	Down map[string][]string
}

func (psm *policyServerMigration) forDriver(driverName string) *migrate.Migration {
	return &migrate.Migration{
		Id:   psm.Id,
		Up:   psm.Up[driverName],
		Down: psm.Down[driverName],
	}
}

func (psm *policyServerMigration) supportsDriver(driverName string) bool {
	_, foundUp := psm.Up[driverName]
	_, foundDown := psm.Down[driverName]

	return foundUp && foundDown
}

var migrationDownNotImplemented = map[string][]string{
	"mysql": {
		``,
	},
	"postgres": {
		``,
	},
}