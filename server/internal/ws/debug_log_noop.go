//go:build !debug

package ws

import "google.golang.org/protobuf/proto"

func logRecv(proto.Message) {}
func logSend(proto.Message) {}
