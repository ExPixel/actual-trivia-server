package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/expixel/actual-trivia-server/trivia/validate"

	"github.com/expixel/actual-trivia-server/eplog"
	"github.com/expixel/actual-trivia-server/trivia"
	"github.com/expixel/actual-trivia-server/trivia/null"
)

var logger = eplog.NewPrefixLogger("auth")

const maxTokenGenerationRetries = 2

var errTokenGenMaxReached = errors.New("auth: reached the maximum number of retries for token generation")

type service struct {
	users  trivia.UserService
	tokens trivia.AuthTokenService
}

func (s *service) LoginWithEmailOrUsername(emailOrUsername string, password string) (*trivia.TokenPair, error) {
	var creds *trivia.UserCred
	var err error

	if validate.IsEmail(emailOrUsername) {
		creds, err = s.users.CredByEmail(emailOrUsername)
	} else {
		creds, err = s.users.CredByUsername(emailOrUsername)
	}

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

	authTokenString, refreshTokenString, err := s.generateTokenStrings(creds.UserID, false)
	if err != nil {
		return nil, err
	}
	return s.storeTokenStrings(null.NewInt64(creds.UserID), null.Int64{}, authTokenString, refreshTokenString)
}

func (s *service) LoginAsGuest() (*trivia.TokenPair, error) {
	guestID, err := s.users.NextGuestID()
	if err != nil {
		return nil, err
	}

	authTokenString, refreshTokenString, err := s.generateTokenStrings(guestID, true)
	if err != nil {
		return nil, err
	}

	return s.storeTokenStrings(null.Int64{}, null.NewInt64(guestID), authTokenString, refreshTokenString)
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

// storeTokenStrings stores the generated auth and refresh tokens into the database and
// returns the stored token pair.
func (s *service) storeTokenStrings(userID null.Int64, guestID null.Int64, authTokenString string, refreshTokenString string) (*trivia.TokenPair, error) {
	// #FIXME this is annoying as hell for manual testing right now so I'm temporarily extended the
	// token expiration delay. Will definitely have to change this back to something reasonable
	// once I have a good API consumer set up (probably in a React application)

	// const authTokenExpiresIn time.Duration = 5 * time.Minute
	// const refreshTokenExpiresIn time.Duration = (24 * time.Hour) * 7

	// #FIXME REMOVE THESE
	const authTokenExpiresIn time.Duration = 14 * (24 * time.Hour)
	const refreshTokenExpiresIn time.Duration = 30 * (24 * time.Hour)

	now := time.Now()
	authTokenExpiresAt := now.Add(authTokenExpiresIn)
	refreshTokenExpiresAt := now.Add(refreshTokenExpiresIn)

	authToken := &trivia.AuthToken{
		Token:     authTokenString,
		UserID:    userID,
		GuestID:   guestID,
		ExpiresAt: authTokenExpiresAt,
	}

	refreshToken := &trivia.RefreshToken{
		Token:     refreshTokenString,
		AuthToken: authTokenString,
		UserID:    userID,
		GuestID:   guestID,
		ExpiresAt: refreshTokenExpiresAt,
	}

	err := s.tokens.CreateTokenPair(authToken, refreshToken)
	if err != nil {
		return nil, err
	}

	pair := &trivia.TokenPair{Auth: authToken, Refresh: refreshToken}
	return pair, nil
}

func (s *service) generateTokenStrings(userID int64, isGuest bool) (string, string, error) {
	useIDStr := strconv.FormatInt(userID, 36)
	if isGuest {
		useIDStr = "0." + useIDStr
	}

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
			authTokenString = hex.EncodeToString(buffer) + "." + useIDStr

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
			refreshTokenString = hex.EncodeToString(buffer) + "." + useIDStr

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
