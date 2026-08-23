package main

import "testing"

func TestBlogVisibleTo(t *testing.T) {
	tests := []struct {
		name string
		blog Blog
		uid  string
		want bool
	}{
		{"public, anonymous", Blog{Visibility: "public"}, "", true},
		{"public, any user", Blog{Visibility: "public"}, "someone", true},
		{"private, anonymous", Blog{Visibility: "private", OwnerID: "owner"}, "", false},
		{"private, owner", Blog{Visibility: "private", OwnerID: "owner"}, "owner", true},
		{"private, allowed user", Blog{Visibility: "private", OwnerID: "owner", AllowedUserIDs: []string{"friend"}}, "friend", true},
		{"private, stranger", Blog{Visibility: "private", OwnerID: "owner", AllowedUserIDs: []string{"friend"}}, "stranger", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.blog.visibleTo(tt.uid); got != tt.want {
				t.Errorf("visibleTo(%q) = %v, want %v", tt.uid, got, tt.want)
			}
		})
	}
}
