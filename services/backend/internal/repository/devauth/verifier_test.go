package devauth

import (
	"context"
	"testing"
)

func TestTokenVerifier(t *testing.T) {
	for _, tt := range []struct {
		token   string
		wantUID string
	}{
		{"dev", "dev"},
		{"dev:bob", "bob"},
		{"dev:", ""},
		{"devbob", ""},
		{"", ""},
		{"eyJhbGciOiJSUzI1NiJ9.e30.sig", ""},
	} {
		caller, err := TokenVerifier{}.Verify(context.Background(), tt.token)
		if caller.UID != tt.wantUID || (err == nil) != (tt.wantUID != "") {
			t.Errorf("Verify(%q) = %+v, %v; want uid %q", tt.token, caller, err, tt.wantUID)
		}
	}
}
