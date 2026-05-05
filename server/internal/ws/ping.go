package ws

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

func handlePing(env *dleaguev1.Envelope, serverNowMS int64) (*dleaguev1.Envelope, error) {
	var ping dleaguev1.Ping
	if err := proto.Unmarshal(env.GetPayload(), &ping); err != nil {
		return nil, fmt.Errorf("ping unmarshal: %w", err)
	}
	pong := &dleaguev1.Pong{
		ClientUnixMs: ping.GetClientUnixMs(),
		ServerUnixMs: serverNowMS,
	}
	body, err := proto.Marshal(pong)
	if err != nil {
		return nil, fmt.Errorf("pong marshal: %w", err)
	}
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_PONG,
		RequestId: env.GetRequestId(),
		Payload:   body,
	}, nil
}
