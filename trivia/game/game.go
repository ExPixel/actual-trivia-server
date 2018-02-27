package game

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/expixel/actual-trivia-server/trivia/game/message"

	"github.com/expixel/actual-trivia-server/eplog"

	"github.com/expixel/actual-trivia-server/trivia"
)

var logger = eplog.NewPrefixLogger("game")

// ErrGameNotFound is returned when trying to use a Game ID that does not exist.
var ErrGameNotFound = errors.New("no game with the given ID was found")

// TriviaGamesSet contains a set of trivia games that are currently running.
type TriviaGamesSet struct {
	// gameFinishedChan receives IDs of games that have been completed.
	gameFinishedChan chan string

	// gamesMapLock is a lock on the map of games that are currently running.
	gamesMapLock *sync.Mutex

	// games is a map of games that are currently running using their game IDs
	// as keys.
	games map[string]*TriviaGame

	tokenService trivia.AuthTokenService
}

// NewGameSet creates a new set of trivia games.
func NewGameSet(tokenService trivia.AuthTokenService) *TriviaGamesSet {
	return &TriviaGamesSet{
		gameFinishedChan: make(chan string, 16),
		gamesMapLock:     &sync.Mutex{},
		games:            make(map[string]*TriviaGame),
		tokenService:     tokenService,
	}
}

// GetGame gets a game from the game map.
// #CLEANUP might as well remove this (it used to do something else entirely.)
func (set *TriviaGamesSet) GetGame(gameID string) (game *TriviaGame, ok bool) {
	game, ok = set.games[gameID]
	return
}

// CreateGame creates a new game with the given ID and options.
func (set *TriviaGamesSet) CreateGame(gameID string, gameOptions *TriviaGameOptions) error {
	game := &TriviaGame{
		ID:                  gameID,
		gameFinishedChan:    set.gameFinishedChan,
		pendingClients:      make([]*Conn, 0),
		clients:             make([]*TriviaGameClient, 0),
		clientConnectedChan: make(chan *Conn, 16),
		stopGameChan:        make(chan bool, 1),
		MsgPendingCond:      &sync.Cond{L: &sync.Mutex{}},
		options:             gameOptions,
		gameWaitForIOChan:   make(chan bool, 1),
		gameWakeupCond:      &sync.Cond{L: &sync.Mutex{}},
		tokenService:        set.tokenService,
	}

	if _, ok := set.games[gameID]; ok {
		return fmt.Errorf("cannot create game, the ID %s is already in use", gameID)
	}

	set.games[gameID] = game
	game.Start()

	logger.Debug("created game with ID %s", gameID) // #TODO remove debug code.
	return nil
}

// State is used to represent the current state of the game.
type State int

// at the moment the game works as a state machine
// that is partly run on the IO loop goroutine and
// the game goroutine.
const (
	// gameStateWaitForStart: The game goroutine starts out locked and in this state and waits for
	// there to be enough players in a room to be woken up. (it may also wait for a leading player - if there is one - to start the game. )
	gameStateWaitForStart = State(iota)

	// #TODO I might need an intermediate state for the countdown to game state.

	// gameStateQuestion: There is a new question waiting and it should be broadcasted to users.
	// The IO loop will immediately switch to gameStateAnswers after it has finished
	// broadcasting the question to players.
	gameStateQuestion

	// gameStateAnswers: Waiting for players to input their answers to questions.
	// The game goroutine will wait a delay (specified in options) before changing
	// to the next state to being processing answers.
	gameStateAnswers

	// gameStateProcessing: The game goroutine goes through answers and marks them as correct
	// or incorrect during this state and sets point values.
	gameStateProcessing

	// gameStateReporting: The IO goroutine sends point values and reports to players whether they got
	// the previous question right or wrong during this state.
	gameStateReporting
)

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

	// clientConnectedChan is a channel that new connections should be sent to
	// so that they can be handled in the startConnectionLoop routine.
	clientConnectedChan chan *Conn

	// stopGameChan is a channel used for stopping the current game.
	stopGameChan chan bool

	// MsgPendingCond is a condition that will be signaled every time there is a message
	// waiting for this game to process.
	MsgPendingCond *sync.Cond

	options *TriviaGameOptions

	tokenService trivia.AuthTokenService

	// gameWaitForIOChan is used to signal the IO loop that the goroutine is waiting to be woken up.
	gameWaitForIOChan chan bool

	// gameWakupCond is a CondVar used to wakeup the game after it has started waiting for the IO loop
	// to complete a single tick.
	gameWakeupCond *sync.Cond

	// currentState is the current state of the game. This is usually being written
	// to by the game goroutine and waitForIO should be called immediately after a change
	// in order to ensure that the change is visible to the IO goroutine.
	currentState State

	// participationClosed is true if the game should no longer accept participation.
	participationClosed bool

	participantsCount int
	spectatorsCount   int
}

// TriviaGameOptions are a set of options for a single trivia game.
type TriviaGameOptions struct {
	// MinParticipants is the minimum number of participants required before
	// the game starts.
	MinParticipants int

	// MaxParticipants is the maximum number of participants allowed in the game.
	MaxParticipants int

	// GameStartDelay is the delay before the game starts after the minimum number of participants
	// has been reached.
	GameStartDelay time.Duration
}

// TriviaGameClient represents a user that is currently connected to the game.
type TriviaGameClient struct {
	// User is the user represented by this client.
	User *trivia.User

	// Conn is the connection being held by this client.
	Conn *Conn

	// Participant is true if this game client is an active participant of this game
	// and not just a spectator. Participants' scores are actually used when
	// the winner of the game is being calculated, and they are also the only players
	// displayed on screen by default throughout a game.
	Participant bool

	// CurrentQuestion is the index of the question that this client is currently
	// answering.
	CurrentQuestion int
}

// Start starts the trivia game.
func (g *TriviaGame) Start() {
	go g.startIOLoop()
	go g.startGame()
}

// Stop stops the game as well as the connection loop.
func (g *TriviaGame) Stop() {
	g.stopGameChan <- true
	g.MsgPendingCond.Signal()
}

// AddConn adds a new connection to the game.
func (g *TriviaGame) AddConn(conn *Conn) {
	g.clientConnectedChan <- conn
	g.MsgPendingCond.Signal()
}

// startIOLoop runs a loop that waits for new connections to the game.
func (g *TriviaGame) startIOLoop() {
	logger.Debug("game(%s) started connection loop", g.ID) // #TODO remove debug code
	stopGameChanClosed := false

connectionLoop:
	for {
		logger.Debug("connection loop tick (%d pending)", len(g.pendingClients))

		// true if the game goroutine is waiting to hear back about an IO loop tick completing.
		waitingForIOLoop := false

	pendingClientsLoop:
		for {
			select {
			case conn := <-g.clientConnectedChan:
				g.pendingClients = append(g.pendingClients, conn)
				logger.Debug("client %s added to pending clients", conn.wsConn.RemoteAddr()) // #TODO remove debug code
				conn.WriteMessage(&message.ClientInfoRequest{})
			case val, ok := <-g.stopGameChan:
				stopGameChanClosed = !ok
				if val || !ok {
					break connectionLoop
				}
			case waiting := <-g.gameWaitForIOChan:
				logger.Debug("I know you're waiting fam.")
				waitingForIOLoop = waiting
			default:
				break pendingClientsLoop
			}
		}

		g.handlePendingClients()
		g.ioGameTick()
		if waitingForIOLoop {
			g.gameWakeupCond.Signal() // let the game goroutine know that we're done.
		}

		// wait for some kind of message to come in.
		g.MsgPendingCond.L.Lock()
		g.MsgPendingCond.Wait()
		g.MsgPendingCond.L.Unlock()
	}

	if !stopGameChanClosed {
		close(g.stopGameChan)
	}

	logger.Debug("game(%s) stopped connection loop", g.ID) // #TODO remove debug code
}

// startGame starts the actual trivia game once enough players have connected.
func (g *TriviaGame) startGame() {
	logger.Debug("game(%s) started game routine", g.ID) // #TODO remove debug code

	g.gameWakeupCond.L.Lock()
	g.gameWakeupCond.Wait()
	g.gameWakeupCond.L.Unlock()

	logger.Debug("game(%s) game is no longer in the waiting state.")
	// #TODO set the current question first here. We'll handle actually fetching the questions later.
	g.currentState = gameStateQuestion
	g.waitForIO()

	logger.Debug("game(%s) ended game routine", g.ID) // #TODO remove debug code
}

func (g *TriviaGame) addGameClient(conn *Conn, user *trivia.User) {
	logger.Debug("adding user to game: %s", user.Username) // #TODO remove debug code
	client := &TriviaGameClient{
		User: user,
		Conn: conn,
	}

	// #TODO figure out whatever the fuck else goes into making someone a game participant or not.
	if !g.participationClosed && len(g.clients) < g.options.MaxParticipants {
		client.Participant = true
		g.participantsCount++
	} else {
		g.spectatorsCount++
	}

	g.clients = append(g.clients, client)
}

// waitForIO waits for one IO loop to pass before continuing.
func (g *TriviaGame) waitForIO() {
	g.gameWaitForIOChan <- true
	g.MsgPendingCond.Signal()

	g.gameWakeupCond.L.Lock()
	g.gameWakeupCond.Wait()
	g.gameWakeupCond.L.Unlock()
}

// ioGameTick executes a game tick for the IO loop.
func (g *TriviaGame) ioGameTick() {
	switch g.currentState {
	case gameStateWaitForStart:
		if g.participantsCount >= g.options.MinParticipants {
			logger.Debug("You have enough players now, fam.")
		}
	default:
		logger.Error("unknown game state: %d", g.currentState)
	}
}

// handlePendingClients handle ClientAuth messages from pending clients and remove them from the waiting list
func (g *TriviaGame) handlePendingClients() {
	for i := 0; i < len(g.pendingClients); i++ {
		c := g.pendingClients[i]
		if c.IsStopped() {
			// remove pending client (shifts the last pending client to i but that shouldn't be a problem)
			g.pendingClients[i] = g.pendingClients[len(g.pendingClients)-1]
			g.pendingClients = g.pendingClients[:len(g.pendingClients)-1]
			i--
		} else {
			if msg, ok := c.ReadMessage().(*message.ClientAuth); ok && msg != nil {
				authTokenString := msg.AuthToken
				_, user, err := g.tokenService.GetAuthTokenAndUser(authTokenString)
				if err != nil {
					logger.Error("error getting user auth: %s", err)
				} else if user == nil {
					c.WriteMessage(&message.UserNotFound{})
				} else {
					g.addGameClient(c, user)
				}

				// remove pending client (shifts the last pending client to i but that shouldn't be a problem)
				g.pendingClients[i] = g.pendingClients[len(g.pendingClients)-1]
				g.pendingClients = g.pendingClients[:len(g.pendingClients)-1]
				i--
			}
		}
	}
}
