//go:build js && wasm

// Pattern adapted from hajimehoshi/ebiten/examples/2048/main.go (Apache-2.0).
// See NOTICE for the full attribution.

package main

import (
	"log"
	"syscall/js"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"google.golang.org/protobuf/proto"

	clientnet "github.com/tiennm99/dleague/client/internal/net"
	"github.com/tiennm99/dleague/client/internal/scene"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

const (
	screenWidth  = 480
	screenHeight = 640
	windowTitle  = "Dleague"
)

type app struct {
	title *scene.Title
}

func (a *app) Update() error             { return a.title.Update() }
func (a *app) Draw(screen *ebiten.Image) { a.title.Draw(screen) }
func (a *app) Layout(_, _ int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	go connectAndPing()

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle(windowTitle)
	if err := ebiten.RunGame(&app{title: scene.NewTitle()}); err != nil {
		log.Fatal(err)
	}
}

// connectAndPing opens a WebSocket to the same host that served the page and
// exchanges one Ping/Pong roundtrip. Used at Phase 1 to verify the wire format
// end-to-end. Real game code in later phases will own the connection lifecycle.
func connectAndPing() {
	loc := js.Global().Get("location")
	scheme := "ws"
	if loc.Get("protocol").String() == "https:" {
		scheme = "wss"
	}
	url := scheme + "://" + loc.Get("host").String() + "/ws"

	c, err := clientnet.Dial(url, func(env *dleaguev1.Envelope) {
		log.Printf("recv type=%v request_id=%s", env.GetType(), env.GetRequestId())
	})
	if err != nil {
		log.Printf("dial: %v", err)
		return
	}
	c.WaitOpen()

	ping := &dleaguev1.Ping{ClientUnixMs: time.Now().UnixMilli()}
	body, err := proto.Marshal(ping)
	if err != nil {
		log.Printf("marshal ping: %v", err)
		return
	}
	if err := c.Send(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PING,
		RequestId: "phase1-bootstrap",
		Payload:   body,
	}); err != nil {
		log.Printf("send: %v", err)
	}
}
