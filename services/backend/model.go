package main

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

// Blog is a /blogs/{blogId} document.
//
// Every read and write goes through this service: it is the only thing holding credentials for
// the collection, so access rules live in Go (canRead, requireOwnedBlog) and nowhere else.
type Blog struct {
	ID             string    `json:"id" firestore:"-"`
	OwnerID        string    `json:"ownerId" firestore:"ownerId"`
	Title          string    `json:"title" firestore:"title"`
	Content        string    `json:"content" firestore:"content"`
	Visibility     string    `json:"visibility" firestore:"visibility"`
	AllowedUserIDs []string  `json:"allowedUserIds,omitempty" firestore:"allowedUserIds"`
	CreatedAt      time.Time `json:"createdAt" firestore:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt" firestore:"updatedAt"`
}

// User is a /users/{userId} profile document, keyed by the owner's Google account ID (the `sub`
// claim of their ID token, see auth.go). There is no
// server-assigned ID to hand out, so profiles are written with PUT /users/{id} rather than POSTed.
//
// Any signed-in caller may read a profile; only its owner may write one (requireSelf).
type User struct {
	ID          string    `json:"id" firestore:"-"`
	DisplayName string    `json:"displayName" firestore:"displayName"`
	Bio         string    `json:"bio,omitempty" firestore:"bio"`
	CreatedAt   time.Time `json:"createdAt" firestore:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt" firestore:"updatedAt"`
}
