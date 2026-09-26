package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

const commentStranger = "stranger"

func (f *commentFixture) approve(uid, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPut, "/blogs/"+commentSlug+"/comments/"+id+"/moderation", nil)
	req.SetPathValue("slug", commentSlug)
	req.SetPathValue("id", id)
	if uid != "" {
		req = withUID(req, uid)
	}

	rec := httptest.NewRecorder()
	f.service.ApproveComment(rec, req)
	return rec
}

// seedFlagged stores one approved and one flagged comment by commentReader on the fixture's post.
func (f *commentFixture) seedFlagged() {
	at := time.Now().UTC()
	f.comments.seed(
		entity.Comment{ID: "ok", BlogSlug: commentSlug, AuthorID: commentReader, Body: "nicely put", CreatedAt: at,
			Moderation: &entity.Moderation{Status: entity.ModerationApproved, Category: "none", At: at}},
		entity.Comment{ID: "spam", BlogSlug: commentSlug, AuthorID: commentReader, Body: "buy now", CreatedAt: at,
			Moderation: &entity.Moderation{Status: entity.ModerationFlagged, Category: "spam", At: at}},
	)
}

func TestListComments_Moderation(t *testing.T) {
	f := newCommentFixture(t)
	f.seedFlagged()

	for _, tt := range []struct {
		name      string
		uid       string
		wantIDs   []string
		wantMarks bool
	}{
		{"anonymous readers do not see a flagged comment", "", []string{"ok"}, false},
		{"other readers do not see a flagged comment", commentStranger, []string{"ok"}, false},
		{"its author sees it, unmarked", commentReader, []string{"ok", "spam"}, false},
		{"the post's owner sees it, marked", commentAuthor, []string{"ok", "spam"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := f.list(tt.uid)
			assertStatus(t, rec, http.StatusOK)

			comments := decodeComments(t, rec)
			if len(comments) != len(tt.wantIDs) {
				t.Fatalf("listed %d comments, want %v", len(comments), tt.wantIDs)
			}
			for i, comment := range comments {
				if comment.ID != tt.wantIDs[i] {
					t.Errorf("comment %d = %q, want %q", i, comment.ID, tt.wantIDs[i])
				}
				if marked := comment.Moderation != nil; marked != tt.wantMarks {
					t.Errorf("comment %q carries moderation = %v, want %v", comment.ID, marked, tt.wantMarks)
				}
			}
			if tt.wantMarks {
				if m := comments[1].Moderation; m.Status != "flagged" || m.Category != "spam" {
					t.Errorf("flagged comment's moderation = %+v, want flagged/spam", m)
				}
			}
		})
	}
}

func TestApproveComment(t *testing.T) {
	f := newCommentFixture(t)
	f.seedFlagged()

	// Neither the comment's author nor another reader may overrule the classifier.
	assertStatus(t, f.approve(commentReader, "spam"), http.StatusForbidden)
	assertStatus(t, f.approve(commentStranger, "spam"), http.StatusForbidden)
	if len(decodeComments(t, f.list(commentStranger))) != 1 {
		t.Fatal("a refused approval published the comment anyway")
	}

	rec := f.approve(commentAuthor, "spam")
	assertStatus(t, rec, http.StatusOK)
	if m := decodeComment(t, rec).Moderation; m == nil || m.Status != "approved" || m.Category != "spam" {
		t.Errorf("approved comment's moderation = %+v, want approved, keeping its category", m)
	}
	if got := len(decodeComments(t, f.list(""))); got != 2 {
		t.Errorf("anonymous readers see %d comments after approval, want 2", got)
	}

	assertStatus(t, f.approve(commentAuthor, "missing"), http.StatusNotFound)
}

// The route is declared in the policy table: approving is private to the post's owner.
func TestModerationPolicy(t *testing.T) {
	post := entity.Blog{OwnerID: commentAuthor}
	comment := entity.Comment{AuthorID: commentReader}
	for _, action := range []entity.Action{entity.ActionRead, entity.ActionUpdate} {
		permission := comment.ModerationPermission(action, post)
		if permission.Access != entity.AccessPrivate {
			t.Errorf("moderation %s access = %q, want private", action, permission.Access)
		}
		if !permission.Allows(commentAuthor) || permission.Allows(commentReader) || permission.Allows("") {
			t.Errorf("moderation %s should allow the post's owner alone", action)
		}
	}
}
