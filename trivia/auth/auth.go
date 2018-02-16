package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/expixel/actual-trivia-server/eplog"
	"github.com/expixel/actual-trivia-server/trivia"
)

var logger = eplog.NewPrefixLogger("auth")

const maxTokenGenerationRetries = 5

var errTokenGenMaxReached = errors.New("auth: reached the maximum number of retries for token generation")

type service struct {
	users  trivia.UserService
	tokens trivia.AuthTokenService
}

func (s *service) LoginWithEmail(email string, password string) (*trivia.TokenPair, error) {
	creds, err := s.users.CredByEmail(email)
	if err != nil {
		return nil, err
	}
	if creds == nil {
		return nil, trivia.ErrUserNotFound
	}

	err = ComparePassword(creds.Password, password)
	if err != nil {
		return nil, trivia.ErrIncorrectPassword
	}

	authTokenString, refreshTokenString, err := s.generateTokenStrings()
	if err != nil {
		return nil, err
	}

	const authTokenExpiresIn time.Duration = 5 * time.Minute
	const refreshTokenExpiresIn time.Duration = (24 * time.Hour) * 7
	now := time.Now()
	authTokenExpiresAt := now.Add(authTokenExpiresIn)
	refreshTokenExpiresAt := now.Add(refreshTokenExpiresIn)

	authToken := &trivia.AuthToken{
		Token:     authTokenString,
		UserID:    creds.UserID,
		GuestID:   0,
		ExpiresAt: authTokenExpiresAt,
	}

	refreshToken := &trivia.RefreshToken{
		Token:     refreshTokenString,
		AuthToken: authTokenString,
		UserID:    creds.UserID,
		GuestID:   0,
		ExpiresAt: refreshTokenExpiresAt,
	}

	err = s.tokens.CreateTokenPair(authToken, refreshToken)
	if err != nil {
		return nil, err
	}

	pair := &trivia.TokenPair{Auth: authToken, Refresh: refreshToken}
	return pair, nil
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

func (s *service) generateTokenStrings() (string, string, error) {
	buffer := make([]byte, 32)
	authTokenString := ""
	refreshTokenString := ""

	// #CLEANUP this kind of looks dumb, I'll clean it up someday™
	currentTry := 0
	for {
		if len(authTokenString) < 1 {
			_, err := rand.Read(buffer)
			if err != nil {
				return "", "", err
			}
			authTokenString = hex.EncodeToString(buffer)

			exists, err := s.tokens.AuthTokenExists(authTokenString)
			if err != nil {
				return "", "", err
			}
			if exists {
				authTokenString = ""
			}
		}

		if len(refreshTokenString) < 1 {
			_, err := rand.Read(buffer)
			if err != nil {
				return "", "", err
			}
			refreshTokenString = hex.EncodeToString(buffer)

			exists, err := s.tokens.RefreshTokenExists(refreshTokenString)
			if err != nil {
				return "", "", err
			}
			if exists {
				refreshTokenString = ""
			}
		}

		if len(authTokenString) > 0 && len(refreshTokenString) > 0 {
			break
		}

		currentTry++
		if currentTry > maxTokenGenerationRetries {
			return "", "", errTokenGenMaxReached
		}
	}
	return authTokenString, refreshTokenString, nil
}

// NewService creates a new authentication service.
func NewService(users trivia.UserService, tokens trivia.AuthTokenService) trivia.AuthService {
	return &service{users: users, tokens: tokens}
}
