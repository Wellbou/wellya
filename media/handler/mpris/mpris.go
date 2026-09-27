//go:build linux && !nomedia

package mpris

import (
	"sync"
	"time"

	"github.com/quarckster/go-mpris-server/pkg/events"
	"github.com/quarckster/go-mpris-server/pkg/server"
	"github.com/quarckster/go-mpris-server/pkg/types"
	"github.com/wellbou/wellya/media/handler"
)

type MprisHandler struct {
	server      *server.Server
	evHandler   *events.EventHandler
	name        string
	description string
	msgChan     chan handler.Message
	ansChan     chan any
	done        chan struct{}
	reqMu       sync.Mutex
}

// requestTimeout bounds how long a D-Bus call waits for the UI thread.
// Without it a query arriving while the UI is gone (quit) or busy blocks
// the D-Bus goroutine — and on quit, the main goroutine — forever.
const requestTimeout = 2 * time.Second

func NewHandler(name, description string) *MprisHandler {
	mh := &MprisHandler{
		name:        name,
		description: description,
		msgChan:     make(chan handler.Message),
		ansChan:     make(chan any, 1),
		done:        make(chan struct{}),
	}

	mh.server = server.NewServer(mh.name, mh, mh)
	mh.evHandler = events.NewEventHandler(mh.server)

	return mh
}

func (mh *MprisHandler) Start(handler func() error) error {
	go mh.server.Listen()

	err := handler()

	// UI is gone: unblock every pending/future D-Bus request first, then
	// tear the server down. msgChan/ansChan are intentionally left open —
	// closing them would panic late D-Bus callbacks that still send.
	close(mh.done)
	mh.server.Stop()

	return err
}

// post delivers a fire-and-forget command to the UI bridge.
func (mh *MprisHandler) post(msg handler.Message) {
	select {
	case mh.msgChan <- msg:
	case <-mh.done:
	case <-time.After(requestTimeout):
	}
}

// request sends a query and waits for its answer. Requests are serialised
// and stale answers (from a query that previously timed out) are dropped.
func (mh *MprisHandler) request(t handler.MessageType) (any, bool) {
	mh.reqMu.Lock()
	defer mh.reqMu.Unlock()

	select {
	case <-mh.ansChan:
	default:
	}

	timeout := time.NewTimer(requestTimeout)
	defer timeout.Stop()

	select {
	case mh.msgChan <- handler.Message{Type: t}:
	case <-mh.done:
		return nil, false
	case <-timeout.C:
		return nil, false
	}

	select {
	case ans := <-mh.ansChan:
		return ans, true
	case <-mh.done:
		return nil, false
	case <-timeout.C:
		return nil, false
	}
}

func (mh *MprisHandler) Message() <-chan handler.Message {
	return mh.msgChan
}

func (mh *MprisHandler) SendAnswer(ans any) {
	select {
	case mh.ansChan <- ans:
	default:
	}
}

func (mh *MprisHandler) OnEnded() {
	mh.evHandler.Player.OnEnded()
}

func (mh *MprisHandler) OnVolume() {
	mh.evHandler.Player.OnVolume()
}

func (mh *MprisHandler) OnPlayback() {
	mh.evHandler.Player.OnPlayback()
}

func (mh *MprisHandler) OnPlayPause() {
	mh.evHandler.Player.OnPlayPause()
}

func (mh *MprisHandler) OnSeek(position time.Duration) {
	mh.evHandler.Player.OnSeek(types.Microseconds(position.Microseconds()))
}
