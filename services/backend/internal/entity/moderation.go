package entity

import (
	"slices"
	"time"
)

// ModerationStatus is what screening decided about a comment. A comment with no Moderation at all
// has not been screened yet, which is the state every comment starts in and the one it stays in if
// the classifier is down: comments are published first and hidden afterwards if flagged, so an
// outage leaves them unscreened rather than unposted.
type ModerationStatus string

const (
	ModerationApproved ModerationStatus = "approved"
	ModerationFlagged  ModerationStatus = "flagged"
)

// ModerationCategories are the reasons a classifier may give. "none" is the category of an
// approved comment.
var ModerationCategories = []string{"spam", "harassment", "hate", "sexual", "dangerous", "none"}

// Moderation is the screening result stored on a comment. Model names whoever decided it: the
// classifier's model id, or empty when the post's owner approved it by hand.
type Moderation struct {
	Status   ModerationStatus `json:"status"`
	Category string           `json:"category"`
	Model    string           `json:"model,omitempty"`
	At       time.Time        `json:"at"`
}

// NewModeration builds the result of one classification, refusing a category outside
// ModerationCategories so a model that answers off-schema is a failure rather than a verdict.
func NewModeration(flagged bool, category, model string) (Moderation, error) {
	if !slices.Contains(ModerationCategories, category) {
		return Moderation{}, ValidationError{Field: "category", Message: "is not a moderation category"}
	}
	status := ModerationApproved
	if flagged {
		status = ModerationFlagged
	}
	return Moderation{Status: status, Category: category, Model: model, At: time.Now().UTC()}, nil
}

// Flagged reports whether the comment is hidden from readers other than its author and the post's
// owner.
func (c Comment) Flagged() bool {
	return c.Moderation != nil && c.Moderation.Status == ModerationFlagged
}

// VisibleTo reports whether uid is shown the comment in its post's thread. A flagged comment is
// hidden from everybody except its author, who sees nothing different (so a spammer learns nothing
// from being screened), and the post's owner, who decides whether to approve or delete it. Whether
// the thread is reachable at all is the post's own read permission, asked first.
func (c Comment) VisibleTo(uid string, post Blog) bool {
	return !c.Flagged() || (uid != "" && (uid == c.AuthorID || uid == post.OwnerID))
}

// ModerationPermission is who may see and overrule the classifier's verdict on a comment: the
// owner of the post it sits under, and nobody else - not even its author, who could otherwise
// approve their own spam.
func (c Comment) ModerationPermission(action Action, post Blog) Permission {
	permission := PermissionFor(ResourceModeration, action)
	permission.OwnerID = post.OwnerID
	return permission
}
