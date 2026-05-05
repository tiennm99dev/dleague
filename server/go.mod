module github.com/tiennm99/dleague/server

go 1.26

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/tiennm99/dleague/shared v0.0.0
	google.golang.org/protobuf v1.36.11
	nhooyr.io/websocket v1.8.17
)

replace github.com/tiennm99/dleague/shared => ../shared
