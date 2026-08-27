package entity

import (
	"errors"
	"strings"
	"testing"
)

func TestUser_SetDisplayName(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  string
		valid bool
	}{
		{"plain name", "Ed", "Ed", true},
		{"trims surrounding space", "  Ed  ", "Ed", true},
		{"at the length limit", strings.Repeat("a", MaxDisplayNameLength), strings.Repeat("a", MaxDisplayNameLength), true},
		{"empty is rejected", "", "", false},
		{"whitespace only is rejected", "   ", "", false},
		{"over the length limit is rejected", strings.Repeat("a", MaxDisplayNameLength+1), "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var user User
			err := user.SetDisplayName(tt.input)

			if tt.valid {
				if err != nil {
					t.Fatalf("SetDisplayName(%q) = %v, want no error", tt.input, err)
				}
				if user.DisplayName != tt.want {
					t.Errorf("DisplayName = %q, want %q", user.DisplayName, tt.want)
				}
				return
			}

			var invalid ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("SetDisplayName(%q) = %v, want a ValidationError", tt.input, err)
			}
			if invalid.Field != "displayName" {
				t.Errorf("Field = %q, want %q", invalid.Field, "displayName")
			}
			if user.DisplayName != "" {
				t.Errorf("DisplayName = %q, want the rejected value not to be applied", user.DisplayName)
			}
		})
	}
}

// A rune is one character even when it is several bytes, so the limit counts runes not bytes.
func TestUser_SetDisplayName_CountsRunesNotBytes(t *testing.T) {
	var user User
	name := strings.Repeat("é", MaxDisplayNameLength)

	if err := user.SetDisplayName(name); err != nil {
		t.Fatalf("SetDisplayName = %v, want no error for %d runes", err, MaxDisplayNameLength)
	}
}

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
	if err := (User{ID: "u1", Username: "sly-dancing-monkey", DisplayName: "Ed", Bio: "hello"}).Validate(); err != nil {
		t.Errorf("Validate = %v, want no error", err)
	}
	if err := (User{ID: "u1", Username: "sly-dancing-monkey", DisplayName: ""}).Validate(); err == nil {
		t.Error("Validate with no display name = nil, want an error")
	}
	if err := (User{ID: "u1", Username: "sly-dancing-monkey", DisplayName: "Ed", Bio: strings.Repeat("a", MaxBioLength+1)}).Validate(); err == nil {
		t.Error("Validate with an overlong bio = nil, want an error")
	}

	// Every profile is looked up by username, so one without a name is not storable.
	if err := (User{ID: "u1", DisplayName: "Ed"}).Validate(); err == nil {
		t.Error("Validate with no username = nil, want an error")
	}

	// Profiles are keyed by id, so one without an id has nowhere to be written.
	if err := (User{Username: "sly-dancing-monkey", DisplayName: "Ed"}).Validate(); err == nil {
		t.Error("Validate with no id = nil, want an error")
	}
}
