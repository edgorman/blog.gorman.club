package entity

import "testing"

func verified(email string) Caller {
	return Caller{UID: "uid", Email: email, EmailVerified: true}
}

func TestAssistantAllowlist_Allows(t *testing.T) {
	// Case is folded on both sides, so a list entry enables the account however the address was
	// spelled in the token.
	allowlist := NewAssistantAllowlist([]string{"ejgorman@gmail.com", "  Someone.Else@example.com  ", ""})

	for _, tt := range []struct {
		email   string
		allowed bool
	}{
		{"ejgorman@gmail.com", true},
		{"EJGorman@Gmail.com", true},
		{"someone.else@example.com", true},
		{"stranger@example.com", false},
		{"ejgorman@gmail.com.evil.example", false},
		{"", false},
	} {
		t.Run(tt.email, func(t *testing.T) {
			if got := allowlist.Allows(verified(tt.email)); got != tt.allowed {
				t.Errorf("Allows(%q) = %v, want %v", tt.email, got, tt.allowed)
			}
		})
	}
}

// An address the provider did not verify never matches. Without this the allowlist would be keyed
// on something an account could merely claim rather than on something it proved.
func TestAssistantAllowlist_AllowsRequiresAVerifiedAddress(t *testing.T) {
	allowlist := NewAssistantAllowlist([]string{"ejgorman@gmail.com"})

	unverified := Caller{UID: "uid", Email: "ejgorman@gmail.com"}
	if allowlist.Allows(unverified) {
		t.Error("Allows(unverified) = true, want false")
	}
	if !allowlist.Allows(verified("ejgorman@gmail.com")) {
		t.Error("Allows(verified) = false, want true - the same address, proved")
	}
	// The anonymous caller carries no address at all, so it can never match an entry.
	if allowlist.Allows(Caller{}) {
		t.Error("Allows(anonymous) = true, want false")
	}
}

// An unconfigured deployment enables the assistant for nobody, rather than for everybody.
func TestAssistantAllowlist_Empty(t *testing.T) {
	for _, tt := range []struct {
		name   string
		emails []string
		empty  bool
	}{
		{"unset", nil, true},
		{"blank entries only", []string{"", "  "}, true},
		{"one address", []string{"ejgorman@gmail.com"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			allowlist := NewAssistantAllowlist(tt.emails)

			if allowlist.Empty() != tt.empty {
				t.Errorf("Empty() = %v, want %v", allowlist.Empty(), tt.empty)
			}
			if allowlist.Allows(verified("ejgorman@gmail.com")) == tt.empty {
				t.Errorf("Allows disagrees with Empty for %v", tt.emails)
			}
		})
	}
}
