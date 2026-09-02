package entity

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestUser_SetBio(t *testing.T) {
	var user User

	if err := user.SetBio("  hello  "); err != nil {
		t.Fatalf("SetBio = %v, want no error", err)
	}
	if user.Bio != "hello" {
		t.Errorf("Bio = %q, want %q", user.Bio, "hello")
	}

	// An empty bio is allowed; it is an optional field.
	if err := user.SetBio(""); err != nil {
		t.Errorf("SetBio(\"\") = %v, want no error", err)
	}

	if err := user.SetBio(strings.Repeat("a", MaxBioLength+1)); err == nil {
		t.Error("SetBio over the limit = nil, want a ValidationError")
	}
}

func TestUser_SetUsername(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  string
		valid bool
	}{
		{"three words", "sly-dancing-monkey", "sly-dancing-monkey", true},
		{"trims surrounding space", "  sly-dancing-monkey  ", "sly-dancing-monkey", true},
		{"digits are allowed", "otter99", "otter99", true},
		{"case is preserved", "SlyMonkey", "SlyMonkey", true},
		{"at the lower limit", strings.Repeat("a", MinUsernameLength), strings.Repeat("a", MinUsernameLength), true},
		{"at the upper limit", strings.Repeat("a", MaxUsernameLength), strings.Repeat("a", MaxUsernameLength), true},
		{"empty is rejected", "", "", false},
		{"whitespace only is rejected", "   ", "", false},
		{"below the lower limit is rejected", strings.Repeat("a", MinUsernameLength-1), "", false},
		{"over the upper limit is rejected", strings.Repeat("a", MaxUsernameLength+1), "", false},
		{"inner space is rejected", "sly dancing monkey", "", false},
		{"underscores are rejected", "sly_dancing_monkey", "", false},
		{"a reserved name is rejected", "me", "", false},
		{"a reserved name is rejected whatever its case", "ME", "", false},
		{"other punctuation is rejected", "sly.dancing.monkey", "", false},
		{"a slash is rejected", "sly/monkey", "", false},
		{"non-ascii is rejected", "slyé-monkey", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var user User
			err := user.SetUsername(tt.input)

			if tt.valid {
				if err != nil {
					t.Fatalf("SetUsername(%q) = %v, want no error", tt.input, err)
				}
				if user.Username != tt.want {
					t.Errorf("Username = %q, want %q", user.Username, tt.want)
				}
				return
			}

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("SetUsername(%q) = %v, want a ValidationError", tt.input, err)
			}
			if invalid.Field != "username" {
				t.Errorf("Field = %q, want %q", invalid.Field, "username")
			}
			if user.Username != "" {
				t.Errorf("Username = %q, want the rejected value not to be applied", user.Username)
			}
		})
	}
}

// Uniqueness is enforced on the folded form, so two names differing only in case are one name.
func TestUser_UsernameKey(t *testing.T) {
	if got := (User{Username: "SlyMonkey"}).UsernameKey(); got != "slymonkey" {
		t.Errorf("UsernameKey = %q, want %q", got, "slymonkey")
	}
	if (User{Username: "Ed-Otter"}).UsernameKey() != (User{Username: "ed-otter"}).UsernameKey() {
		t.Error("names differing only in case produced different keys")
	}
}

func TestUser_Validate(t *testing.T) {
	if err := (User{ID: "u1", Username: "sly-dancing-monkey", Bio: "hello"}).Validate(); err != nil {
		t.Errorf("Validate = %v, want no error", err)
	}
	if err := (User{ID: "u1", Username: "sly-dancing-monkey", Bio: strings.Repeat("a", MaxBioLength+1)}).Validate(); err == nil {
		t.Error("Validate with an overlong bio = nil, want an error")
	}

	// Every profile is looked up by username, so one without a name is not storable.
	if err := (User{ID: "u1"}).Validate(); err == nil {
		t.Error("Validate with no username = nil, want an error")
	}

	// Profiles are keyed by id, so one without an id has nowhere to be written.
	if err := (User{Username: "sly-dancing-monkey"}).Validate(); err == nil {
		t.Error("Validate with no id = nil, want an error")
	}
}

// The repository stores what Normalized returns, so a value that only differs by surrounding space
// must come back trimmed rather than merely being accepted - a raw " alice " would be reserved
// under a key no lookup could match.
func TestUser_Normalized(t *testing.T) {
	got, err := (User{ID: "u1", Username: "  Sly-Dancing-Monkey  ", Bio: "  hi  "}).Normalized()
	if err != nil {
		t.Fatalf("Normalized = %v, want no error", err)
	}
	if got.Username != "Sly-Dancing-Monkey" {
		t.Errorf("Username = %q, want it trimmed", got.Username)
	}
	if got.UsernameKey() != "sly-dancing-monkey" {
		t.Errorf("UsernameKey = %q, want the trimmed name folded", got.UsernameKey())
	}
	if got.Bio != "hi" {
		t.Errorf("Bio = %q, want it trimmed", got.Bio)
	}

	if _, err := (User{ID: "u1", Username: "me"}).Normalized(); err == nil {
		t.Error("Normalized with a reserved username = nil, want an error")
	}
}

// Subscribed is the whole of what "has this account paid" means: never subscribed and lapsed are
// the same answer, since the only thing anything here asks is whether the account may spend now.
func TestUser_Subscribed(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	future := now.Add(time.Second)

	for _, tt := range []struct {
		name string
		user User
		want bool
	}{
		{"never subscribed", User{ID: "user-1"}, false},
		{"subscribed until later", User{ID: "user-1", SubscribedUntil: &future}, true},
		{"ran out", User{ID: "user-1", SubscribedUntil: &past}, false},
		// The instant it runs out is not a moment of access: the boundary belongs to the side that
		// refuses, so a subscription cannot be extended by asking at exactly the wrong microsecond.
		{"runs out exactly now", User{ID: "user-1", SubscribedUntil: &now}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.Subscribed(now); got != tt.want {
				t.Errorf("Subscribed = %v, want %v", got, tt.want)
			}
		})
	}
}

// A profile is public to read and nobody's to write but its own account's.
func TestUser_Permission(t *testing.T) {
	user := User{ID: "user-1", Username: "calm-smiling-kestrel"}

	if !user.Permission(ActionRead).Allows("") {
		t.Error("Permission(read) refused an anonymous caller, want a public profile")
	}
	for _, action := range []Action{ActionUpdate, ActionDelete} {
		if !user.Permission(action).Allows("user-1") {
			t.Errorf("Permission(%q) refused the account itself", action)
		}
		for _, uid := range []string{"another", ""} {
			if user.Permission(action).Allows(uid) {
				t.Errorf("Permission(%q).Allows(%q) = true, want only the account itself", action, uid)
			}
		}
	}
}

// Nothing a client sends can subscribe an account: the profile a request applies to is the stored
// one, and a subscription rides through untouched by the setters a request goes through.
func TestUser_NormalizedKeepsTheSubscription(t *testing.T) {
	until := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	user := User{ID: "user-1", Username: "calm-smiling-kestrel", SubscribedUntil: &until}

	normalized, err := user.Normalized()
	if err != nil {
		t.Fatalf("Normalized = %v, want no error", err)
	}
	if normalized.SubscribedUntil == nil || !normalized.SubscribedUntil.Equal(until) {
		t.Errorf("SubscribedUntil = %v, want %v", normalized.SubscribedUntil, until)
	}
}
