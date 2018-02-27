package game

import (
	"net/http"
	"time"

	"github.com/expixel/actual-trivia-server/trivia"

	"github.com/expixel/actual-trivia-server/trivia/game/message"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// #FIXME I should have a same origin policy in here.
		// or at least not allow everything :P
		return true
	},
}

type handler struct {
	games *TriviaGamesSet
}

func (h *handler) enterGame(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]

	rawConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("error occurred while upgrading to ws conn: %s", err)
		return
	}

	game, ok := h.games.GetGame(gameID)
	if ok {
		conn := NewWSConn(rawConn, game.MsgPendingCond)
		go conn.StartReadLoop()
		game.AddConn(conn)
	} else {
		conn := NewWSConn(rawConn, nil)
		// we don't bother to start the read loop
		conn.WriteMessage(&message.GameNotFound{})
		conn.Close()
	}
}

// NewHandler creates a new handler for the game endpoint/
func NewHandler(tokenService trivia.AuthTokenService) http.Handler {
	h := handler{
		games: NewGameSet(tokenService),
	}

	// #TODO remove this test code once I have a way to create games from
	// the client.
	h.games.CreateGame("test", &TriviaGameOptions{
		MinParticipants: 1,
		MaxParticipants: 1,
		GameStartDelay:  5 * time.Second,
	})

	r := mux.NewRouter()
	r.HandleFunc("/v1/game/ws/{id}", h.enterGame).Methods("GET")
	return r
}
