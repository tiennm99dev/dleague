//go:build js && wasm && !debug

package net

import "google.golang.org/protobuf/proto"

func logRecv(proto.Message) {}
func logSend(proto.Message) {}
