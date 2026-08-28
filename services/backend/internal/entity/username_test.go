package entity

import (
	"strings"
	"testing"
)

// A generated name goes straight into storage without passing back through the HTTP layer, so the
// pools have to be incapable of producing one SetUsername would reject. Rather than sample, this
// builds the single worst case - the longest word from each pool - and checks that.
func TestNewUsername_LongestCombinationIsValid(t *testing.T) {
	longest := func(pool []string) string {
		var found string
		for _, word := range pool {
			if len(word) > len(found) {
				found = word
			}
		}
		return found
	}

	worst := strings.Join([]string{
		longest(usernameAdjectives),
		longest(usernameActions),
		longest(usernameAnimals),
	}, usernameSeparator)

	var user User
	if err := user.SetUsername(worst); err != nil {
		t.Fatalf("SetUsername(%q) = %v, want the longest possible name to be valid", worst, err)
	}
}

// Every word must be storable on its own terms too: lowercase, ASCII, and free of the separator,
// since a stray one would change how many words a name appears to have.
func TestUsernamePoolsAreWellFormed(t *testing.T) {
	for _, pool := range []struct {
		name  string
		words []string
	}{
		{"adjectives", usernameAdjectives},
		{"actions", usernameActions},
		{"animals", usernameAnimals},
	} {
		t.Run(pool.name, func(t *testing.T) {
			if len(pool.words) == 0 {
				t.Fatal("pool is empty, which would make every name identical")
			}

			seen := make(map[string]bool, len(pool.words))
			for _, word := range pool.words {
				if !usernamePattern.MatchString(word) || strings.Contains(word, usernameSeparator) {
					t.Errorf("%q is not a plain word of letters and digits", word)
				}
				if word != strings.ToLower(word) {
					t.Errorf("%q is not lowercase", word)
				}
				if seen[word] {
					t.Errorf("%q appears twice, which skews the draw towards it", word)
				}
				seen[word] = true
			}
		})
	}
}

func TestNewUsername_ShapeAndValidity(t *testing.T) {
	// Enough draws to exercise a spread of the pools while staying a fast unit test.
	for range 500 {
		name := NewUsername()

		var user User
		if err := user.SetUsername(name); err != nil {
			t.Fatalf("SetUsername(%q) = %v, want every generated name to be valid", name, err)
		}
		if got := strings.Count(name, usernameSeparator); got != 2 {
			t.Fatalf("NewUsername() = %q, want two descriptive words followed by an animal", name)
		}
	}
}

// Two draws being equal is possible but should be rare; a generator stuck on one name would make
// every sign-up after the first collide.
func TestNewUsername_Varies(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		seen[NewUsername()] = true
	}

	if len(seen) < 90 {
		t.Errorf("100 draws produced only %d distinct names, want the pools to be doing their job", len(seen))
	}
}
