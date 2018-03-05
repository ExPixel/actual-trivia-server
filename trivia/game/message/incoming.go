package message

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gorilla/websocket"
)

// ErrPayloadExpected is an error returned while reading a client message
// when a client message expecting a payload does not find one in the message.
var ErrPayloadExpected = errors.New("payload was expected but was nil")

// IncomingMessageType is the type of a tag for an incoming message.
type IncomingMessageType string

// incoming message tags:
const (
	// This identifies the client to the server and completes the "handshake".
	tagClientAuth = IncomingMessageType("client-auth")

	// This is a tag for an internal message that is sent when a socket is closed.
	tagSocketClose = IncomingMessageType("@socket-closed")

	tagSelectAnswer = IncomingMessageType("select-answer")
)

// ClientAuth is a message carrying the client auth token.
type ClientAuth struct {
	AuthToken string `json:"authToken"`
}

// SocketClosed is a message sent when a websocket has been closed either by the client or by the server.
type SocketClosed struct {
	connPtr *websocket.Conn
}

// CreateSocketClosed creates a new socket closed event for a web socket.
func CreateSocketClosed(sock *websocket.Conn) *SocketClosed {
	return &SocketClosed{connPtr: sock}
}

// IsSocketClosed returns true if the given websocket is the one that a socket closed
// message is in reference to.
func IsSocketClosed(msg *SocketClosed, sock *websocket.Conn) bool {
	return msg.connPtr == sock
}

// SelectAnswer is an incoming message sent when a user has selected an answer.
type SelectAnswer struct {
	// QuestionIndex is the index of the question that this answer is for.
	QuestionIndex int `json:"questionIndex"`
	Index         int `json:"index"`
}

// #NOTE should only define incoming messages in here
func unmarshalIncomingPayload(incoming *incomingJSONMessage) (msg interface{}, err error) {
	switch incoming.Tag {
	case tagSocketClose:
		msg = &SocketClosed{}
		unmarshalPayloadOptional(incoming.Payload, &msg)
	case tagClientAuth:
		msg = &ClientAuth{}
		unmarshalPayloadRequired(incoming.Payload, &msg)
	case tagSelectAnswer:
		msg = &SelectAnswer{}
		unmarshalPayloadRequired(incoming.Payload, &msg)
	default:
		return nil, fmt.Errorf("trivia: unknown incoming message tag '%s'", incoming.Tag)
	}
	return
}

func unmarshalPayloadOptional(payload *json.RawMessage, target *interface{}) error {
	if payload == nil {
		return nil
	}
	return json.Unmarshal(*payload, *target)
}

func unmarshalPayloadRequired(payload *json.RawMessage, target *interface{}) error {
	if payload == nil {
		return ErrPayloadExpected
	}
	return json.Unmarshal(*payload, *target)
}
