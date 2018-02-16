package postgres

import (
	"database/sql"
	"log"

	"github.com/expixel/actual-trivia-server/trivia"
)

type userService struct {
	db *sql.DB
}

func (s *userService) UserByID(id int) (*trivia.User, error) {
	var user trivia.User
	row := s.db.QueryRow(`SELECT id, username FROM users WHERE id = $1`, id)
	if err := row.Scan(&user.ID, &user.Username); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (s *userService) UserByUsername(username string) (*trivia.User, error) {
	var user trivia.User
	row := s.db.QueryRow(`SELECT id, username FROM users WHERE lower(username) = lower($1)`, username)
	if err := row.Scan(&user.ID, &user.Username); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (s *userService) CredByEmail(email string) (*trivia.UserCred, error) {
	var cred trivia.UserCred
	row := s.db.QueryRow(`SELECT user_id, email, password FROM user_creds WHERE lower(email) = lower($1)`, email)
	if err := row.Scan(&cred.UserID, &cred.Email, &cred.Password); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &cred, nil
}

func (s *userService) CreateUser(user *trivia.User, cred *trivia.UserCred) error {
	return transact(s.db, func(tx *sql.Tx) error {
		var userID int
		err := tx.QueryRow(`INSERT INTO users (username) VALUES ($1) RETURNING id`, user.Username).Scan(&userID)
		if err != nil {
			return err
		}

		user.ID = userID
		cred.UserID = userID

		_, err = tx.Exec(`INSERT INTO user_creds (user_id, email, password) VALUES ($1, $2, $3)`, cred.UserID, cred.Email, cred.Password)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *userService) DeleteUser(id int) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	aff, err := res.RowsAffected()
	if err != nil {
		// we shouldn't encounter this error ever so for now it's just logged and ignored.
		log.Println("error occurred while checking rows affected: ", err)
		return false, nil
	}
	return aff > 0, nil
}

// NewUserService returns a new user service backed by a postgres database.
func NewUserService(db *sql.DB) trivia.UserService {
	return &userService{db: db}
}
