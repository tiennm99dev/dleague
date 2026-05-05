module github.com/tiennm99/dleague/server

go 1.26

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-sql-driver/mysql v1.10.0
	github.com/google/uuid v1.6.0
	github.com/tiennm99/dleague/shared v0.0.0
	google.golang.org/protobuf v1.36.11
	nhooyr.io/websocket v1.8.17
)

require filippo.io/edwards25519 v1.2.0 // indirect

replace github.com/tiennm99/dleague/shared => ../shared
