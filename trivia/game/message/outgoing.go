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
	tagClientInfoRequest = OutgoingMessageType("client-info-request")
)

// GameNotFound is an outgoing message used to signal to the client that it has provided an invalid game id.
type GameNotFound struct{}

// ClientInfoRequest is an outgoing message used to request that the client send it's authentication token and other information.
type ClientInfoRequest struct{}

// #NOTE should only define outgoing messages in here
func getTagForOutgoingPayload(payload interface{}) (OutgoingMessageType, error) {
	switch payload.(type) {
	case *GameNotFound:
		return tagGameNotFound, nil
	case *ClientInfoRequest:
		return tagClientInfoRequest, nil
	}
	return tagUnknown, errUnknownOutgoingTag
}
