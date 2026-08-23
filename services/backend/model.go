package main

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

// Blog mirrors /blogs/{blogId} in firestore.rules.
//
// Reads are served to the frontend directly by the Firebase SDK, gated by those rules - this
// service only handles writes, so that createdAt/updatedAt come from the server rather than a
// client clock. Nothing here re-implements the rules' read condition.
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
