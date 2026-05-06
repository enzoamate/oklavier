package mtls

import (
	"os"
	"testing"
)

func TestEnabled_AllUnset(t *testing.T) {
	resetEnv(t)
	if Enabled() {
		t.Fatal("Enabled() returned true with no env set")
	}
}

func TestEnabled_PartialUnset(t *testing.T) {
	resetEnv(t)
	t.Setenv("MTLS_CERT_FILE", "/foo")
	t.Setenv("MTLS_KEY_FILE", "/bar")
	// MTLS_CA_FILE is missing
	if Enabled() {
		t.Fatal("Enabled() returned true with partial env (missing MTLS_CA_FILE)")
	}
}

func TestEnabled_AllSet(t *testing.T) {
	resetEnv(t)
	t.Setenv("MTLS_CERT_FILE", "/foo")
	t.Setenv("MTLS_KEY_FILE", "/bar")
	t.Setenv("MTLS_CA_FILE", "/baz")
	if !Enabled() {
		t.Fatal("Enabled() returned false with all 3 vars set")
	}
}

func TestServerConfig_DisabledReturnsNilNil(t *testing.T) {
	resetEnv(t)
	cfg, err := ServerConfig()
	if err != nil || cfg != nil {
		t.Fatalf("disabled: want (nil, nil), got (%v, %v)", cfg, err)
	}
}

func TestClientTransport_DisabledReturnsNilNil(t *testing.T) {
	resetEnv(t)
	tr, err := ClientTransport()
	if err != nil || tr != nil {
		t.Fatalf("disabled: want (nil, nil), got (%v, %v)", tr, err)
	}
}

func TestServerConfig_BadFiles(t *testing.T) {
	resetEnv(t)
	t.Setenv("MTLS_CERT_FILE", "/nope")
	t.Setenv("MTLS_KEY_FILE", "/nope")
	t.Setenv("MTLS_CA_FILE", "/nope")
	if _, err := ServerConfig(); err == nil {
		t.Fatal("expected error when files don't exist")
	}
}

// resetEnv clears the MTLS_* env vars before each test so tests don't leak state.
func resetEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"MTLS_CERT_FILE", "MTLS_KEY_FILE", "MTLS_CA_FILE"} {
		_ = os.Unsetenv(k)
	}
}
