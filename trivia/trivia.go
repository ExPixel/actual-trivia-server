package trivia

import (
	"errors"
	"fmt"
	"time"

	"github.com/expixel/actual-trivia-server/trivia/null"
)

// User is a representation of a user profile.
type User struct {
	// ID is negative if the user is a guest. This just makes ensuring
	// uniqueness of clients connecting to the game much easier.
	ID       int64
	Username string

	// these properties don't get saved to the DB:

	// Guest is a flag that is set during authentication and denotes this particular
	// user as a guest. Guest users all have a UserID of 0. A GuestID should be used
	// for them instead for comparisons.
	Guest bool

	// GuestID is an identifier used for guest users.
	GuestID null.Int64
}

// UserCred is a representation of a user's login credentials.
type UserCred struct {
	UserID   int64
	Email    string
	Password []byte
}

// Question is a representation of a single trivia question.
type Question struct {
	ID            int64
	Category      string
	Difficulty    int
	Prompt        string
	Choices       []string
	CorrectChoice int
	Source        string
}

// AuthToken is a representation of an authentication used for signing and verifying requests to the API.
type AuthToken struct {
	Token     string
	UserID    null.Int64
	GuestID   null.Int64
	ExpiresAt time.Time
}

// RefreshToken is a representation of a token used for getting a new auth token after it has expired.
type RefreshToken struct {
	Token string

	// AuthToken is the auth token that this refresh token is for.
	AuthToken string

	UserID    null.Int64
	GuestID   null.Int64
	ExpiresAt time.Time
}

// TokenPair is a pair of auth and refresh tokens
type TokenPair struct {
	Auth    *AuthToken
	Refresh *RefreshToken
}

// A UserService contains methods for finding, creating, and modifying users.
type UserService interface {
	// UserById finds a user using their ID.
	UserByID(id int64) (*User, error)

	// UserByUsername finds a user using their username.
	UserByUsername(username string) (*User, error)

	// CredByEmail finds a user's credentials using an email address.
	CredByEmail(email string) (*UserCred, error)

	// CredByUsername finds a user's credentials using a username.
	CredByUsername(username string) (*UserCred, error)

	// CreateUser creates a user as well as their credentials.
	CreateUser(user *User, cred *UserCred) error

	// DeleteUser deletes a user from the data store by ID, and returns true if a user with the
	// given ID did exist and was deleted.
	DeleteUser(id int64) (bool, error)

	// NextGuestID generates an ID that should be used by the next guest account.
	NextGuestID() (int64, error)
}

// An AuthTokenService contains methods for creating and retrieving authentication and refresh tokens.
type AuthTokenService interface {
	// AuthTokenByString finds an authentication token using the token string.
	AuthTokenByString(token string) (*AuthToken, error)

	// CreateTokenPair inserts both an auth token and refresh token into the database.
	CreateTokenPair(auth *AuthToken, refresh *RefreshToken) error

	// AuthTokenExists returns true if a the given token already exists in the database.
	AuthTokenExists(token string) (bool, error)

	// RefreshTokenExists returns true if the given token already exists in the database.
	RefreshTokenExists(token string) (bool, error)

	// GetAuthTokenAndUser gets an auth token as well as the associated user using the
	// token string. This will return a null user if this is a token for a guest.
	GetAuthTokenAndUser(token string) (*AuthToken, *User, error)
}

// An AuthService contains methods for authenticating users.
type AuthService interface {
	// LoginWithEmailOrUsername attempts to authenticate a user by matching the email address or username  and password
	// with a user in the data store. Returns the found and authenticated user with authentication
	// is successful. This may return one of the known errors: ErrUserNotFound, or ErrIncorrectPassword
	// which are recoverable.
	LoginWithEmailOrUsername(emailOrUsername string, password string) (*TokenPair, error)

	// CreateUser creates a user and their credentials and adds them to the data store.
	CreateUser(username string, email string, password string) (*User, *UserCred, error)

	// LoginAsGuest creates a pair of tokens for a guest account.
	LoginAsGuest() (*TokenPair, error)
}

// A QuestionService contains methods for fetching and interacting with questions.
type QuestionService interface {
	GetRandomQuestions(count int) ([]Question, error)
}

// A GameService is a service responsible for coordinating running games,
// creating new games, and connecting users to those games.
type GameService interface {
	// #TODO figure out what the game service is going to look like.
}

// ErrUsernameInUse is an error returned by an authentication service when trying to create a
// user with a username that is already in use.
var ErrUsernameInUse = errors.New("username is already in use")

// ErrEmailInUse is an error retruned by an authentication service when trying to create a
// user with an email address that is already in use.
var ErrEmailInUse = errors.New("email is already in use")

// ErrUserNotFound is an error returned by an authentication service when trying to login a
// user by email. The specifics of this error should not be made public to the user attempting to
// login.
var ErrUserNotFound = errors.New("user not found for credentials")

// ErrIncorrectPassword is an error returned by an authentication service when trying to login
/// a user by email. The specifics of this error should not be made public to the user attempting to
// login.
var ErrIncorrectPassword = errors.New("password provided does not match user password")

// ErrTokenExpired is an error retruned when a method that required a vlaid auth or refresh token
// finds that a given token is no longer valid.
var ErrTokenExpired = errors.New("token is expired")

// ErrTokenNotFound is an error returned when an auth or refresh token cannot be found in the database.
var ErrTokenNotFound = errors.New("token was not found")

// ErrInvalidToken is an error returned when a provided auth token has an invalid format.
var ErrInvalidToken = errors.New("malformed token")

// ErrNoAuthInfo is returned when a function searching for an authorization header, cookie, ect. cannot find
// one in a given request.
var ErrNoAuthInfo = errors.New("no authentication information found")

// NewGuestUser creates a new guest user given a gest ID.
func NewGuestUser(guestID null.Int64) *User {
	if !guestID.Valid {
		panic("cannot create guest user with null guest ID.")
	}

	return &User{
		ID:       -1 * guestID.Int64,
		Username: fmt.Sprintf("#Guest%d", guestID.Int64),
		Guest:    true,
		GuestID:  guestID,
	}
}
