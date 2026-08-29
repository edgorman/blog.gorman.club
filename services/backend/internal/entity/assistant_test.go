package entity

import "testing"

func TestAssistantAllowlist_Allows(t *testing.T) {
	// Case is folded exactly as username uniqueness is, so a list naming "edgorman" enables the
	// account holding that name however it was typed.
	allowlist := NewAssistantAllowlist([]string{"edgorman", "  Someone-Else  ", ""})

	for _, tt := range []struct {
		username string
		allowed  bool
	}{
		{"edgorman", true},
		{"EdGorman", true},
		{"someone-else", true},
		{"sly-dancing-monkey", false},
		{"edgorman2", false},
		{"", false},
	} {
		t.Run(tt.username, func(t *testing.T) {
			if got := allowlist.Allows(User{ID: "uid", Username: tt.username}); got != tt.allowed {
				t.Errorf("Allows(%q) = %v, want %v", tt.username, got, tt.allowed)
			}
		})
	}
}

// An unconfigured deployment enables the assistant for nobody, rather than for everybody.
func TestAssistantAllowlist_Empty(t *testing.T) {
	for _, tt := range []struct {
		name      string
		usernames []string
		empty     bool
	}{
		{"unset", nil, true},
		{"blank entries only", []string{"", "  "}, true},
		{"one name", []string{"edgorman"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			allowlist := NewAssistantAllowlist(tt.usernames)

			if allowlist.Empty() != tt.empty {
				t.Errorf("Empty() = %v, want %v", allowlist.Empty(), tt.empty)
			}
			if allowlist.Allows(User{ID: "uid", Username: "edgorman"}) == tt.empty {
				t.Errorf("Allows disagrees with Empty for %v", tt.usernames)
			}
		})
	}
}
