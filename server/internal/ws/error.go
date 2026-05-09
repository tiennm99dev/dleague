package ws

import (
	"google.golang.org/protobuf/proto"

	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// errorEnvelope builds a MESSAGE_TYPE_ERROR Envelope with the given request ID,
// numeric code, and human-readable message. The Error message is marshaled into
// the Envelope payload. If proto.Marshal fails (which should never happen for a
// well-formed message) we return an envelope with an empty payload rather than
// surfacing a secondary error.
func errorEnvelope(reqID string, code int32, msg string) *dleaguev1.Envelope {
	body, _ := proto.Marshal(&dleaguev1.Error{Code: code, Message: msg})
	return &dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_ERROR,
		RequestId: reqID,
		Payload:   body,
	}
}
