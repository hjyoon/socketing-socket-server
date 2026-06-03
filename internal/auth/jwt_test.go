package auth

import (
	"testing"
	"time"
)

func TestSignVerifyAndUUID(t *testing.T) {
	token, err := Sign(map[string]any{"sub": "u"}, "secret", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := Verify(token, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != "u" || len(UUID()) != 36 {
		t.Fatalf("claims or uuid mismatch")
	}
	if _, err := Verify(token, "bad"); err == nil {
		t.Fatalf("bad secret accepted")
	}
	old := randomBytes
	randomBytes = func([]byte) (int, error) { return 0, errFake{} }
	if len(UUID()) != 32 {
		t.Fatalf("fallback uuid failed")
	}
	randomBytes = old
}

type errFake struct{}

func (errFake) Error() string { return "fake" }
