package message

import (
	"encoding/json"
	"errors"
)

// ErrPayloadExpected is an error returned while reading a client message
// when a client message expecting a payload does not find one in the message.
var ErrPayloadExpected = errors.New("payload was expected but was nil")

// IncomingMessageType is the type of a tag for an incoming message.
type IncomingMessageType string

var errUnknownIncomingTag = errors.New("trivia: unknown incoming message tag")

// incoming message tags:
const (
	// This identifies the client to the server and completes the "handshake".
	tagClientAuth = IncomingMessageType("client-auth")

	// This is a tag for an internal message that is sent when a socket is closed.
	tagSocketClose = IncomingMessageType("@socket-closed")
)

// ClientAuth is a message carrying the client auth token.
type ClientAuth struct {
	AuthToken string `json:"authToken"`
}

// SocketClosed is a message sent when a websocket has been closed either by the client or by the server.
type SocketClosed struct{}

// #NOTE should only define incoming messages in here
func unmarshalIncomingPayload(incoming *incomingJSONMessage) (msg interface{}, err error) {
	switch incoming.Tag {
	case tagSocketClose:
		msg = &SocketClosed{}
		unmarshalPayloadOptional(incoming.Payload, &msg)
	case tagClientAuth:
		msg = &ClientAuth{}
		unmarshalPayloadRequired(incoming.Payload, &msg)
	default:
		return nil, errUnknownIncomingTag
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
