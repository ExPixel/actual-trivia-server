package game

import (
	"bytes"
	"encoding/json"
	"sync"
	"time"

	"github.com/expixel/actual-trivia-server/trivia/game/message"

	"github.com/expixel/actual-trivia-server/eplog"

	"github.com/expixel/actual-trivia-server/trivia"
)

// questionAnimationTime is the delay in between sending the question prompt to users
// and starting the question answer countdown. This time should be used for animating
// between trivia prompts.
const questionAnimationTime = time.Second * 2

// answerAnimationTime is the delay between revealing an answer, and moving on to the next question.
// This time should be used for animating the answer reveal and the participants' point totals.
const answerAnimationTime = time.Second*2 + time.Millisecond*500

// pingDelay is the delay used to pad transtitions between certain game
// states to account for the amount of time it takes messages to get to
// some users.
const pingDelay = time.Millisecond * 500

var logger = eplog.NewPrefixLogger("game")

var bmUserNotFound = message.MustEncodeBytes(&message.UserNotFound{})

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
	gameStateStartQuestionCountdown
	gameStateQuestionCountdown
	gameStateProcessAnswers
	gameStateReporting
)

// TriviaGame represents and coordinates a currently running game.
type TriviaGame struct {
	ID string

	// OwningSet is the trivia game set that owns this game.
	OwningSet *TriviaGamesSet

	// pendingClients are clients that the  server is waiting for authentication messages from.
	pendingClients []*Conn

	// clients are the clients that are currently connected to the game
	// and answering questions.
	clients map[int64]*TriviaGameClient

	// disconnectedClients contains clients that have had their websockets
	// closed and are awaiting reconnection.
	disconnectedClients map[int64]*TriviaGameClient

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

	// acceptingParticipants is true if the game is still in a state where participants
	// can be added to the game.
	acceptingParticipants     bool
	acceptingParticipantsLock *sync.Mutex
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

// url('/sample-path

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
	// answering. This is -1 if the client has not been posed a trivia prompt.
	CurrentQuestion int

	// SelectedAnswer is the index of the answer that the client selected.
	// This is -1 if the client has not selected an answer.
	SelectedAnswer int

	// Points is this client's user's current score.
	Points int

	// Closed is true if the websocket for this client is currently Closed.
	Closed bool
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

				// #TODO I can move this generic client info request inside of the game struct.
				conn.WriteBytes(message.MustEncodeBytes(&message.ClientInfoRequest{GameID: g.ID}))
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
		g.readClientMessages()

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
		SelectedAnswer:  -1,
	}

	// #TODO figure out whatever the fuck else goes into making someone a game participant or not.
	if !g.isParticipationClosed() {
		client.Participant = true
		g.participantsCount++
		g.updateSetParticipation()
	} else {
		g.spectatorsCount++
	}

	g.clients[user.ID] = client
}

func (g *TriviaGame) isParticipationClosed() bool {
	if g.currentState != gameStateWaitForStart &&
		g.currentState != gameStateFetchQuestions &&
		g.currentState != gameStateCountdownToStart {
		return true
	}

	if g.participantsCount >= g.options.MaxParticipants {
		return true
	}

	return false
}

func (g *TriviaGame) updateSetParticipation() {
	g.OwningSet.WithSetGame(g.ID, func(set *TriviaGameSetGame) {
		set.ParticipationClosed = g.isParticipationClosed()
	})
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
			g.updateSetParticipation()
			g.broadcastMessage(&message.GameStart{})
			g.tickWait(pingDelay)
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
		g.prepareClientsForQuestion()
		g.broadcastMessage(&message.SetPrompt{
			Prompt:     q.Prompt,
			Choices:    q.Choices,
			Category:   q.Category,
			Difficulty: "Unknown", // #TODO right now 0 = Unknown. Figure the rest out later.
			Index:      g.currentQuestion,
		})
		logger.Debug("ask question: %s", q.Prompt)
		g.currentState = gameStateStartQuestionCountdown
		g.tickWait(questionAnimationTime) // time allowance for question animation/extra reading time
	case gameStateStartQuestionCountdown:
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
			g.tickWait(pingDelay)
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
		// #TODO find a way to maybe end the round if all users (participants & spectators) have answered the question
		// ^ maybe I should only do that if there are no spectators in the game.
		if g.currentQuestion < len(g.questions) {
			q := g.questions[g.currentQuestion]
			g.broadcastMessage(&message.RevealAnswer{QuestionIndex: g.currentQuestion, AnswerIndex: q.CorrectChoice})
			g.processAnswers()
			// #TODO send information about the point totals of the game's participants.
			// ^ First I will have to send information about the participants of the game to begin with.
		}
		g.currentState = gameStateQuestion
		g.tickWait(answerAnimationTime) // I forget why I have a wait here, probably not important :|
	default:
		logger.Error("reached unexpected game state %d", g.currentState)
	}
}

// processAnswers awards points for correct answers to game clients.
func (g *TriviaGame) processAnswers() {
	q := g.questions[g.currentQuestion]
	for _, client := range g.clients {
		if client.CurrentQuestion == g.currentQuestion && client.SelectedAnswer == q.CorrectChoice {
			client.Points += 100
		}
	}
}

func (g *TriviaGame) readClientMessages() {
	for key, client := range g.clients {
		if client.Closed {
			delete(g.clients, key)
			g.disconnectedClients[key] = client
			g.afterClientDisconnected(client)
			continue
		}

	readSingleClientMessages:
		// for now we read at most 16 messages from a client
		// not sure how else I plan to stop a client from just launching a DoS attack
		// to stop other clients from sending messages.
		for climsg := 0; climsg < 16; climsg++ {
			msg := client.Conn.ReadMessage()
			if msg == nil {
				break readSingleClientMessages
			}

			switch msg := msg.(type) {
			case *message.SocketClosed:
				client.Closed = true
				client.Conn = nil
				logger.Debug("connection to user %s closed", client.User.Username)

				delete(g.clients, key)
				g.disconnectedClients[key] = client
				g.afterClientDisconnected(client)

				break readSingleClientMessages
			case *message.SelectAnswer:
				if msg.QuestionIndex == client.CurrentQuestion && msg.QuestionIndex == g.currentQuestion {
					if msg.Index >= 0 && client.SelectedAnswer < 0 {
						client.SelectedAnswer = msg.Index
					}
				}
			default:
				logger.Error("unhandled client message of type '%T'", msg)
			}
		}
	}
}

func (g *TriviaGame) afterClientDisconnected(client *TriviaGameClient) {
	if client.Participant {
		g.participantsCount--
		if g.participantsCount < 1 {
			// #TODO Here I should actually put the game into a gameStateTooFewClients
			// state or something and wait an amount of time for clients to disconnect
			// before actually just stopping the game.
			logger.Debug("too few players, resetting game")
			g.reset(true)
		}
	}
}

// prepareClientsForQuestion iterates through all of the connected game clients
// and prepares them for question answering events.
func (g *TriviaGame) prepareClientsForQuestion() {
	for _, client := range g.clients {
		if !client.Closed {
			client.CurrentQuestion = g.currentQuestion // so disconnected clients aren't penalized.
		}
		client.SelectedAnswer = -1 // reset the selected answer
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
		if !c.Closed {
			c.Conn.WriteBytes(b)
		}
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

// reset sets this game back to its starting state while maintaining its list of
// connected and pending clients.
func (g *TriviaGame) reset(removeClients bool) {
	g.currentState = gameStateWaitForStart
	g.questions = make([]trivia.Question, 0)
	g.currentQuestion = -1

	if removeClients {
		g.participantsCount = 0
		for _, client := range g.clients {
			if !client.Closed {
				client.Conn.Close()
			}
		}
		g.clients = make(map[int64]*TriviaGameClient)
		g.disconnectedClients = make(map[int64]*TriviaGameClient)
		g.participantsCount = 0
		g.spectatorsCount = 0
	}
	g.updateSetParticipation()

	g.MsgPendingCond.Signal()
	g.tickImm()
}

func isSameUser(a *trivia.User, b *trivia.User) bool {
	if a != nil && b != nil {
		if a.Guest && b.Guest {
			return (a.GuestID.Valid && b.GuestID.Valid) && (a.GuestID.Int64 == b.GuestID.Int64)
		}
		return a.ID == b.ID
	}
	return false
}

func (g *TriviaGame) restoreReconnectedClient(client *TriviaGameClient) {
	// #TODO in here I want to deliver all of the necessary state to
	// for a reconnected client to start playing the game right where they left
	// off.
}

// tryReconnectConn reassociates a connection and user with a trivia game client
// if there is one with the same user. It returns true if it was successful or false
// if no client with the same user was found.
func (g *TriviaGame) tryReconnectConn(conn *Conn, user *trivia.User) bool {
	if client, ok := g.clients[user.ID]; ok {
		// we just jump over to the new connection
		client.Conn.Close()
		client.Conn = conn
		g.restoreReconnectedClient(client)

		logger.Debug("reconnected user (connected): %s", client.User.Username)
		return true
	}

	if client, ok := g.disconnectedClients[user.ID]; ok {
		client.Conn = conn
		delete(g.disconnectedClients, user.ID)
		g.clients[user.ID] = client
		if client.Participant {
			g.participantsCount++
		}
		client.Closed = false
		g.restoreReconnectedClient(client)

		logger.Debug("reconnected user (disconnected): %s", client.User.Username)
		return true
	}

	return false
}

// handlePendingClients handle ClientAuth messages from pending clients and remove them from the waiting list
func (g *TriviaGame) handlePendingClients() {
	for i := 0; i < len(g.pendingClients); i++ {
		c := g.pendingClients[i]
		if c.IsStopped() { // #CLEANUP since I'm already checking for socket closed this might not be necessary.
			// remove pending client (shifts the last pending client to i but that shouldn't be a problem)
			g.pendingClients[i] = g.pendingClients[len(g.pendingClients)-1]
			g.pendingClients = g.pendingClients[:len(g.pendingClients)-1]
			i--
		} else {
			msg := c.ReadMessage()
			if msg == nil {
				continue
			}

			switch msg := msg.(type) {
			case *message.ClientAuth:
				authTokenString := msg.AuthToken
				_, user, err := g.tokenService.GetAuthTokenAndUser(authTokenString)
				if err != nil {
					logger.Error("error getting user auth: %s", err)
				} else if user == nil {
					c.WriteBytes(bmUserNotFound)
				} else {
					if !g.tryReconnectConn(c, user) {
						g.addGameClient(c, user)
					}
				}

				// remove pending client (shifts the last pending client to i but that shouldn't be a problem)
				g.pendingClients[i] = g.pendingClients[len(g.pendingClients)-1]
				g.pendingClients = g.pendingClients[:len(g.pendingClients)-1]
				i--
			case *message.SocketClosed:
				// remove pending client (shifts the last pending client to i but that shouldn't be a problem)
				g.pendingClients[i] = g.pendingClients[len(g.pendingClients)-1]
				g.pendingClients = g.pendingClients[:len(g.pendingClients)-1]
				i--
			}
		}
	}
}
