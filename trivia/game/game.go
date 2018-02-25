package game

import (
	"sync"

	"github.com/expixel/actual-trivia-server/eplog"

	"github.com/expixel/actual-trivia-server/trivia"
)

var logger = eplog.NewPrefixLogger("game")

// TriviaGamesSet contains a set of trivia games that are currently running.
type TriviaGamesSet struct {
	// gameFinishedChan receives IDs of games that have been completed.
	gameFinishedChan chan string

	// gamesMapLock is a lock on the map of games that are currently running.
	gamesMapLock sync.Mutex

	// games is a map of games that are currently running using their game IDs
	// as keys.
	games map[string]*TriviaGame
}

// TriviaGame represents and coordinates a currently running game.
type TriviaGame struct {
	ID string

	// gameFinishedChan is a send only channel that this game's ID is written
	// to when it has completed and should be removed from the trivia games set
	// that is it a part of.
	gameFinishedChan chan<- string

	// pendingClients are clients that the  server is waiting for authentication messages from.
	pendingClients []*Conn

	// clients are the clients that are currently connected to the game
	// and answering questions.
	clients []*TriviaGameClient
}

// TriviaGameClient represents a user that is currently connected to the game.
type TriviaGameClient struct {
	// User is the user represented by this client.
	User trivia.User

	// Conn is the connection being held by this client.
	Conn *Conn

	// Participant is true if this game client is an active participant of this game
	// and not just a spectator. Participants' scores are actually used when
	// the winner of the game is being calculated, and they are also the only players
	// displayed on screen by default throughout a game.
	Participant bool
}
