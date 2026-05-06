package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tiennm99/dleague/server/internal/store"
)

// puzzleHandler serves daily puzzles. The store impl is responsible for any
// caching layer behind GetPuzzle — handlers stay paradigm-agnostic.
type puzzleHandler struct {
	s   store.Store
	now func() time.Time
}

func newPuzzleHandler(s store.Store) *puzzleHandler {
	return &puzzleHandler{s: s, now: time.Now}
}

// GET /api/v1/puzzles/today — derives today's UTC date, then delegates to
// the date-specific handler.
func (h *puzzleHandler) today(w http.ResponseWriter, r *http.Request) {
	h.byDate(w, r, h.now().UTC().Format(dateFmt))
}

// GET /api/v1/puzzles/:date
func (h *puzzleHandler) get(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	if !validDate(date) {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	h.byDate(w, r, date)
}

func (h *puzzleHandler) byDate(w http.ResponseWriter, r *http.Request, date string) {
	p, err := h.s.GetPuzzle(r.Context(), date)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "puzzle not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store error")
		return
	}
	// Don't leak the solution.
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"date":       p.Date,
		"hint":       p.Hint,
		"difficulty": p.Difficulty,
		"length":     len(p.Word),
	})
}

// GET /api/v1/puzzles/me/today — auth-protected; includes the solution so
// the client UI can render per-guess color feedback without an extra
// per-keystroke round trip. The leaderboard score is still re-derived
// server-side in attempts.submit, so leaking the word here doesn't open a
// cheating vector beyond what dev tools already allow on a SPA.
func (h *puzzleHandler) meToday(w http.ResponseWriter, r *http.Request) {
	h.meByDate(w, r, h.now().UTC().Format(dateFmt))
}

// GET /api/v1/puzzles/me/:date — auth-protected variant of `get`.
func (h *puzzleHandler) meGet(w http.ResponseWriter, r *http.Request) {
	date := chi.URLParam(r, "date")
	if !validDate(date) {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	h.meByDate(w, r, date)
}

func (h *puzzleHandler) meByDate(w http.ResponseWriter, r *http.Request, date string) {
	p, err := h.s.GetPuzzle(r.Context(), date)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "puzzle not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}
