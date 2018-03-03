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

	tagQuestionCountdownTick = OutgoingMessageType("q-countdown-tick")
	tagSetPrompt             = OutgoingMessageType("q-set-prompt")
	tagRevealAnswer          = OutgoingMessageType("q-reveal-answer")
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

	// MillisRemaining is the number of seconds remaining before the game starts.
	MillisRemaining int `json:"millisRemaining"`
}

// GameStart is an outgoing message to let the client know that the game has started and that
// questions are going to start being delivered.
type GameStart struct{}

// SetPrompt is an outgoing message that sets the current prompt and choices for the clients.
type SetPrompt struct {
	// Index is  the index of this question in the question set for the current trivia game.
	Index int `json:"index"`

	Prompt     string   `json:"prompt"`
	Choices    []string `json:"choices"`
	Category   string   `json:"category"`
	Difficulty string   `json:"Difficulty"`
}

// QuestionCountdownTick is an outgoing message used to tell the clients the number of seconds
// remaining to answer the current question.
type QuestionCountdownTick struct {
	// Begin is true if this is the start of the countdown.
	Begin bool `json:"begin"`

	// MillisRemaining is the number of seconds the client has to answer the questions.
	MillisRemaining int `json:"millisRemaining"`
}

// RevealAnswer is an outgoing message that reveals the answer to a question to a client.
type RevealAnswer struct {
	QuestionIndex int `json:"questionIndex"`
	AnswerIndex   int `json:"answerIndex"`
}

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
	case *SetPrompt:
		return tagSetPrompt, nil
	case *QuestionCountdownTick:
		return tagQuestionCountdownTick, nil
	case *RevealAnswer:
		return tagRevealAnswer, nil
	}
	return tagUnknown, errUnknownOutgoingTag
}
