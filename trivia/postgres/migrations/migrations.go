package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/expixel/actual-trivia-server/eplog"
)

// #NOTE migrations go here :)
func init() {
	register(1, "init", mg001InitDB)
	register(2, "create_users_table", mg002CreateUserTable)
	register(3, "create_user_creds_table", mg003CreateUserCredsTable)
	register(4, "create_auth_tokens_table", mg004CreateAuthTokensTable)
	register(5, "create_guest_id_sequence", mg005CreateGuestSequence)
	register(6, "create_questions_table", mg006CreateQuestionsTable)
}

// MigrationFunc is a function that executes a migration on a transaction.
type MigrationFunc func(*sql.Tx) error

// Migration represents an update to the database.
type Migration struct {
	Version int
	Name    string
	Func    MigrationFunc
}

var logger = eplog.NewPrefixLogger("migrations")
var migrations = make([]Migration, 0)

// RunMigrations runs all of the latest defined migrations and returns true if it completed successfully.
func RunMigrations(db *sql.DB) (success bool) {
	success = false

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS ep_migrations(
		version INTEGER PRIMARY KEY,
		name TEXT
	);`)
	if err != nil {
		logger.Error("error creating migrations table: ", err)
		return
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS ep_migrations_lock(
		mg_lock BOOLEAN
	);`)
	if err != nil {
		logger.Error("error creating migration locking table: ", err)
		return
	}

	if err = waitForMigrationLock(db, 1*time.Second, 10*time.Second); err != nil {
		logger.Error("error while waiting for migration lock: ", err)
		return
	}

	completedMigrations := 0
	defer func() {
		if err = unlockMigrations(db); err != nil {
			logger.Error("error while unlocking migrations: ", err)
			success = false
		} else {
			if completedMigrations < 1 {
				logger.Info("no new migrations to complete")
			} else {
				logger.Info("%d migrations completed successfully", completedMigrations)
			}
		}
	}()
	if err = lockMigrations(db); err != nil {
		logger.Error("error while locking migrations: ", err)
		return
	}

	latest, err := getLatestMigration(db)
	if err != nil {
		logger.Error("error while getting latest migration: ", err)
		return
	}

	for _, m := range migrations {
		if latest != nil && m.Version <= latest.Version {
			logger.Debug("skipped migration %d_%s", m.Version, m.Name)
			continue
		}

		tx, err := db.Begin()
		if err != nil {
			logger.Error("error while starting transaction for migration %d_%s: ", m.Version, m.Name, err)
			return
		}

		err = m.Func(tx)
		if err != nil {
			logger.Error("error while executing migration %d_%s: ", m.Version, m.Name, err)
			err = tx.Rollback()
			if err != nil {
				logger.Error("error while rolling back migration %d_%s: ", m.Version, m.Name, err)
			}
			return
		}
		err = tx.Commit()
		if err != nil {
			logger.Error("error while committing migration %d_%s: ", m.Version, m.Name, err)
			return
		}

		err = setLatestMigration(db, &m)
		if err != nil {
			logger.Error("error while setting latest migration to %d_%s: ", m.Version, m.Name, err)
			return
		}

		logger.Info("executed migration %d_%s", m.Version, m.Name)
		completedMigrations++
	}

	success = true
	return
}

func setLatestMigration(db *sql.DB, m *Migration) (err error) {
	tx, err := db.Begin()
	if err != nil {
		logger.Error("error while starting transaction for migration %d_%s: ", m.Version, m.Name, err)
		return err
	}
	defer func() {
		if err != nil {
			logger.Error("error while executing migration %d_%s: ", m.Version, m.Name, err)
			rollErr := tx.Rollback()
			if rollErr != nil {
				logger.Error("error while rolling back migration %d_%s: ", m.Version, m.Name, rollErr)
				err = rollErr
			}
		} else {
			err = tx.Commit()
			if err != nil {
				logger.Error("error while committing migration %d_%s: ", m.Version, m.Name, err)
			}
		}
	}()

	_, err = tx.Exec(`DELETE FROM ep_migrations;`)
	if err != nil {
		return
	}

	_, err = tx.Exec(`INSERT INTO ep_migrations(version, name) VALUES ($1, $2);`, m.Version, m.Name)
	if err != nil {
		return
	}

	return
}

func getLatestMigration(db *sql.DB) (*Migration, error) {
	m := &Migration{}
	err := db.QueryRow(`SELECT version, name FROM ep_migrations ORDER BY version DESC LIMIT 1`).Scan(&m.Version, &m.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return m, nil
}

func waitForMigrationLock(db *sql.DB, checkPauseDelay time.Duration, timeout time.Duration) error {
	startTime := time.Now()
	for {
		var locked bool
		err := db.QueryRow(`SELECT mg_lock FROM ep_migrations_lock WHERE mg_lock = TRUE LIMIT 1;`).Scan(&locked)
		if err != nil {
			if err == sql.ErrNoRows {
				break
			} else {
				return err
			}
		}

		if time.Since(startTime) > timeout {
			return errors.New("timeout occurred while waiting for migration lock")
		}
	}
	return nil
}

func lockMigrations(db *sql.DB) error {
	logger.Debug("locking migrations...")
	_, err := db.Exec(`INSERT INTO ep_migrations_lock(mg_lock) VALUES (true);`)
	return err
}

func unlockMigrations(db *sql.DB) error {
	logger.Debug("unlocking migrations...")
	_, err := db.Exec(`DELETE FROM ep_migrations_lock;`)
	return err
}

// register registers a new migration
func register(version int, name string, f MigrationFunc) {
	for _, m := range migrations {
		if m.Version == version {
			panic(fmt.Sprintf("A migration with the version %d already exists.", version))
		}
	}

	migrations = append(migrations, Migration{
		Version: version,
		Name:    name,
		Func:    f,
	})
}

// tryRollback attempts to rollback a transaction after an error.
// If an error occurs during rollback, tryRollback will return a new
// error with information from the original error and the rollback error merged.
func tryRollback(tx *sql.Tx, err error) error {
	rollbackErr := tx.Rollback()
	if rollbackErr != nil {
		return errors.New(rollbackErr.Error() + err.Error())
	}
	return err
}

func transact(db *sql.DB, transactionFn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	err = transactionFn(tx)
	if err != nil {
		return tryRollback(tx, err)
	}

	return nil
}
