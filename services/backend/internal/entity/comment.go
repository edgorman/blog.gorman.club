package entity

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// MaxCommentLength bounds one comment. It is far below MaxContentLength because a comment is a
	// reply to a post rather than a post of its own: somebody with more than this to say has
	// written something that belongs under their own title.
	MaxCommentLength = 2_000
	// MaxCommentIDLength bounds the id a store may assign. Nothing here generates one, so this is
	// a bound on what the repository hands back and on what arrives in a URL, not a target.
	MaxCommentIDLength = 64
)

// commentIDPattern is what an id has to look like to be addressable and storable: exactly the
// alphabet Firestore's own auto-generated ids are drawn from, which is also the narrowest thing
// that can carry one. Being letters and digits alone, it excludes every value Firestore refuses
// inside a document key ("." and "..", anything holding a slash, and the "__reserved__" form),
// exactly as blogSlugPattern does for a post.
var commentIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// Comment is one reader's reply to a post.
//
// Unlike a post it has no name of its own to be addressed by: a title is what makes a slug, and a
// comment has no title. So its id is assigned by the store that writes it (see
// repository.CommentRepository.Create) rather than derived from anything here, and is only ever
// meaningful beneath the post it hangs off - the pair (BlogSlug, ID) is what addresses one.
//
// A comment is never edited, only written and removed. That is a deliberate limit rather than a
// missing feature: a reply somebody has already read and answered should not be able to become a
// different reply afterwards, and "delete and say it again" leaves the thread honest about what
// happened.
type Comment struct {
	ID       string `json:"id"`
	BlogSlug string `json:"blogSlug"`
	// AuthorID is the uid of whoever wrote it, which is also how the delete rule below recognises
	// them. It is carried in the response for the same reason a post carries OwnerID: a client has
	// to know whose comment it is looking at to know whether to offer a delete button.
	AuthorID  string    `json:"authorId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

// NewComment builds a validated comment on blogSlug, stamping the time it was written. The id is
// left empty for the repository to assign.
func NewComment(blogSlug, authorID, body string) (Comment, error) {
	comment := Comment{BlogSlug: blogSlug, AuthorID: authorID, CreatedAt: time.Now().UTC()}
	if err := comment.SetBody(body); err != nil {
		return Comment{}, err
	}
	if err := comment.Validate(); err != nil {
		return Comment{}, err
	}
	return comment, nil
}

// SetID trims and validates an id before applying it. Like Blog.SetSlug it guards what a store
// assigned and what arrives in a URL rather than anything a request body holds - a client never
// chooses a comment's id.
func (c *Comment) SetID(id string) error {
	trimmed := strings.TrimSpace(id)
	switch {
	case trimmed == "":
		return ValidationError{Field: "id", Message: "is required"}
	case len(trimmed) > MaxCommentIDLength:
		return ValidationError{Field: "id", Message: lengthMessage(MaxCommentIDLength)}
	case !commentIDPattern.MatchString(trimmed):
		return ValidationError{Field: "id", Message: "must be letters and digits"}
	}

	c.ID = trimmed
	return nil
}

// SetBody trims and validates a new body before applying it. Unlike a post's content, whitespace
// is not significant here: a comment is a paragraph or two of prose, and one made entirely of
// blank lines is an empty comment rather than a formatted one.
func (c *Comment) SetBody(body string) error {
	trimmed := strings.TrimSpace(body)
	switch {
	case trimmed == "":
		return ValidationError{Field: "body", Message: "is required"}
	case utf8.RuneCountInString(trimmed) > MaxCommentLength:
		return ValidationError{Field: "body", Message: lengthMessage(MaxCommentLength)}
	}

	c.Body = trimmed
	return nil
}

// Validate reports whether the comment is in a storable state: it names the post it is on and the
// caller who wrote it, and its body is one SetBody would accept. The id is checked only when it is
// present, since a comment on its way to being created does not have one yet.
func (c Comment) Validate() error {
	// A comment lives beneath its post, so its slug has to satisfy exactly the rule that makes a
	// slug usable as a document key - checked by borrowing the post's own setter rather than by
	// restating it, as Chat.Validate does.
	var post Blog
	if err := post.SetSlug(c.BlogSlug); err != nil {
		return err
	}
	if c.AuthorID == "" {
		return ValidationError{Field: "authorId", Message: "is required"}
	}

	candidate := c
	if c.ID != "" {
		if err := candidate.SetID(c.ID); err != nil {
			return err
		}
	}
	return candidate.SetBody(c.Body)
}

// Permission is the single definition of who may do what to a comment, and it needs the post
// because one of the rules is about the post: deleting a comment is a whitelist of exactly one
// name beside its author, the owner of the post it sits under. That second name is what makes
// deletion moderation rather than only retraction - an author is answerable for what appears
// beneath their post, so they can take a comment down without being able to write one in somebody
// else's name or edit what was said.
//
// The zero uid is nobody under either mode: an anonymous reader deletes nothing, whatever the
// comment holds.
//
// Reading is public here, and stays a smaller claim than it sounds: whether the thread is reachable
// at all is the post's own read permission, asked first (see the service's comment routes). Being
// public means a comment holds nothing further back from a reader who got that far.
func (c Comment) Permission(action Action, post Blog) Permission {
	permission := PermissionFor(ResourceComment, action)
	permission.OwnerID = c.AuthorID
	if action == ActionDelete && post.OwnerID != "" {
		permission.AllowedUserIDs = []string{post.OwnerID}
	}
	return permission
}
