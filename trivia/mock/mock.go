package mock

import (
	"github.com/expixel/actual-trivia-server/trivia"
)

// DB is a mock database implementation used for testing services without
// using postgres.
type DB struct {
	users []trivia.User
	creds []trivia.UserCred
}

// NewDB creates a new Mock DB.
func NewDB() *DB {
	return &DB{
		users: make([]trivia.User, 0),
		creds: make([]trivia.UserCred, 0),
	}
}
