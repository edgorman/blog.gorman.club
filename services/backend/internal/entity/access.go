package entity

import "slices"

// This file is the whole of who may do what. Every gate in the service is one question asked of
// it - "may this caller take this action on this thing?" - rather than a rule of its own invented
// where it happens to be needed, which is what the assistant's ad hoc email check used to be.
//
// The model has three parts and no more:
//
//   - A Resource is a kind of thing this service holds (a post, a profile, a comment).
//   - An Action is one thing that can be done to it (read it, create one, update it, delete it).
//   - An Access is how wide the audience for that pair is: public, private, or a whitelist.
//
// The policy table below declares an Access for every (Resource, Action) pair that exists. A pair
// it does not name has no audience at all and is refused, so a feature added without a line here
// is closed rather than open.
//
// Two things are deliberately *not* here. There are no roles: a role would be a name for a set of
// permissions, and every permission below is decided by who owns the thing or who was named on it,
// which a role sits between rather than answers. And a permission is asked about one thing only -
// a comment's thread is readable by whoever may read the post above it, and that is two questions
// asked in order (see the service's comment routes), not a mode of its own.

// Resource is a kind of thing this service holds. It names what a permission is about, and is what
// makes the policy table below readable as a list of everything that can be reached.
type Resource string

const (
	ResourceUser     Resource = "user"
	ResourceBlog     Resource = "blog"
	ResourceComment  Resource = "comment"
	ResourceReaction Resource = "reaction"
	// ResourceAssistant is the AI writing assistant. It is a feature rather than something stored,
	// which is exactly why it is in the same table as the rest: a gated feature that is not a
	// resource is how the assistant ended up with a bespoke allowlist bolted onto the config.
	ResourceAssistant Resource = "assistant"
)

// Action is one thing that can be done to a resource. These four are the whole vocabulary, and
// they line up with the HTTP methods the routes are registered under - a fifth would mean the API
// grew a verb, not that this list was incomplete.
type Action string

const (
	ActionRead   Action = "read"
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// Access is how wide the audience for one action on one resource is. Three modes, which is the
// whole scale:
//
//   - Public is everybody, signed in or not.
//   - Private is the owner of the thing and nobody else.
//   - Whitelist is the owner plus whoever was named alongside them.
//
// Private is not a special case of a whitelist with an empty list, and is kept separate on
// purpose: a mode says what a thing *is*, so a post that names no readers is private rather than
// whitelisted-to-nobody, and one that names some is a whitelist however few they are.
type Access string

const (
	AccessPublic    Access = "public"
	AccessPrivate   Access = "private"
	AccessWhitelist Access = "whitelist"
)

// Valid reports whether the mode is one of the three. The zero Access is not: it is what an
// undeclared (resource, action) pair carries, and it denies.
func (a Access) Valid() bool {
	return a == AccessPublic || a == AccessPrivate || a == AccessWhitelist
}

// policy declares the audience for every action on every resource. It is the one place to read to
// know what this service allows, and the one place to change to alter it.
//
// Several entries are private for a caller who could only ever be acting on their own: a profile is
// addressed as /users/me, a reaction is keyed by the reader who left it, and a post is created
// owned by whoever asked. Those are enforced by the address rather than by a check - the forbidden
// case is unreachable, not refused - and they are declared here anyway, because a table that only
// listed the checks that happen would not say what the rules are.
var policy = map[Resource]map[Action]Access{
	// A profile is public because a username is the whole of a public identity: a post shows its
	// author by name, so a reader who cannot read profiles cannot be shown who wrote anything.
	// Writing one is the account's own business and nobody else's.
	ResourceUser: {
		ActionRead:   AccessPublic,
		ActionUpdate: AccessPrivate,
		ActionDelete: AccessPrivate,
	},
	// A post is the one resource whose read audience is not fixed here: it is whatever the post
	// itself says (see Blog.Permission), which is what visibility and the whitelist on a post
	// have always meant. The entry below is the widest a post can be, not what every post is.
	ResourceBlog: {
		ActionRead:   AccessPublic,
		ActionCreate: AccessPrivate,
		ActionUpdate: AccessPrivate,
		ActionDelete: AccessPrivate,
	},
	// A comment is public to read - as public as the post it hangs under, which is asked first -
	// and deleted by a whitelist of exactly one beside its author: the owner of that post. That
	// second name is what makes deletion moderation rather than only retraction.
	ResourceComment: {
		ActionRead:   AccessPublic,
		ActionCreate: AccessPrivate,
		ActionDelete: AccessWhitelist,
	},
	// A reaction is public to read as a count, and private to write: the row a reader writes is
	// keyed by that reader, so there is no other reader's reaction to reach.
	ResourceReaction: {
		ActionRead:   AccessPublic,
		ActionCreate: AccessPrivate,
		ActionDelete: AccessPrivate,
	},
	// The assistant is a whitelist whose membership is bought rather than stored on a document:
	// an account is on it while its subscription has not run out (see AssistantEntitlement). Read,
	// update, and delete are the three things done to a conversation, and all three cost the same
	// entitlement - a transcript is as much a paid artifact as the turn that produced it.
	ResourceAssistant: {
		ActionRead:   AccessWhitelist,
		ActionUpdate: AccessWhitelist,
		ActionDelete: AccessWhitelist,
	},
}

// Permission is one question about access, ready to be answered: the audience an action on a
// resource has, narrowed to a particular thing by who owns it and who was named on it.
//
// It carries the resource and action it came from so that a refusal can say what was refused, and
// so a permission handed around is self-describing rather than a bare bool with a comment.
type Permission struct {
	Resource Resource
	Action   Action
	Access   Access
	// OwnerID is the uid the thing belongs to - the post's author, the profile's account, the
	// commenter. It is empty for a resource with no owner, which only a public action can have.
	OwnerID string
	// AllowedUserIDs are the uids named alongside the owner. It is read only under
	// AccessWhitelist, so a list left on a private permission grants nothing.
	AllowedUserIDs []string
}

// PermissionFor returns the declared permission for an action on a resource, before it has been
// narrowed to any particular thing. An undeclared pair comes back with the zero Access, which
// Allows refuses - so reaching for a permission this service does not define denies rather than
// panics or defaults open.
func PermissionFor(resource Resource, action Action) Permission {
	return Permission{Resource: resource, Action: action, Access: policy[resource][action]}
}

// Allows reports whether uid may do the thing this permission describes.
//
// The zero uid is the anonymous caller, and it holds nothing but public access however the rest of
// the permission is filled in: an empty OwnerID must not make a signed-out request the owner of an
// ownerless thing, which is the one way this could have failed open.
func (p Permission) Allows(uid string) bool {
	switch p.Access {
	case AccessPublic:
		return true
	case AccessPrivate:
		return uid != "" && uid == p.OwnerID
	case AccessWhitelist:
		return uid != "" && (uid == p.OwnerID || slices.Contains(p.AllowedUserIDs, uid))
	default:
		return false
	}
}
