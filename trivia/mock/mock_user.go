package mock

import (
	"strings"

	"github.com/expixel/actual-trivia-server/trivia"
)

type mockUserService struct {
	db *DB
}

func (s *mockUserService) UserByID(id int) (*trivia.User, error) {
	for _, u := range s.db.users {
		if u.ID == id {
			return &u, nil
		}
	}
	return nil, nil
}

func (s *mockUserService) UserByUsername(username string) (*trivia.User, error) {
	for _, u := range s.db.users {
		if strings.EqualFold(u.Username, username) {
			return &u, nil
		}
	}
	return nil, nil
}

func (s *mockUserService) CredByEmail(email string) (*trivia.UserCred, error) {
	for _, c := range s.db.creds {
		if strings.EqualFold(email, c.Email) {
			return &c, nil
		}
	}
	return nil, nil
}

func (s *mockUserService) CreateUser(user *trivia.User, cred *trivia.UserCred) error {
	userCopy := *user
	credCopy := *cred
	s.db.users = append(s.db.users, userCopy)
	s.db.creds = append(s.db.creds, credCopy)
	return nil
}

func (s *mockUserService) DeleteUser(id int) (bool, error) {
	for idx, u := range s.db.users {
		if u.ID == id {
			s.db.users = append(s.db.users[:idx], s.db.users[idx+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// NewUserService creates a new mock user service.
func NewUserService(db *DB) trivia.UserService {
	return &mockUserService{db: db}
}
