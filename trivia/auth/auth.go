package auth

import (
	"github.com/expixel/actual-trivia-server/eplog"
	"github.com/expixel/actual-trivia-server/trivia"
)

var logger = eplog.NewPrefixLogger("auth")

type service struct {
	users  trivia.UserService
	tokens trivia.AuthTokenService
}

func (s *service) LoginWithEmail(email string, password string) (*trivia.TokenPair, error) {
	// #TODO implement this shit
	return nil, nil
}

func (s *service) CreateUser(username string, email string, password string) (*trivia.User, *trivia.UserCred, error) {
	preparedPassword, err := PreparePassword(password)
	if err != nil {
		return nil, nil, err
	}

	// #CLEANUP I should merge the email and username search into a single query to reduce how much I'm hitting the database for signups.
	// Maybe something like:
	// 		SELECT u.username, c.email FROM users u
	// 		INNER JOIN user_creds c ON c.user_id = u.id WHERE lower(c.email) = {EMAIL} AND lower(c.username) = {USERNAME}
	// and then I can find how what's conflicting by comparing again on the client side.
	userFound, err := s.users.UserByUsername(username)
	if err != nil || userFound != nil {
		return nil, nil, trivia.ErrUsernameInUse
	}

	emailFound, err := s.users.CredByEmail(email)
	if err != nil || emailFound != nil {
		return nil, nil, trivia.ErrEmailInUse
	}

	user := &trivia.User{Username: username}
	creds := &trivia.UserCred{Email: email, Password: preparedPassword}
	if err = s.users.CreateUser(user, creds); err != nil {
		return nil, nil, err
	}

	return user, creds, nil
}

// NewService creates a new authentication service.
func NewService(users trivia.UserService, tokens trivia.AuthTokenService) trivia.AuthService {
	return &service{users: users, tokens: tokens}
}
