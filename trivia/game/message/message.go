package message

import (
	"encoding/json"
)

// JSONMessage is an outgoing JSON message with a type tag and a payload.
type JSONMessage struct {
	Tag     OutgoingMessageType `json:"tag"`
	Payload interface{}         `json:"payload"`
}

type incomingJSONMessage struct {
	Tag     IncomingMessageType `json:"tag"`
	Payload *json.RawMessage    `json:"payload"`
}

// WrapMessage wraps an outgoing game message into a JSONMessage with the correct tag.
func WrapMessage(payload interface{}) (JSONMessage, error) {
	tag, err := getTagForOutgoingPayload(payload)
	if err != nil {
		return JSONMessage{}, err
	}
	return JSONMessage{Tag: tag, Payload: payload}, nil
}

// DecodeMessage decodes some incoming bytes into JSON and then into a game message.
func DecodeMessage(incomingMessage []byte) (interface{}, error) {
	m := incomingJSONMessage{}
	err := json.Unmarshal(incomingMessage, &m)
	if err != nil {
		return nil, err
	}
	msg, err := unmarshalIncomingPayload(&m)
	return msg, err
}
