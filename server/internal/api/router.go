// Package api wires the async-PvP REST surface under /api/v1/.
//
// Handlers depend on `store.Store` only — the composed (Couchbase + Redis)
// impl in production, memstore in tests. Auth is handled by the middleware
// from `internal/auth`; protected routes are wrapped at the router level.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tiennm99/dleague/server/internal/auth"
	"github.com/tiennm99/dleague/server/internal/store"
)

const dateFmt = "2006-01-02"

// gracePastDays — clients can submit attempts dated up to this many days
// before today's UTC date. Beta posture: keep tight; expand later if
// timezone complaints surface.
const gracePastDays = 1

// Mount wires the API onto a chi.Router under /api/v1/. `verifier` and
// `upserter` may be nil to disable auth (tests without auth coverage).
func Mount(parent chi.Router, s store.Store, verifier auth.Verifier, upserter auth.Upserter) {
	puzzles := newPuzzleHandler(s)
	attempts := newAttemptHandler(s)
	leaderboards := newLeaderboardHandler(s)

	parent.Route("/api/v1", func(r chi.Router) {
		// Public reads.
		r.Get("/puzzles/today", puzzles.today)
		r.Get("/puzzles/{date}", puzzles.get)
		r.Get("/leaderboards/{scope}", leaderboards.get)
		r.Get("/leaderboards/{scope}/{date}", leaderboards.get)

		// Auth-protected writes / personalized reads.
		r.Group(func(r chi.Router) {
			if verifier != nil {
				r.Use(auth.Middleware(verifier, upserter))
			}
			r.Post("/attempts", attempts.submit)
			r.Get("/attempts/me/{date}", attempts.mine)
			r.Get("/puzzles/me/today", puzzles.meToday)
			r.Get("/puzzles/me/{date}", puzzles.meGet)
		})
	})
}

// validDate enforces YYYY-MM-DD shape. Returns false on parse failure.
func validDate(s string) bool {
	if len(s) != len(dateFmt) {
		return false
	}
	_, err := time.Parse(dateFmt, s)
	return err == nil
}

// dateInWindow returns true when `date` is today (UTC) or up to
// gracePastDays before. Future dates are rejected.
func dateInWindow(date string, nowUTC time.Time) bool {
	d, err := time.Parse(dateFmt, date)
	if err != nil {
		return false
	}
	today := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
	if d.After(today) {
		return false
	}
	earliest := today.AddDate(0, 0, -gracePastDays)
	return !d.Before(earliest)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
