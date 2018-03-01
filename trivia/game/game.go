package game

import (
	"bytes"
	"encoding/json"
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

var bmUserNotFound = message.MustEncodeBytes(&message.UserNotFound{})
var bmClientInfoRequest = message.MustEncodeBytes(&message.ClientInfoRequest{})

// TriviaGamesSet contains a set of trivia games that are currently running.
type TriviaGamesSet struct {
	// gameFinishedChan receives IDs of games that have been completed.
	gameFinishedChan chan string

	// gamesMapLock is a lock on the map of games that are currently running.
	gamesMapLock *sync.Mutex

	// games is a map of games that are currently running using their game IDs
	// as keys.
	games map[string]*TriviaGame

	tokenService    trivia.AuthTokenService
	questionService trivia.QuestionService
}

// NewGameSet creates a new set of trivia games.
func NewGameSet(tokenService trivia.AuthTokenService, questionService trivia.QuestionService) *TriviaGamesSet {
	return &TriviaGamesSet{
		gameFinishedChan: make(chan string, 16),
		gamesMapLock:     &sync.Mutex{},
		games:            make(map[string]*TriviaGame),
		tokenService:     tokenService,
		questionService:  questionService,
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
	msgPendingCond := &sync.Cond{L: &sync.Mutex{}}
	timerChan := make(chan bool, 1)

	game := &TriviaGame{
		ID:                  gameID,
		gameFinishedChan:    set.gameFinishedChan,
		pendingClients:      make([]*Conn, 0),
		clients:             make([]*TriviaGameClient, 0),
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
	gameStateWaitForStart = State(iota)
	gameStateFetchQuestions
	gameStateCountdownToStart
	gameStateQuestion
	gameStateQuestionCountdown
	gameStateProcessAnswers
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

	tokenService    trivia.AuthTokenService
	questionService trivia.QuestionService

	// participationClosed is true if the game should no longer accept participation.
	participationClosed bool

	participantsCount int
	spectatorsCount   int

	// currentState represents the current state of the game. A state of gameStateWaitingToStart
	currentState State

	// gameTickWaiting is true if the game loop should only run the next game tick
	// after the timer fires in the current iteration of the game loop.
	gameTickWaiting bool

	// gameTickTimer is the timer that is waited on before executing the next tick of the game.
	// this timer will wakeup the IO loop once it has completed.
	gameTickTimer *time.Timer

	// gameTickTimerChan receives true from the timer goroutine when the timer has completed.
	gameTickTimerChan chan bool

	// gameCountdownEnd is the time at which the game should end the countdown
	// and move on to the next state of the game.
	gameCountdownEnd time.Time

	// skipLoopPase if this is true the game will not wait on the condition variable for the
	// current iteration of the game loop.
	skipLoopPause bool

	// broadcastBuffer is a buffer used to encode messages before they are broadcasted to a client.
	broadcastBuffer bytes.Buffer

	currentQuestion int
	questions       []trivia.Question
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

	// QuestionsCount is the number of questions that will be presented during this trivia game.
	QuestionCount int

	// QuestionAnswerDuration is the amount of time that players get to answer each question.
	QuestionAnswerDuration time.Duration
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
	go g.startLoop()
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

// startLoop runs the game's loop which handles both IO and the actual game.
func (g *TriviaGame) startLoop() {
	logger.Debug("game(%s) started connection loop", g.ID) // #TODO remove debug code
	stopGameChanClosed := false

connectionLoop:
	for {
		// logger.Debug("connection loop tick (%d pending)", len(g.pendingClients))

		executeNextTick := !g.gameTickWaiting
	selectIOLoop:
		for {
			select {
			case conn := <-g.clientConnectedChan:
				g.pendingClients = append(g.pendingClients, conn)
				logger.Debug("client %s added to pending clients", conn.wsConn.RemoteAddr()) // #TODO remove debug code
				conn.WriteBytes(bmClientInfoRequest)
			case val, ok := <-g.stopGameChan:
				stopGameChanClosed = !ok
				if val || !ok {
					break connectionLoop
				}
			case v := <-g.gameTickTimerChan:
				if v && g.gameTickWaiting {
					executeNextTick = true
					g.gameTickWaiting = false
				}
			default:
				break selectIOLoop
			}
		}

		g.handlePendingClients()
		if executeNextTick {
			g.gameTick()
		}

		if g.skipLoopPause {
			g.skipLoopPause = false
		} else {
			// wait for some kind of message to come in.
			g.MsgPendingCond.L.Lock()
			g.MsgPendingCond.Wait()
			g.MsgPendingCond.L.Unlock()
		}
	}

	if !stopGameChanClosed {
		close(g.stopGameChan)
	}

	logger.Debug("game(%s) stopped connection loop", g.ID) // #TODO remove debug code
}

func (g *TriviaGame) addGameClient(conn *Conn, user *trivia.User) {
	logger.Debug("adding user to game: %s", user.Username) // #TODO remove debug code
	client := &TriviaGameClient{
		User:            user,
		Conn:            conn,
		CurrentQuestion: -1,
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

func (g *TriviaGame) gameTick() {
	// logger.Debug("game tick executed")
	switch g.currentState {
	case gameStateWaitForStart:
		logger.Debug("checking participants count: %d >= %d", g.participantsCount, g.options.MinParticipants)
		if g.participantsCount >= g.options.MinParticipants {
			g.gameCountdownEnd = time.Now().Add(g.options.GameStartDelay)
			g.currentState = gameStateFetchQuestions
			g.tickImm()
		}
	case gameStateFetchQuestions:
		var err error
		g.questions, err = g.questionService.GetRandomQuestions(g.options.QuestionCount)
		if err != nil {
			logger.Error("error occurred while fetching questions for game(%s): %s", g.ID, err)
			// #TODO I should end the game here.
		}

		g.broadcastMessage(&message.GameStartCountdownTick{
			Begin:           true,
			MillisRemaining: int(g.options.GameStartDelay.Nanoseconds() / int64(time.Millisecond)),
		})
		g.currentState = gameStateCountdownToStart
		g.tickImm()
	case gameStateCountdownToStart:
		now := time.Now()
		if now.After(g.gameCountdownEnd) {
			// #TODO set the game question here first.
			g.currentState = gameStateQuestion
			g.broadcastMessage(&message.GameStart{})
			g.tickImm()
		} else {
			var waitDur time.Duration
			untilEnd := g.gameCountdownEnd.Sub(now)
			if untilEnd < time.Second {
				waitDur = untilEnd
			} else {
				waitDur = time.Second
			}
			g.broadcastMessage(&message.GameStartCountdownTick{
				Begin:           true,
				MillisRemaining: int(untilEnd.Nanoseconds() / int64(time.Millisecond)),
			})
			g.tickWait(waitDur)
		}
	case gameStateQuestion:
		g.currentQuestion++
		if g.currentQuestion >= len(g.questions) {
			g.currentState = gameStateReporting
			g.tickImm()
			break
		}

		q := g.questions[g.currentQuestion]
		g.broadcastMessage(&message.SetPrompt{
			Prompt:     q.Prompt,
			Choices:    q.Choices,
			Category:   q.Category,
			Difficulty: "Unknown", // #TODO right now 0 = Unknown. Figure the rest out later.
			Index:      g.currentQuestion - 1,
		})
		logger.Debug("ask question: %s", q.Prompt)
		g.gameCountdownEnd = time.Now().Add(g.options.QuestionAnswerDuration)
		g.broadcastMessage(&message.QuestionCountdownTick{
			Begin:           true,
			MillisRemaining: int(g.options.QuestionAnswerDuration.Nanoseconds() / int64(time.Millisecond)),
		})
		g.currentState = gameStateQuestionCountdown
		g.tickImm()
	case gameStateQuestionCountdown:
		now := time.Now()
		if now.After(g.gameCountdownEnd) {
			g.currentState = gameStateProcessAnswers
			g.tickImm()
		} else {
			var waitDur time.Duration
			untilEnd := g.gameCountdownEnd.Sub(now)
			if untilEnd < time.Second {
				waitDur = untilEnd
			} else {
				waitDur = time.Second
			}
			g.broadcastMessage(&message.QuestionCountdownTick{
				Begin:           false,
				MillisRemaining: int(untilEnd.Nanoseconds() / int64(time.Millisecond)),
			})
			g.tickWait(waitDur)
		}
	case gameStateProcessAnswers:
		// #TODO mark answers as right or wrong for each game client and award points here
		// #NOTE maybe I should have a mode where users are penalized for making incorrect guesses instead of passing.
		// ^ might not be fun because that would require that everyone wait the full 10 seconds or whatever for a round.
		// ^ on a separate note I should end the round early if all users have answered the question
		g.currentState = gameStateQuestion
		g.tickWait(time.Millisecond) // I forget why I have a wait here, probably not important :|
	default:
		logger.Error("reached unexpected game state %d", g.currentState)
	}
}

// broadcastMessage sends a single message to all connected trivia game clients.
func (g *TriviaGame) broadcastMessage(msg interface{}) {
	wrapped, err := message.WrapMessage(msg)
	if err != nil {
		logger.Error("error wrapping broadcast message: %s", err.Error())
		return
	}

	g.broadcastBuffer.Reset()
	encoder := json.NewEncoder(&g.broadcastBuffer)
	err = encoder.Encode(wrapped)
	if err != nil {
		logger.Error("error encoding broadcast message: %s", err.Error())
		return
	}

	b := g.broadcastBuffer.Bytes()
	for _, c := range g.clients {
		c.Conn.WriteBytes(b)
	}
}

// tickImm causes the next tick of the game to be executed immediately.
// Use this when you don't want the game loop to pause after a tick.
func (g *TriviaGame) tickImm() {
	g.gameTickWaiting = false
	g.gameTickTimer.Stop()
	g.skipLoopPause = true
}

// tickWait sets the delay until the next game tick.
func (g *TriviaGame) tickWait(dur time.Duration) {
	if dur <= 0 {
		g.tickImm()
		return
	}

	g.gameTickWaiting = true
	g.gameTickTimer.Stop()
	g.gameTickTimer.Reset(dur)
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
					c.WriteBytes(bmUserNotFound)
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
