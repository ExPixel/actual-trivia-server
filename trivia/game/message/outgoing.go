package message

// OutgoingMessageType is the type of a tag for an outgoing message.
import "errors"

// OutgoingMessageType type for a message that can be sent to the client.
type OutgoingMessageType string

var errUnknownOutgoingTag = errors.New("trivia: unknown outgoing message tag")

// outgoing message tags:
const (
	tagUnknown           = OutgoingMessageType("o-unknown")
	tagGameNotFound      = OutgoingMessageType("game-not-found")
	tagUserNotFound      = OutgoingMessageType("user-not-found")
	tagClientInfoRequest = OutgoingMessageType("client-info-request")

	tagGameStartCountdownTick = OutgoingMessageType("g-start-countdown-tick")
	tagGameStart              = OutgoingMessageType("g-start")
)

// GameNotFound is an outgoing message used to signal to the client that it has provided an invalid game id.
type GameNotFound struct{}

// ClientInfoRequest is an outgoing message used to request that the client send it's authentication token and other information.
type ClientInfoRequest struct{}

// UserNotFound is an outgoing message sent when a user cannot be authenticated with a ClientAuthInfo
type UserNotFound struct{}

// GameStartCountdownTick is an outgoing message used to tell the client the number of seconds remaining
// until a game begins.
type GameStartCountdownTick struct {
	// Begin is true to mark the start of the countdown.
	Begin bool `json:"begin"`

	// SecondsRemaining is the number of seconds until the game will begin.
	SecondsRemaining int `json:"secondsRemaining"`
}

// GameStart is an outgoing message to let the client know that the game has started and that
// questions are going to start being delivered.
type GameStart struct{}

// #NOTE should only define outgoing messages in here
func getTagForOutgoingPayload(payload interface{}) (OutgoingMessageType, error) {
	switch payload.(type) {
	case *GameNotFound:
		return tagGameNotFound, nil
	case *ClientInfoRequest:
		return tagClientInfoRequest, nil
	case *UserNotFound:
		return tagUserNotFound, nil
	case *GameStartCountdownTick:
		return tagGameStartCountdownTick, nil
	case *GameStart:
		return tagGameStart, nil
	}
	return tagUnknown, errUnknownOutgoingTag
}
