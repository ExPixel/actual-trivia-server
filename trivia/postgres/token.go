package postgres

import (
	"database/sql"

	"github.com/expixel/actual-trivia-server/trivia"
)

type tokenService struct {
	db *sql.DB
}

func (s *tokenService) AuthTokenByString(token string) (*trivia.AuthToken, error) {
	// #TODO not yet implemented.
	return nil, nil
}

func (s *tokenService) CreateTokenPair(auth *trivia.AuthToken, refresh *trivia.RefreshToken) error {
	// #TODO not yet implemented
	return nil
}

// NewTokenService creats a use AuthTokenService
func NewTokenService(db *sql.DB) trivia.AuthTokenService {
	return &tokenService{db: db}
}
