package main

import (
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("not found")
)

// User mirrors /users/{userId} in firestore.rules - a profile document keyed by the owner's
// Firebase Auth uid.
type User struct {
	ID          string `json:"id" firestore:"-"`
	DisplayName string `json:"displayName" firestore:"displayName"`
}

// Blog mirrors /blogs/{blogId} in firestore.rules.
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

// visibleTo mirrors firestore.rules' read condition for /blogs/{blogId}: the Admin SDK this
// backend uses bypasses those rules entirely, so the same check has to be re-enforced here.
func (b Blog) visibleTo(uid string) bool {
	if b.Visibility == "public" {
		return true
	}
	if uid == "" {
		return false
	}
	if b.OwnerID == uid {
		return true
	}
	for _, id := range b.AllowedUserIDs {
		if id == uid {
			return true
		}
	}
	return false
}
