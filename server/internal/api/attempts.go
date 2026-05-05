package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/tiennm99/dleague/server/internal/auth"
	"github.com/tiennm99/dleague/server/internal/store"
)

// attemptHandler owns the POST/GET attempt endpoints. Auth is required —
// callers must be wrapped in auth.Middleware so ClaimsFromContext returns ok.
type attemptHandler struct {
	s   store.Store
	now func() time.Time
}

func newAttemptHandler(s store.Store) *attemptHandler {
	return &attemptHandler{s: s, now: time.Now}
}

type submitAttemptRequest struct {
	Date    string   `json:"date"`
	Guesses []string `json:"guesses"`
}

type submitAttemptResponse struct {
	Score int64 `json:"score"`
	Won   bool  `json:"won"`
}

// POST /api/v1/attempts — server re-scores using the persisted puzzle so a
// bad client cannot inflate its own score.
func (h *attemptHandler) submit(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	var req submitAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if !validDate(req.Date) {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	if !dateInWindow(req.Date, h.now().UTC()) {
		writeError(w, http.StatusBadRequest, "date out of submit window")
		return
	}
	if len(req.Guesses) == 0 {
		writeError(w, http.StatusBadRequest, "no guesses")
		return
	}

	puzzle, err := h.s.GetPuzzle(r.Context(), req.Date)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "puzzle not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store error")
		return
	}

	attempt := finalize(store.Attempt{
		UID:         claims.UID,
		PuzzleDate:  req.Date,
		Guesses:     req.Guesses,
		CompletedAt: h.now().UTC(),
	}, puzzle.Word)

	if err := h.s.UpsertAttempt(r.Context(), attempt); err != nil {
		writeError(w, http.StatusInternalServerError, "persist attempt")
		return
	}

	// Leaderboard updates are best-effort: a Redis hiccup must not block
	// the user-visible response. Logging hooks in once a logger is wired.
	if attempt.Score > 0 {
		_ = h.s.SubmitScore(r.Context(), "lb:daily:"+req.Date, claims.UID, attempt.Score)
		_ = h.s.SubmitScore(r.Context(), "lb:global:alltime", claims.UID, attempt.Score)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(submitAttemptResponse{
		Score: attempt.Score, Won: attempt.Won,
	})
}

// GET /api/v1/attempts/me/:date — resume an in-progress attempt or return
// the final state for a closed one. 404 if absent.
func (h *attemptHandler) mine(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "no claims")
		return
	}
	date := chi.URLParam(r, "date")
	if !validDate(date) {
		writeError(w, http.StatusBadRequest, "invalid date")
		return
	}
	a, err := h.s.GetAttempt(r.Context(), claims.UID, date)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "no attempt")
			return
		}
		writeError(w, http.StatusInternalServerError, "store error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(a)
}
