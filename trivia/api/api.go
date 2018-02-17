package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/expixel/actual-trivia-server/eplog"

	"github.com/expixel/actual-trivia-server/trivia"
)

var logger = eplog.NewPrefixLogger("api")
var errTokenWithNoUserOrGuest = errors.New("token has no valid user_id or guest_id")

type apiResponse struct {
	Code    int         `json:"code"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

type apiError struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Response writes a JSON response to the given response writer.
func Response(w http.ResponseWriter, data interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	encoder := json.NewEncoder(w)
	err := encoder.Encode(&apiResponse{
		Code:    code,
		Success: true,
		Data:    data,
	})

	if err != nil {
		log.Println("error occurred encoding JSON Response: ", err)
	}
}

// Error writes an message (as JSON) to the given http writer and sends the given response code to the client.
func Error(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	encoder := json.NewEncoder(w)
	err := encoder.Encode(&apiError{
		Code:    code,
		Success: false,
		Message: message,
	})

	if err != nil {
		log.Println("error occurred encoding JSON Response (err): ", err)
	}
}

// RequireJSONBody is a helper function for unmarshalling a JSON body if it is valid
// or returning the right errors to the client if it is not valid.
func RequireJSONBody(w http.ResponseWriter, r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(target)
	if err != nil {
		Error(w, "Body was not valid JSON or field types are not correct.", http.StatusBadRequest)
		return err
	}
	return nil
}

// GetUserForAuthToken returns a user for a token or returns nil and an error if the user was null
// or the token was expired. In the case of an expired token the error, ErrTokenExpired will be returned.
func GetUserForAuthToken(token string, ts trivia.AuthTokenService) (*trivia.User, error) {
	auth, user, err := ts.GetAuthTokenAndUser(token)
	if err != nil {
		return nil, err
	}
	if auth == nil {
		return nil, trivia.ErrTokenNotFound
	}
	if !auth.GuestID.Valid && user == nil {
		return nil, errTokenWithNoUserOrGuest
	}

	if user == nil {
		user = &trivia.User{
			ID:       0,
			Username: fmt.Sprintf("#Guest%d", auth.GuestID.Int64),
			Guest:    true,
			GuestID:  auth.GuestID,
		}
	}

	return user, nil
}

// GetRequestUser extracts a user from a request.
func GetRequestUser(r *http.Request, ts trivia.AuthTokenService) (*trivia.User, error) {
	authHeaders, ok := r.Header["Authorization"]
	if !ok || len(authHeaders) < 1 {
		return nil, trivia.ErrNoAuthInfo
	}
	authHeader := authHeaders[len(authHeaders)-1]

	fields := strings.Fields(authHeader)
	if len(fields) != 2 {
		return nil, trivia.ErrInvalidToken
	}

	tokenType := fields[0]
	if !strings.EqualFold(tokenType, "Bearer") {
		return nil, trivia.ErrInvalidToken
	}

	tokenString := fields[1]
	user, err := GetUserForAuthToken(tokenString, ts)
	return user, err
}

// RequireRequestUser authenticates a user and sends the proper error messages to the client
// if a user cannot be authenticated.
func RequireRequestUser(w http.ResponseWriter, r *http.Request, ts trivia.AuthTokenService) (*trivia.User, error) {
	user, err := GetRequestUser(r, ts)
	if err != nil {
		switch err {
		case trivia.ErrNoAuthInfo:
			Error(w, "Must provide an authentication token.", http.StatusUnauthorized)
		case trivia.ErrInvalidToken:
			Error(w, "Auth token format is not valid.", http.StatusBadRequest)
		case trivia.ErrTokenNotFound:
			Error(w, "Auth token does not exist or is expired.", http.StatusUnauthorized)
		default:
			logger.Error("error occurred while authenticating: %s", err)
			Error(w, "An unknown error occurred while authenticating your request.", http.StatusInternalServerError)
		}
	}
	return user, err
}
