package profile

import (
	"github.com/expixel/actual-trivia-server/trivia/null"
)

type userProfileResponse struct {
	ID       int64      `json:"id"`
	Username string     `json:"username"`
	Guest    bool       `json:"guest"`
	GuestID  null.Int64 `json:"guestId"`
}
