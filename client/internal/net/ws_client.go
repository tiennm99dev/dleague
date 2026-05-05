//go:build js && wasm

// Package net is the browser-side WebSocket client. It wraps the JS WebSocket
// API via syscall/js and exchanges binary protobuf Envelopes with the server.
package net

import (
	"errors"
	"fmt"
	"sync"
	"syscall/js"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// Handler is invoked for every inbound Envelope.
type Handler func(*dleaguev1.Envelope)

// Client is one browser WebSocket. js.Func handles are owned by Client and
// released on Close so reconnect cycles don't leak Go-side closures.
type Client struct {
	ws       js.Value
	onMsg    Handler
	openCh   chan struct{}
	openOnce sync.Once
	funcs    []js.Func
}

// Dial opens the browser WebSocket to url and returns a Client. The returned
// client is not yet open — callers should wait via WaitOpen before sending.
func Dial(url string, onMsg Handler) (*Client, error) {
	wsCtor := js.Global().Get("WebSocket")
	if !wsCtor.Truthy() {
		return nil, errors.New("WebSocket constructor unavailable")
	}
	c := &Client{
		ws:     wsCtor.New(url),
		onMsg:  onMsg,
		openCh: make(chan struct{}),
	}
	c.ws.Set("binaryType", "arraybuffer")
	c.bind()
	return c, nil
}

func (c *Client) bind() {
	c.listen("open", func(_ js.Value, _ []js.Value) any {
		c.openOnce.Do(func() { close(c.openCh) })
		return nil
	})
	c.listen("message", func(_ js.Value, args []js.Value) any {
		buf := args[0].Get("data")
		n := buf.Get("byteLength").Int()
		dst := make([]byte, n)
		js.CopyBytesToGo(dst, js.Global().Get("Uint8Array").New(buf))
		var env dleaguev1.Envelope
		if err := proto.Unmarshal(dst, &env); err != nil {
			fmt.Printf("ws unmarshal: %v\n", err)
			return nil
		}
		logRecv(&env)
		if c.onMsg != nil {
			c.onMsg(&env)
		}
		return nil
	})
	c.listen("error", func(js.Value, []js.Value) any {
		fmt.Println("ws error")
		return nil
	})
}

func (c *Client) listen(event string, fn func(js.Value, []js.Value) any) {
	f := js.FuncOf(fn)
	c.funcs = append(c.funcs, f)
	c.ws.Call("addEventListener", event, f)
}

// WaitOpen blocks until the WebSocket reaches OPEN state.
func (c *Client) WaitOpen() { <-c.openCh }

// Send marshals env as binary protobuf and writes it to the socket.
func (c *Client) Send(env *dleaguev1.Envelope) error {
	body, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	logSend(env)
	u8 := js.Global().Get("Uint8Array").New(len(body))
	js.CopyBytesToJS(u8, body)
	c.ws.Call("send", u8.Get("buffer"))
	return nil
}

// Close releases all js.Func handles and closes the underlying socket.
// Safe to call once; subsequent calls are no-ops.
func (c *Client) Close() {
	c.ws.Call("close")
	for _, f := range c.funcs {
		f.Release()
	}
	c.funcs = nil
}
