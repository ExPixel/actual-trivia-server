package auth

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/expixel/actual-trivia-server/trivia"
	"github.com/expixel/actual-trivia-server/trivia/api"
	"github.com/expixel/actual-trivia-server/trivia/auth/validate"
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

	type signupResponse struct {
		UserID int `json:"userID"`
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
		UserID: user.ID,
	}
	api.Response(w, &resp, http.StatusOK)
}

func (h *handler) login(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling auth login.")
	w.WriteHeader(200) // #TODO implement this shit
}

// NewHandler creates a new handler for requests to the authentication api.
func NewHandler(as trivia.AuthService) http.Handler {
	h := handler{authService: as}
	r := mux.NewRouter()
	r.HandleFunc("/v1/auth/signup", h.signup).Methods("POST")
	r.HandleFunc("/v1/auth/login", h.login).Methods("POST")
	return r
}
