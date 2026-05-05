//go:build js && wasm && debug

package net

import (
	"syscall/js"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var jsonMarshaller = protojson.MarshalOptions{Multiline: false}

func logRecv(m proto.Message) {
	if b, err := jsonMarshaller.Marshal(m); err == nil {
		js.Global().Get("console").Call("log", "ws recv", string(b))
	}
}

func logSend(m proto.Message) {
	if b, err := jsonMarshaller.Marshal(m); err == nil {
		js.Global().Get("console").Call("log", "ws send", string(b))
	}
}
