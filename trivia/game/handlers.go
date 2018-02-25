package game

import (
	"fmt"
	"net/http"

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
}

func (h *handler) enterGame(w http.ResponseWriter, r *http.Request) {
	rawConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Error("error occurred while upgrading to ws conn: %s", err)
		return
	}

	conn := NewWSConn(rawConn)
	go conn.StartReadLoop()

	for {
		msg := conn.ReadMessageBlock()
		fmt.Printf("-- received: %T (%v)\n", msg, msg)
		if _, ok := msg.(*message.SocketClosed); ok {
			break
		}
	}
	fmt.Println("-- conn dropped.")
}

// NewHandler creates a new handler for the game endpoint/
func NewHandler() http.Handler {
	h := handler{}
	r := mux.NewRouter()
	r.HandleFunc("/v1/game/ws/{id}", h.enterGame).Methods("GET")
	return r
}
