package profile

import (
	"net/http"

	"github.com/expixel/actual-trivia-server/trivia"
	"github.com/expixel/actual-trivia-server/trivia/api"
	"github.com/gorilla/mux"
)

type handler struct {
	userService  trivia.UserService
	tokenService trivia.AuthTokenService
}

func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	currentUser, err := api.RequireRequestUser(w, r, h.tokenService)
	if err != nil {
		return
	}

	resp := userProfileResponse{
		ID:       currentUser.ID,
		Username: currentUser.Username,
		Guest:    currentUser.Guest,
		GuestID:  currentUser.GuestID,
	}
	api.Response(w, &resp, http.StatusOK)
}

// NewHandler creates a new handler for the profile service.
func NewHandler(us trivia.UserService, ts trivia.AuthTokenService) http.Handler {
	h := handler{userService: us, tokenService: ts}
	r := mux.NewRouter()
	r.HandleFunc("/v1/profile/me", h.me).Methods("GET")
	return r
}
