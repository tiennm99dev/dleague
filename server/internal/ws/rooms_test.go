package ws_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"nhooyr.io/websocket"

	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/store/memstore"
	"github.com/tiennm99/dleague/server/internal/ws"
	dleaguev1 "github.com/tiennm99/dleague/shared/pb/dleague/v1"
)

// uidVerifier returns AuthClaims with a UID derived from the token. Lets
// us spin two distinct connections in one test by using different tokens.
type uidVerifier struct{}

func (uidVerifier) Verify(_ context.Context, tok string) (store.AuthClaims, error) {
	return store.AuthClaims{UID: "uid-" + tok}, nil
}

func startSyncPvPServer(t *testing.T) (string, *memstore.Store) {
	t.Helper()
	mem := memstore.New()
	t.Cleanup(func() { _ = mem.Close() })

	hub := ws.NewHubWithStore(mem)
	srv := httptest.NewServer(ws.UpgradeHandler(hub, ws.UpgradeOptions{Verifier: uidVerifier{}}))
	t.Cleanup(srv.Close)
	return strings.Replace(srv.URL, "http://", "ws://", 1), mem
}

// dialAuth opens a WS conn and completes the AUTH handshake. Returns the
// authenticated conn.
func dialAuth(t *testing.T, ctx context.Context, wsURL, token string) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.Dial(ctx, wsURL+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })

	authPayload, _ := proto.Marshal(&dleaguev1.AuthRequest{IdToken: token})
	authEnv, _ := proto.Marshal(&dleaguev1.Envelope{
		Type:      dleaguev1.MessageType_MESSAGE_TYPE_AUTH_REQUEST,
		RequestId: "auth",
		Payload:   authPayload,
	})
	if err := c.Write(ctx, websocket.MessageBinary, authEnv); err != nil {
		t.Fatalf("write AUTH: %v", err)
	}
	// Drain AUTH_RESPONSE.
	if _, _, err := c.Read(ctx); err != nil {
		t.Fatalf("read AUTH_RESPONSE: %v", err)
	}
	return c
}

func sendEnvelope(t *testing.T, ctx context.Context, c *websocket.Conn, msgType dleaguev1.MessageType, requestID string, body proto.Message) {
	t.Helper()
	payload, err := proto.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	env, err := proto.Marshal(&dleaguev1.Envelope{Type: msgType, RequestId: requestID, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Write(ctx, websocket.MessageBinary, env); err != nil {
		t.Fatalf("write %v: %v", msgType, err)
	}
}

func readEnv(t *testing.T, ctx context.Context, c *websocket.Conn) *dleaguev1.Envelope {
	t.Helper()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var env dleaguev1.Envelope
	if err := proto.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	return &env
}

func TestJoinRoomTurnAndMatchEnd(t *testing.T) {
	wsURL, mem := startSyncPvPServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Pre-create the match doc with both UIDs (uidVerifier yields uid-<token>).
	if err := mem.UpsertMatch(ctx, store.Match{
		ID:         "m-42",
		Players:    []string{"uid-tok-a", "uid-tok-b"},
		Mode:       "sync",
		PuzzleDate: "2026-05-05",
		State:      "active",
		CreatedAt:  time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	cA := dialAuth(t, ctx, wsURL, "tok-a")
	cB := dialAuth(t, ctx, wsURL, "tok-b")

	// Both join.
	sendEnvelope(t, ctx, cA, dleaguev1.MessageType_MESSAGE_TYPE_JOIN_ROOM, "j-a", &dleaguev1.JoinRoom{MatchId: "m-42"})
	ackA := readEnv(t, ctx, cA)
	if ackA.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_JOIN_ROOM_ACK {
		t.Fatalf("type = %v", ackA.GetType())
	}
	var ackBodyA dleaguev1.JoinRoomAck
	_ = proto.Unmarshal(ackA.GetPayload(), &ackBodyA)
	if !ackBodyA.GetOk() || ackBodyA.GetRole() != "p1" || ackBodyA.GetOpponentUid() != "uid-tok-b" {
		t.Fatalf("ack A = %+v", &ackBodyA)
	}

	sendEnvelope(t, ctx, cB, dleaguev1.MessageType_MESSAGE_TYPE_JOIN_ROOM, "j-b", &dleaguev1.JoinRoom{MatchId: "m-42"})
	ackB := readEnv(t, ctx, cB)
	var ackBodyB dleaguev1.JoinRoomAck
	_ = proto.Unmarshal(ackB.GetPayload(), &ackBodyB)
	if !ackBodyB.GetOk() || ackBodyB.GetRole() != "p2" {
		t.Fatalf("ack B = %+v", &ackBodyB)
	}

	// A sends TURN; B receives forwarded frame.
	sendEnvelope(t, ctx, cA, dleaguev1.MessageType_MESSAGE_TYPE_TURN, "t1", &dleaguev1.Turn{
		MatchId: "m-42", TurnIndex: 1, Payload: []byte("guess1"),
	})
	gotByB := readEnv(t, ctx, cB)
	if gotByB.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_TURN {
		t.Fatalf("B received type = %v", gotByB.GetType())
	}
	var turn dleaguev1.Turn
	_ = proto.Unmarshal(gotByB.GetPayload(), &turn)
	if string(turn.GetPayload()) != "guess1" {
		t.Fatalf("turn payload = %q", turn.GetPayload())
	}

	// A submits MATCH_END; both A (echo via dispatch return) and B (broadcast)
	// should observe it; store should reflect ended state + leaderboards.
	sendEnvelope(t, ctx, cA, dleaguev1.MessageType_MESSAGE_TYPE_MATCH_END, "e1", &dleaguev1.MatchEnd{
		MatchId: "m-42", WinnerUid: "uid-tok-a", ScoreP1: 80, ScoreP2: 30,
	})
	ackEnd := readEnv(t, ctx, cA)
	if ackEnd.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_MATCH_END {
		t.Fatalf("end ack type = %v", ackEnd.GetType())
	}
	endByB := readEnv(t, ctx, cB)
	if endByB.GetType() != dleaguev1.MessageType_MESSAGE_TYPE_MATCH_END {
		t.Fatalf("B end type = %v", endByB.GetType())
	}

	// Match doc updated.
	finalMatch, err := mem.GetMatch(ctx, "m-42")
	if err != nil {
		t.Fatalf("GetMatch: %v", err)
	}
	if finalMatch.State != "ended" || finalMatch.Winner != "uid-tok-a" {
		t.Errorf("match final = %+v", finalMatch)
	}

	// Both leaderboards reflect both players.
	for _, board := range []string{"lb:daily:2026-05-05", "lb:global:alltime"} {
		top, err := mem.TopN(ctx, board, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(top) != 2 || top[0].UID != "uid-tok-a" || top[0].Score != 80 || top[1].Score != 30 {
			t.Errorf("board %s = %+v", board, top)
		}
	}
}

func TestJoinRoomForbiddenForNonPlayer(t *testing.T) {
	wsURL, mem := startSyncPvPServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mem.UpsertMatch(ctx, store.Match{
		ID: "m-50", Players: []string{"uid-alice", "uid-bob"}, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	c := dialAuth(t, ctx, wsURL, "intruder") // → uid-intruder
	sendEnvelope(t, ctx, c, dleaguev1.MessageType_MESSAGE_TYPE_JOIN_ROOM, "j", &dleaguev1.JoinRoom{MatchId: "m-50"})
	ack := readEnv(t, ctx, c)
	var body dleaguev1.JoinRoomAck
	_ = proto.Unmarshal(ack.GetPayload(), &body)
	if body.GetOk() {
		t.Fatalf("expected ok=false, got %+v", &body)
	}
	if body.GetError() == "" {
		t.Errorf("expected error message")
	}
}

func TestJoinRoomMatchNotFound(t *testing.T) {
	wsURL, _ := startSyncPvPServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := dialAuth(t, ctx, wsURL, "tok-a")
	sendEnvelope(t, ctx, c, dleaguev1.MessageType_MESSAGE_TYPE_JOIN_ROOM, "j", &dleaguev1.JoinRoom{MatchId: "nope"})
	ack := readEnv(t, ctx, c)
	var body dleaguev1.JoinRoomAck
	_ = proto.Unmarshal(ack.GetPayload(), &body)
	if body.GetOk() {
		t.Fatalf("expected ok=false, got %+v", &body)
	}
}
