//go:build debug

package ws

import (
	"log"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var jsonMarshaller = protojson.MarshalOptions{Multiline: false}

func logRecv(m proto.Message) {
	if b, err := jsonMarshaller.Marshal(m); err == nil {
		log.Printf("ws recv %s", b)
	}
}

func logSend(m proto.Message) {
	if b, err := jsonMarshaller.Marshal(m); err == nil {
		log.Printf("ws send %s", b)
	}
}
