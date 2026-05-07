package config

import (
	"errors"
	"testing"
)

// envSet is a tiny helper: applies a map of env vars via t.Setenv (auto
// cleanup) and then runs the test. Empty value = key explicitly unset.
func envSet(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		if v == "" {
			t.Setenv(k, "") // present but empty triggers required-field validation
		} else {
			t.Setenv(k, v)
		}
	}
}

// validEnv returns the minimum-viable env to make Load() succeed. Tests
// override individual keys to drive specific failure paths.
func validEnv() map[string]string {
	return map[string]string{
		"FIREBASE_CREDENTIALS_JSON": `{"type":"service_account","project_id":"x"}`,
		"FIREBASE_PROJECT_ID":       "dleague-beta",
		"MONGODB_URI":               "mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority",
	}
}

func TestLoadOK(t *testing.T) {
	envSet(t, validEnv())
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr default = %q, want :8080", cfg.Addr)
	}
	if cfg.WebRoot != "./web" {
		t.Errorf("WebRoot default = %q, want ./web", cfg.WebRoot)
	}
	if cfg.FirebaseProjectID != "dleague-beta" {
		t.Errorf("FirebaseProjectID = %q", cfg.FirebaseProjectID)
	}
	if cfg.MongoDB != "dleague" {
		t.Errorf("MongoDB default = %q, want dleague", cfg.MongoDB)
	}
}

func TestLoadOverrides(t *testing.T) {
	env := validEnv()
	env["DLEAGUE_ADDR"] = ":9090"
	env["DLEAGUE_WEB"] = "./dist"
	env["DLEAGUE_WS_ORIGINS"] = "https://a.test, https://b.test , "
	env["MONGODB_DB"] = "dleague_alt"
	envSet(t, env)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.WebRoot != "./dist" {
		t.Errorf("WebRoot = %q", cfg.WebRoot)
	}
	if cfg.MongoDB != "dleague_alt" {
		t.Errorf("MongoDB override = %q", cfg.MongoDB)
	}
	want := []string{"https://a.test", "https://b.test"}
	if len(cfg.AllowedOrigins) != len(want) {
		t.Fatalf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	for i, w := range want {
		if cfg.AllowedOrigins[i] != w {
			t.Errorf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], w)
		}
	}
}

func TestLoadMissingRequired(t *testing.T) {
	cases := []struct {
		name    string
		unset   string
		wantErr error
	}{
		{"firebase creds", "FIREBASE_CREDENTIALS_JSON", ErrMissingFirebaseCredentials},
		{"firebase project", "FIREBASE_PROJECT_ID", ErrMissingFirebaseProject},
		{"mongo uri", "MONGODB_URI", ErrMissingMongoURI},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := validEnv()
			env[tc.unset] = ""
			envSet(t, env)
			_, err := Load()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestLoadMalformedFirebaseJSON(t *testing.T) {
	env := validEnv()
	env["FIREBASE_CREDENTIALS_JSON"] = "{not json"
	envSet(t, env)
	_, err := Load()
	if !errors.Is(err, ErrMalformedFirebaseJSON) {
		t.Fatalf("err = %v, want ErrMalformedFirebaseJSON", err)
	}
}
