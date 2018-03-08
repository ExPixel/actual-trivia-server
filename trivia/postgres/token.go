package postgres

import (
	"database/sql"

	"github.com/expixel/actual-trivia-server/trivia/null"

	"github.com/expixel/actual-trivia-server/trivia"
)

type tokenService struct {
	db *sql.DB
}

func (s *tokenService) AuthTokenByString(tokenString string) (*trivia.AuthToken, error) {
	token := &trivia.AuthToken{}
	err := s.db.QueryRow("SELECT token, user_id, guest_id, expires_at FROM auth_tokens WHERE token = $1;", tokenString).Scan(
		&token.Token,
		&token.UserID,
		&token.GuestID,
		&token.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return token, nil
}

func (s *tokenService) CreateTokenPair(auth *trivia.AuthToken, refresh *trivia.RefreshToken) error {
	return transact(s.db, func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO auth_tokens (token, user_id, guest_id, expires_at) VALUES ($1, $2, $3, $4)`,
			auth.Token, auth.UserID, auth.GuestID, auth.ExpiresAt)
		if err != nil {
			return err
		}

		_, err = tx.Exec(
			`INSERT INTO refresh_tokens (token, auth_token, user_id, guest_id, expires_at) VALUES ($1, $2, $3, $4, $5)`,
			refresh.Token, refresh.AuthToken, refresh.UserID, refresh.GuestID, refresh.ExpiresAt)
		if err != nil {
			return err
		}

		return nil
	})
}

func (s *tokenService) AuthTokenExists(token string) (bool, error) {
	err := s.db.QueryRow("SELECT user_id FROM auth_tokens WHERE token = $1", token).Scan()
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *tokenService) RefreshTokenExists(token string) (bool, error) {
	err := s.db.QueryRow("SELECT user_id FROM refresh_tokens WHERE token = $1", token).Scan()
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *tokenService) GetAuthTokenAndUser(token string) (*trivia.AuthToken, *trivia.User, error) {
	authToken := &trivia.AuthToken{}
	var nullUserID null.Int64
	var nullUsername null.String

	err := s.db.QueryRow(`
		SELECT
			a.user_id, a.guest_id, a.expires_at,
			u.id, u.username
		FROM auth_tokens a
		LEFT JOIN users u ON (a.user_id = u.id)
		WHERE a.token = $1;
	`, token).Scan(&authToken.UserID, &authToken.GuestID, &authToken.ExpiresAt, &nullUserID, &nullUsername)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	authToken.Token = token
	if !nullUserID.Valid || !nullUsername.Valid {
		if authToken.GuestID.Valid {
			return authToken, trivia.NewGuestUser(authToken.GuestID), nil
		}
		return authToken, nil, trivia.ErrUserNotFound
	}
	user := &trivia.User{ID: nullUserID.Int64, Username: nullUsername.String}

	return authToken, user, nil
}

// NewTokenService creats a use AuthTokenService
func NewTokenService(db *sql.DB) trivia.AuthTokenService {
	return &tokenService{db: db}
}
