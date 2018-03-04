package game

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/expixel/actual-trivia-server/trivia"
	"github.com/gorilla/websocket"
)

// ErrGameNotFound is returned when trying to use a Game ID that does not exist.
var ErrGameNotFound = errors.New("no game with the given ID was found")

// TriviaGamesSet contains a set of trivia games that are currently running.
type TriviaGamesSet struct {
	// gamesMapLock is a lock on the map of games that are currently running.
	gamesMapLock *sync.Mutex

	// games is a map of games that are currently running using their game IDs
	// as keys.
	games     map[string]*TriviaGameSetGame
	gamesLock *sync.Mutex

	tokenService    trivia.AuthTokenService
	questionService trivia.QuestionService
}

// TriviaGameSetGame is a game that is in a set. It contains the actual game and then some extra
// information used by the trivia set.
type TriviaGameSetGame struct {
	// Game is that game that this is for.
	Game *TriviaGame

	// ParticipationClosed is set to true if the game is no longer
	// allowing participants.
	ParticipationClosed bool
}

// NewGameSet creates a new set of trivia games.
func NewGameSet(tokenService trivia.AuthTokenService, questionService trivia.QuestionService) *TriviaGamesSet {
	return &TriviaGamesSet{
		gamesMapLock:    &sync.Mutex{},
		games:           make(map[string]*TriviaGameSetGame),
		gamesLock:       &sync.Mutex{},
		tokenService:    tokenService,
		questionService: questionService,
	}
}

// AddRawConnToGame adds a raw connection to the requested game.
func (set *TriviaGamesSet) AddRawConnToGame(rawConn *websocket.Conn, gameID string) error {
	set.gamesLock.Lock()
	defer set.gamesLock.Unlock()

	var game *TriviaGame
	if gameID == "" {
		for _, setGame := range set.games {
			if !setGame.ParticipationClosed {
				game = setGame.Game
			}
		}
	} else {
		if setGame, ok := set.games[gameID]; ok {
			game = setGame.Game
		}
	}

	if game == nil {
		conn := NewWSConn(rawConn, nil)
		// we don't bother to start the read loop
		conn.WriteBytes(bmGameNotFound)
		conn.Close()
		return ErrGameNotFound
	}

	conn := NewWSConn(rawConn, game.MsgPendingCond)
	go conn.StartReadLoop()
	game.AddConn(conn)
	return nil
}

// WithSetGame runs a function with the set game for the given game ID.
func (set *TriviaGamesSet) WithSetGame(gameID string, fn func(setGame *TriviaGameSetGame)) {
	set.gamesLock.Lock()
	defer set.gamesLock.Unlock()

	if setGame, ok := set.games[gameID]; ok {
		fn(setGame)
	}
}

// CreateGame creates a new game with the given ID and options.
func (set *TriviaGamesSet) CreateGame(gameID string, gameOptions *TriviaGameOptions) error {
	msgPendingCond := &sync.Cond{L: &sync.Mutex{}}
	timerChan := make(chan bool, 1)

	game := &TriviaGame{
		ID:                  gameID,
		OwningSet:           set,
		pendingClients:      make([]*Conn, 0),
		clients:             make(map[int64]*TriviaGameClient),
		disconnectedClients: make(map[int64]*TriviaGameClient),
		clientConnectedChan: make(chan *Conn, 16),
		stopGameChan:        make(chan bool, 1),
		MsgPendingCond:      msgPendingCond,
		options:             gameOptions,
		tokenService:        set.tokenService,
		questionService:     set.questionService,
		gameTickTimerChan:   timerChan,
		broadcastBuffer:     bytes.Buffer{},
		currentQuestion:     -1,
		gameTickTimer: time.AfterFunc(0, func() {
			timerChan <- true
			msgPendingCond.Signal()
		}),
	}

	set.gamesLock.Lock()
	if _, ok := set.games[gameID]; ok {
		return fmt.Errorf("cannot create game, the ID %s is already in use", gameID)
	}
	set.games[gameID] = &TriviaGameSetGame{
		Game:                game,
		ParticipationClosed: false,
	}
	set.gamesLock.Unlock()

	game.Start()

	logger.Debug("created game with ID %s", gameID) // #TODO remove debug code.
	return nil
}
