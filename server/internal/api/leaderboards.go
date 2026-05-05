package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tiennm99/dleague/server/internal/store"
)

const defaultTopN = 50

type leaderboardHandler struct {
	s   store.Store
	now func() time.Time
}

func newLeaderboardHandler(s store.Store) *leaderboardHandler {
	return &leaderboardHandler{s: s, now: time.Now}
}

// GET /api/v1/leaderboards/{scope}/{date?} — scope ∈ {global, daily}. Daily
// requires :date or defaults to today's UTC date.
func (h *leaderboardHandler) get(w http.ResponseWriter, r *http.Request) {
	scope := chi.URLParam(r, "scope")
	date := chi.URLParam(r, "date")

	var key string
	switch scope {
	case "global":
		key = "lb:global:alltime"
	case "daily":
		if date == "" {
			date = h.now().UTC().Format(dateFmt)
		}
		if !validDate(date) {
			writeError(w, http.StatusBadRequest, "invalid date")
			return
		}
		key = "lb:daily:" + date
	default:
		writeError(w, http.StatusBadRequest, "scope must be global or daily")
		return
	}

	n := defaultTopN
	if v := r.URL.Query().Get("n"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 1000 {
			n = parsed
		}
	}

	rows, err := h.s.TopN(r.Context(), key, n)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leaderboard error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"scope": scope,
		"date":  date,
		"rows":  rows,
	})
}
