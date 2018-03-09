package auth

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/expixel/actual-trivia-server/trivia"
	"github.com/expixel/actual-trivia-server/trivia/api"
	"github.com/expixel/actual-trivia-server/trivia/validate"
)

type handler struct {
	authService trivia.AuthService
}

func (h *handler) signup(w http.ResponseWriter, r *http.Request) {
	type signupBody struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	body := signupBody{}
	if err := api.RequireJSONBody(w, r, &body); err != nil {
		return
	}

	body.Username = strings.TrimSpace(body.Username)
	if len(body.Username) < 3 || len(body.Username) > 64 {
		api.Error(w, "Username must be from 3 to 64 characters long.", http.StatusBadRequest)
		return
	}
	if !validate.IsValidUsername(body.Username) {
		api.Error(w, "Username can only contain the characters a-z, A-Z, 0-9, <, >, -, _, and .", http.StatusBadRequest)
		return
	}

	if len(body.Password) < 6 || len(body.Password) > 256 {
		api.Error(w, "Password must be from 3 to 256 characters long.", http.StatusBadRequest)
		return
	}

	body.Email = strings.TrimSpace(body.Email)
	if !validate.IsEmail(body.Email) {
		api.Error(w, "A valid email address must be provided.", http.StatusBadRequest)
		return
	}

	user, _, err := h.authService.CreateUser(body.Username, body.Email, body.Password)
	if err != nil {
		switch err {
		case trivia.ErrEmailInUse:
			api.Error(w, "Email address is already in use.", http.StatusConflict)
		case trivia.ErrUsernameInUse:
			api.Error(w, "Username is already in use.", http.StatusConflict)
		default:
			logger.Error("error ocurred while creating user: ", err)
			api.Error(w, "Unknown error occurred while creating user.", http.StatusInternalServerError)
		}
		return
	}

	resp := signupResponse{
		UserID:   user.ID,
		Username: user.Username,
	}
	api.Response(w, &resp, http.StatusOK)
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	type loginBody struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	body := loginBody{}
	if err := api.RequireJSONBody(w, r, &body); err != nil {
		return
	}

	// #FIXME maybe I should check the length of the email
	// and password in here and make sure that they don't go over our limits.
	// for now this should be fine though.

	pair, err := h.authService.LoginWithEmailOrUsername(body.Username, body.Password)
	if err != nil {
		switch err {
		case trivia.ErrUserNotFound:
			api.Error(w, "No user with the given email/username and password.", http.StatusNotFound)
		case trivia.ErrIncorrectPassword:
			api.Error(w, "No user with the given email/username and password.", http.StatusNotFound)
		default:
			logger.Error("error ocurred while logging in with email and password: ", err)
			api.Error(w, "Unknown error occurred while logging in.", http.StatusInternalServerError)
		}
		return
	}

	resp := loginResponse{
		AuthToken:             pair.Auth.Token,
		AuthTokenExpiresAt:    pair.Auth.ExpiresAt.Unix(),
		RefreshToken:          pair.Refresh.Token,
		RefreshTokenExpiresAt: pair.Refresh.ExpiresAt.Unix(),
	}
	api.Response(w, &resp, http.StatusOK)
}

// guest is an endpoint used to option a guest identity to endter games
// without making an actual account.
func (h *handler) guest(w http.ResponseWriter, r *http.Request) {
	pair, err := h.authService.LoginAsGuest()
	if err != nil {
		logger.Error("error ocurred while generating guest tokens: ", err)
		api.Error(w, "Unknown error occurred while logging in.", http.StatusInternalServerError)
	}
	resp := loginResponse{
		AuthToken:             pair.Auth.Token,
		AuthTokenExpiresAt:    pair.Auth.ExpiresAt.Unix(),
		RefreshToken:          pair.Refresh.Token,
		RefreshTokenExpiresAt: pair.Refresh.ExpiresAt.Unix(),
	}
	api.Response(w, &resp, http.StatusOK)
}

// NewHandler creates a new handler for requests to the authentication api.
func NewHandler(as trivia.AuthService) http.Handler {
	h := handler{authService: as}
	r := mux.NewRouter()
	r.HandleFunc("/v1/auth/signup", h.signup).Methods("POST")
	r.HandleFunc("/v1/auth/login", h.login).Methods("POST")
	r.HandleFunc("/v1/auth/guest", h.guest).Methods("POST")
	return api.WrapAPIHandler(r)
}
