package migrations

import (
	"database/sql"
)

// initializes the database with some functions
func mg001InitDB(tx *sql.Tx) (err error) {
	// creates the trigger for updating the modified column on tables.
	_, err = tx.Exec(`
		CREATE OR REPLACE FUNCTION update_modified_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = now();
			RETURN NEW;
		END
		$$ language 'plpgsql';
	`)
	if err != nil {
		return
	}

	// adds the uuid-ossp extension, I don't think I use it for this database
	// but I usually do at some point. This comment will be outdated soon :)
	_, err = tx.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`)
	if err != nil {
		return
	}

	return
}

func mg002CreateUserTable(tx *sql.Tx) (err error) {
	// creates the users table.
	_, err = tx.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			username VARCHAR(128) NOT NULL,
			created TIMESTAMPTZ DEFAULT now(),
			modified TIMESTAMPTZ DEFAULT now()
		);
	`)
	if err != nil {
		return err
	}

	// creates a unique index on lowercase usernames
	_, err = tx.Exec(`
		CREATE UNIQUE INDEX unique_lower_username ON users(lower(username));
	`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TRIGGER update_users_modified
			BEFORE UPDATE ON users
			FOR EACH ROW
			EXECUTE PROCEDURE update_modified_column();
	`)
	if err != nil {
		return err
	}

	return
}

func mg003CreateUserCredsTable(tx *sql.Tx) (err error) {
	_, err = tx.Exec(`
		CREATE TABLE user_creds(
			user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			email VARCHAR(128) NOT NULL,
			password BYTEA,
			created TIMESTAMPTZ DEFAULT now(),
			modified TIMESTAMPTZ DEFAULT now()
		);
	`)
	if err != nil {
		return
	}

	_, err = tx.Exec(`
		CREATE UNIQUE INDEX unique_user_emails ON user_creds(lower(email));
	`)
	if err != nil {
		return
	}

	_, err = tx.Exec(`
		CREATE TRIGGER update_user_creds_modified
			BEFORE UPDATE ON user_creds
			FOR EACH ROW
			EXECUTE PROCEDURE update_modified_column();
	`)
	if err != nil {
		return
	}

	return
}

func mg004CreateAuthTokensTable(tx *sql.Tx) (err error) {
	_, err = tx.Exec(`
		CREATE TABLE auth_tokens(
			token CHAR(64) NOT NULL UNIQUE,
			user_id BIGINT,
			guest_id BIGINT,
			expires_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return
	}

	_, err = tx.Exec(`
		CREATE TABLE refresh_tokens(
			token CHAR(64) NOT NULL UNIQUE,
			auth_token CHAR(64) NOT NULL,
			user_id BIGINT,
			guest_id BIGINT,
			expires_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		return
	}

	return
}
