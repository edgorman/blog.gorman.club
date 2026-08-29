package entity

import "strings"

// Draft is the part of a post the writing assistant is allowed to rewrite: its title and its body,
// and nothing else. Visibility, the whitelist, the owner, and the slug are deliberately absent, so
// a model that goes off the rails can only produce a badly written post - never publish a private
// one, hand it to somebody else, or move it.
//
// It is a value rather than a pointer to the post it came from: the assistant edits a copy over as
// many tool calls as it takes, and only what it leaves behind is written back (see ApplyTo). A run
// that fails halfway therefore changes nothing.
type Draft struct {
	Title   string
	Content string
}

// DraftOf takes the editable part of a stored post.
func DraftOf(blog Blog) Draft {
	return Draft{Title: blog.Title, Content: blog.Content}
}

// SetTitle validates a new title and applies it. The rule is Blog.SetTitle's, run against a
// throwaway post rather than restated here, so a draft can never hold a title the post it will be
// written back to would reject.
func (d *Draft) SetTitle(title string) error {
	var blog Blog
	if err := blog.SetTitle(title); err != nil {
		return err
	}

	d.Title = blog.Title
	return nil
}

// SetContent validates new content and applies it, borrowing Blog.SetContent for the same reason
// SetTitle borrows Blog.SetTitle.
func (d *Draft) SetContent(content string) error {
	var blog Blog
	if err := blog.SetContent(content); err != nil {
		return err
	}

	d.Content = blog.Content
	return nil
}

// ReplaceText swaps every occurrence of find for replace. It is what lets the assistant fix a typo
// or reword a sentence without echoing the whole post back through SetContent, which for a long
// post is most of what a request costs.
//
// A find that appears nowhere is a ValidationError rather than a silent no-op: the caller is a
// model working from a copy of the post, and quietly succeeding at a replacement that did not
// happen is exactly the failure it cannot detect for itself.
func (d *Draft) ReplaceText(find, replace string) error {
	if find == "" {
		return ValidationError{Field: "find", Message: "is required"}
	}
	if !strings.Contains(d.Content, find) {
		return ValidationError{Field: "find", Message: "does not appear in the post"}
	}

	return d.SetContent(strings.ReplaceAll(d.Content, find, replace))
}

// ApplyTo writes the draft back onto a post through the post's own setters, leaving every other
// field - slug, owner, visibility, whitelist, timestamps - as it was. It fails without touching
// blog if either value would be rejected.
func (d Draft) ApplyTo(blog *Blog) error {
	candidate := *blog
	if err := candidate.SetTitle(d.Title); err != nil {
		return err
	}
	if err := candidate.SetContent(d.Content); err != nil {
		return err
	}

	*blog = candidate
	return nil
}
