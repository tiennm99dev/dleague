// Protobuf wire helpers. Keeps a single Root parsed from the inline .proto
// schema below — bytes-on-the-wire identical to what the Go server
// (`shared/pb/dleague/v1`) speaks. When the server proto changes, mirror
// the relevant message fields here.
//
// Using protobufjs runtime parse keeps the build chain free of a separate
// pbjs codegen step; cost is one parse on first import (~ms).

import { parse, Type, Root, Reader, Writer } from 'protobufjs';

// Mirrors proto/dleague/v1/envelope.proto. Add new messages here when the
// server adds them.
const SCHEMA = `
syntax = "proto3";
package dleague.v1;

enum MessageType {
  MESSAGE_TYPE_UNSPECIFIED = 0;
  MESSAGE_TYPE_PING = 1;
  MESSAGE_TYPE_PONG = 2;
  MESSAGE_TYPE_AUTH_REQUEST = 3;
  MESSAGE_TYPE_AUTH_RESPONSE = 4;
  MESSAGE_TYPE_JOIN_ROOM = 5;
  MESSAGE_TYPE_JOIN_ROOM_ACK = 6;
  MESSAGE_TYPE_TURN = 7;
  MESSAGE_TYPE_MATCH_END = 8;
}

message Envelope {
  MessageType type = 1;
  string request_id = 2;
  bytes payload = 3;
}

message Ping {
  int64 client_unix_ms = 1;
}

message Pong {
  int64 client_unix_ms = 1;
  int64 server_unix_ms = 2;
}

message AuthRequest {
  string id_token = 1;
  uint32 version = 2;
}

message AuthResponse {
  bool ok = 1;
  string uid = 2;
  string error = 3;
}

message JoinRoom {
  string match_id = 1;
}

message JoinRoomAck {
  bool ok = 1;
  string match_id = 2;
  string role = 3;
  string opponent_uid = 4;
  string error = 5;
}

message Turn {
  string match_id = 1;
  uint32 turn_index = 2;
  bytes payload = 3;
}

message MatchEnd {
  string match_id = 1;
  string winner_uid = 2;
  int64 score_p1 = 3;
  int64 score_p2 = 4;
}
`;

const root: Root = parse(SCHEMA, { keepCase: false }).root;

export const MessageType = {
  UNSPECIFIED: 0,
  PING: 1,
  PONG: 2,
  AUTH_REQUEST: 3,
  AUTH_RESPONSE: 4,
  JOIN_ROOM: 5,
  JOIN_ROOM_ACK: 6,
  TURN: 7,
  MATCH_END: 8
} as const;

export type MessageTypeValue = (typeof MessageType)[keyof typeof MessageType];

const Envelope = root.lookupType('dleague.v1.Envelope');
const AuthRequest = root.lookupType('dleague.v1.AuthRequest');
const AuthResponse = root.lookupType('dleague.v1.AuthResponse');
const Ping = root.lookupType('dleague.v1.Ping');
const Pong = root.lookupType('dleague.v1.Pong');
const JoinRoom = root.lookupType('dleague.v1.JoinRoom');
const JoinRoomAck = root.lookupType('dleague.v1.JoinRoomAck');
const Turn = root.lookupType('dleague.v1.Turn');
const MatchEnd = root.lookupType('dleague.v1.MatchEnd');

// Encoded envelope ready for `ws.send`.
export function encodeEnvelope(typeNum: MessageTypeValue, requestId: string, innerType: Type, body: object): Uint8Array {
  const payload = innerType.encode(innerType.create(body)).finish();
  return Envelope.encode(Envelope.create({ type: typeNum, requestId, payload })).finish();
}

// Decoded envelope + the inner body, looked up by the envelope's `type`.
export function decodeEnvelope(buf: Uint8Array): { type: MessageTypeValue; requestId: string; body: any } {
  const env = Envelope.decode(buf) as any;
  const inner = innerTypeFor(env.type);
  const body = inner ? inner.decode(env.payload) : null;
  return { type: env.type as MessageTypeValue, requestId: env.requestId, body };
}

function innerTypeFor(t: number): Type | null {
  switch (t) {
    case MessageType.PING: return Ping;
    case MessageType.PONG: return Pong;
    case MessageType.AUTH_REQUEST: return AuthRequest;
    case MessageType.AUTH_RESPONSE: return AuthResponse;
    case MessageType.JOIN_ROOM: return JoinRoom;
    case MessageType.JOIN_ROOM_ACK: return JoinRoomAck;
    case MessageType.TURN: return Turn;
    case MessageType.MATCH_END: return MatchEnd;
    default: return null;
  }
}

// Convenience: typed builders that avoid stringly-typed call sites.
export const Build = {
  auth: (idToken: string, requestId = 'auth') =>
    encodeEnvelope(MessageType.AUTH_REQUEST, requestId, AuthRequest, { idToken, version: 1 }),
  ping: (clientUnixMs: number, requestId = 'ping') =>
    encodeEnvelope(MessageType.PING, requestId, Ping, { clientUnixMs }),
  joinRoom: (matchId: string, requestId = 'join') =>
    encodeEnvelope(MessageType.JOIN_ROOM, requestId, JoinRoom, { matchId }),
  turn: (matchId: string, turnIndex: number, payload: Uint8Array, requestId = 'turn') =>
    encodeEnvelope(MessageType.TURN, requestId, Turn, { matchId, turnIndex, payload }),
  matchEnd: (matchId: string, winnerUid: string, scoreP1: number, scoreP2: number, requestId = 'end') =>
    encodeEnvelope(MessageType.MATCH_END, requestId, MatchEnd, { matchId, winnerUid, scoreP1, scoreP2 })
};

// Re-export so consumers can hold typed bodies without re-importing protobufjs.
export type AuthResponseBody = { ok: boolean; uid: string; error: string };
export type JoinRoomAckBody = { ok: boolean; matchId: string; role: string; opponentUid: string; error: string };
export type PongBody = { clientUnixMs: number; serverUnixMs: number };
export type TurnBody = { matchId: string; turnIndex: number; payload: Uint8Array };
export type MatchEndBody = { matchId: string; winnerUid: string; scoreP1: number; scoreP2: number };

// Silence unused warnings for symbols re-exported as types only above.
export { Reader, Writer };
