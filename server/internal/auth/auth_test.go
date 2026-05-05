package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/tiennm99/dleague/server/internal/auth"
	"github.com/tiennm99/dleague/server/internal/store"
	"github.com/tiennm99/dleague/server/internal/store/memstore"
)

// fakeVerifier returns a fixed claims/error pair. Toggle to drive each path.
type fakeVerifier struct {
	claims store.AuthClaims
	err    error
	calls  int32
}

func (f *fakeVerifier) Verify(_ context.Context, tok string) (store.AuthClaims, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.err != nil {
		return store.AuthClaims{}, f.err
	}
	c := f.claims
	if c.UID == "" {
		c.UID = "uid-from-" + tok
	}
	return c, nil
}

func newServer(t *testing.T, v auth.Verifier, up auth.Upserter) http.Handler {
	t.Helper()
	mw := auth.Middleware(v, up)
	return mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := auth.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "no claims", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(c.UID))
	}))
}

func TestMiddlewareMissingHeader401(t *testing.T) {
	h := newServer(t, &fakeVerifier{}, nil)
	r := httptest.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got == "" {
		t.Errorf("missing WWW-Authenticate header")
	}
}

func TestMiddlewareMalformedHeader401(t *testing.T) {
	h := newServer(t, &fakeVerifier{}, nil)
	for _, hdr := range []string{"Token abc", "Bearer", "Bearer  ", "garbage"} {
		r := httptest.NewRequest("GET", "/p", nil)
		r.Header.Set("Authorization", hdr)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("hdr=%q status = %d, want 401", hdr, w.Code)
		}
	}
}

func TestMiddlewareInvalidToken401(t *testing.T) {
	v := &fakeVerifier{err: auth.ErrInvalidToken}
	h := newServer(t, v, nil)
	r := httptest.NewRequest("GET", "/p", nil)
	r.Header.Set("Authorization", "Bearer expired-jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if got := w.Header().Get("WWW-Authenticate"); got != `Bearer error="invalid_token"` {
		t.Errorf("WWW-Authenticate = %q", got)
	}
}

func TestMiddlewareValidTokenAttachesClaimsAndUpserts(t *testing.T) {
	mem := memstore.New()
	t.Cleanup(func() { _ = mem.Close() })

	v := &fakeVerifier{claims: store.AuthClaims{UID: "u1", Email: "x@y", Provider: "password"}}
	h := newServer(t, v, mem)

	r := httptest.NewRequest("GET", "/p", nil)
	r.Header.Set("Authorization", "Bearer good-jwt")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", w.Code, w.Body.String())
	}
	if w.Body.String() != "u1" {
		t.Errorf("body = %q, want u1", w.Body.String())
	}
	if v.calls != 1 {
		t.Errorf("verifier calls = %d, want 1", v.calls)
	}

	// User must be upserted with beta fields stamped.
	u, err := mem.GetUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if !u.IsBetaTester || u.BetaSignupAt.IsZero() {
		t.Errorf("beta fields not stamped: %+v", u)
	}
}

func TestGateRejectsEmptyAndPropagatesUpsert(t *testing.T) {
	mem := memstore.New()
	t.Cleanup(func() { _ = mem.Close() })

	g := auth.NewGate(&fakeVerifier{claims: store.AuthClaims{UID: "u9"}}, mem)
	if _, err := g.Verify(context.Background(), ""); !errors.Is(err, auth.ErrMissingToken) {
		t.Errorf("err = %v, want ErrMissingToken", err)
	}
	c, err := g.Verify(context.Background(), "good")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if c.UID != "u9" {
		t.Errorf("UID = %q", c.UID)
	}
	if _, err := mem.GetUser(context.Background(), "u9"); err != nil {
		t.Errorf("upsert side effect missing: %v", err)
	}
}
