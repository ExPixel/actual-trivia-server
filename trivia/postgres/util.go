package postgres

import (
	"database/sql"
	"errors"
)

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

	return tx.Commit()
}
