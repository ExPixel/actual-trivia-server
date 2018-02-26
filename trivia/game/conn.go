package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/expixel/actual-trivia-server/eplog"

	"github.com/expixel/actual-trivia-server/trivia/game/message"
	"github.com/gorilla/websocket"
)

// Conn is a wrapper wround a websocket connection that reads and writes JSON messages.
type Conn struct {
	// wsConn is the underlying websocket connection
	wsConn *websocket.Conn

	// recvChan is a channel written to from the read loop
	// that contains messages received from the client.
	recvChan chan interface{}

	// recvBuffer is a dynamically sized buffer used for receiving
	// and deserializing messages.
	recvBuffer bytes.Buffer

	// stopped should only be accessed atomically. It is 1 if the read loop
	// should be stopped.
	stopped int32

	// writeLock should be acquired before writing any messages to wsConn
	writeLock *sync.Mutex

	// writeBuffer is a buffer used for encoding messages to JSON to send
	// them to the client.
	writeBuffer bytes.Buffer
}

// NewWSConn creates a new wrapped web socket connection.
func NewWSConn(conn *websocket.Conn) *Conn {
	return &Conn{
		wsConn:      conn,
		recvChan:    make(chan interface{}, 4),
		recvBuffer:  bytes.Buffer{},
		stopped:     0,
		writeLock:   &sync.Mutex{},
		writeBuffer: bytes.Buffer{},
	}
}

// StartReadLoop starts a loop for waiting for and reading client messages.
// This blocks until the connection is stopped or closed so it should be
// run on its own goroutine.
func (c *Conn) StartReadLoop() {
	// don't bother starting the loop if we're stopped and just resend the close message.

	if atomic.LoadInt32(&c.stopped) != 0 {
		// we send our own synthetic close message from the end of the read loop.
		c.recvChan <- &message.SocketClosed{}
	}

	eplog.Debug("websocket", "started ws reading loop") // #TODO remove test code

	for {
		messageType, r, err := c.wsConn.NextReader()

		// we check if this is stopped after NextReader is done
		// blocking so that we don't read a message after stopping (usually).
		if atomic.LoadInt32(&c.stopped) != 0 {
			break
		}

		if err != nil {
			// #FIXME right now I don't really have a way to communicate this error back
			// to whatever goroutine is currently writing or consuming the messages
			// so for now I just print the error out. Maybe I could create a special
			// error "json" message just for handling errors the way I do closes.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				eplog.Error("websocket", "unexpected error while reading from websocket: %s", err)
			}
			break
		}

		if messageType == websocket.TextMessage {
			c.recvBuffer.Reset()
			c.recvBuffer.ReadFrom(r)
			data := c.recvBuffer.Bytes()
			msg, err := message.DecodeMessage(data)
			if err != nil {
				// #TODO I should have a debug flag for printing invalid messages.
				// for now I just print all invalid messages to the error log.
				eplog.Error("websocket", "error while decoding websocket message: %s", err)
			}
			c.recvChan <- msg
		} else if messageType == websocket.CloseMessage {
			// #FIXME not sure if I need to be reading this message
			// as I already handle the close from the error step above.
			// something to consider for now.
		}
	}

	eplog.Debug("websocket", "stopped ws reading loop") // #TODO remove test code

	// we send our own synthetic close message from the end of the read loop.
	c.recvChan <- &message.SocketClosed{}
}

// WriteMessage writes a game message as json to the underlying websocket.
func (c *Conn) WriteMessage(msg interface{}) {
	c.writeLock.Lock()
	defer c.writeLock.Unlock()

	wrapped, err := message.WrapMessage(msg)
	if err != nil {
		panic(fmt.Sprintf("websocket: attempted to send unencodeable message through websocket: %s", err))
	}

	c.writeBuffer.Reset()
	encoder := json.NewEncoder(&c.writeBuffer)
	encoder.Encode(wrapped)
	err = c.wsConn.WriteMessage(websocket.TextMessage, c.writeBuffer.Bytes())

	if err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			eplog.Error("websocket", "unexpected error while writing to websocket: %s", err)
		}
		c.stop()
	}
}

// Close closes the websocket and stops the reading thread.
func (c *Conn) Close() {
	err := c.wsConn.Close()
	if err != nil {
		// #FIXME I'm not actually even sure what errors to epect here
		// but this seems right, so I'll take a look later.
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			eplog.Error("websocket", "unexpected error while closing websocket: %s", err)
		}
	}
	c.stop()
}

// ReadMessage reads a message from the websocket without blocking. If there is no message
// available it just returns immediately with nil.
func (c *Conn) ReadMessage() interface{} {
	select {
	case m := <-c.recvChan:
		return m
	default:
		return nil
	}
}

// ReadMessageBlock waits for a message from the client.
func (c *Conn) ReadMessageBlock() interface{} {
	select {
	case m := <-c.recvChan:
		return m
	}
}

// IsStopped returns true if the read loop for this connection is currently stopped.
func (c *Conn) IsStopped() bool {
	return atomic.LoadInt32(&c.stopped) != 0
}

// stop stops the websocket's read loop.
func (c *Conn) stop() {
	atomic.StoreInt32(&c.stopped, 1)
}
