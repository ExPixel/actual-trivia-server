package trivia

import (
	"errors"
	"time"
)

// User is a representation of a user profile.
type User struct {
	ID       int
	Username string
}

// UserCred is a representation of a user's login credentials.
type UserCred struct {
	UserID   int
	Email    string
	Password []byte
}

// AuthToken is a representation of an authentication used for signing and verifying requests to the API.
type AuthToken struct {
	Token     string
	UserID    int
	GuestID   int
	ExpiresAt time.Time
}

// RefreshToken is a representation of a token used for getting a new auth token after it has expired.
type RefreshToken struct {
	Token string

	// AuthToken is the auth token that this refresh token is for.
	AuthToken string

	UserID    int
	GuestID   int
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
	UserByID(id int) (*User, error)

	// UserByUsername finds a user using their username.
	UserByUsername(username string) (*User, error)

	// CredByEmail finds a user's credentials using an email address.
	CredByEmail(email string) (*UserCred, error)

	// CreateUser creates a user as well as their credentials.
	CreateUser(user *User, cred *UserCred) error

	// DeleteUser deletes a user from the data store by ID, and returns true if a user with the
	// given ID did exist and was deleted.
	DeleteUser(id int) (bool, error)
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
}

// An AuthService contains methods for authenticating users.
type AuthService interface {
	// AuthenticateByEmail attempts to authenticate a user by matching the email address and password
	// with a user in the data store. Returns the found and authenticated user with authentication
	// is successful. This may return one of the known errors: ErrUserNotFound, or ErrIncorrectPassword
	// which are recoverable.
	LoginWithEmail(email string, password string) (*TokenPair, error)

	// CreateUser creates a user and their credentials and adds them to the data store.
	CreateUser(username string, email string, password string) (*User, *UserCred, error)
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
