package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeUserStore struct {
	users map[string]User
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{users: make(map[string]User)}
}

func (s *fakeUserStore) Get(ctx context.Context, id string) (User, error) {
	user, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (s *fakeUserStore) Put(ctx context.Context, user User) (User, error) {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	s.users[user.ID] = user
	return user, nil
}

func (s *fakeUserStore) Delete(ctx context.Context, id string) error {
	delete(s.users, id)
	return nil
}

// userRequest sets the {id} path value and caller uid explicitly, to cover owner and non-owner.
func userRequest(method, id, uid string, body []byte) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, "/users/"+id, nil)
	} else {
		req = httptest.NewRequest(method, "/users/"+id, bytes.NewReader(body))
	}
	req.SetPathValue("id", id)
	return withUID(req, uid)
}

func TestUserHandler_Get_AnySignedInCaller(t *testing.T) {
	store := newFakeUserStore()
	store.users["someone"] = User{ID: "someone", DisplayName: "Someone"}
	h := newUserHandler(store)

	rec := httptest.NewRecorder()
	h.Get(rec, userRequest(http.MethodGet, "someone", "a-different-caller", nil))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.DisplayName != "Someone" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Someone")
	}
}

func TestUserHandler_Get_NotFound(t *testing.T) {
	h := newUserHandler(newFakeUserStore())

	rec := httptest.NewRecorder()
	h.Get(rec, userRequest(http.MethodGet, "missing", "caller", nil))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

func TestUserHandler_Put_CreatesOwnProfile(t *testing.T) {
	store := newFakeUserStore()
	h := newUserHandler(store)

	body, _ := json.Marshal(User{DisplayName: "Ed", Bio: "hello"})
	rec := httptest.NewRecorder()
	h.Put(rec, userRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	stored := store.users["caller"]
	if stored.DisplayName != "Ed" {
		t.Errorf("DisplayName = %q, want %q", stored.DisplayName, "Ed")
	}
	if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
		t.Error("timestamps must be stamped by the server")
	}
}

func TestUserHandler_Put_UpdatePreservesCreatedAt(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := newFakeUserStore()
	store.users["caller"] = User{ID: "caller", DisplayName: "Ed", CreatedAt: created}
	h := newUserHandler(store)

	// A client trying to backdate its profile must not be able to.
	body, _ := json.Marshal(User{DisplayName: "Edward", CreatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)})
	rec := httptest.NewRecorder()
	h.Put(rec, userRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (existing profile)", rec.Result().StatusCode, http.StatusOK)
	}
	stored := store.users["caller"]
	if !stored.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v (must be carried over, not taken from the body)", stored.CreatedAt, created)
	}
	if stored.DisplayName != "Edward" {
		t.Errorf("DisplayName = %q, want %q", stored.DisplayName, "Edward")
	}
}

func TestUserHandler_Put_ForbiddenForOtherProfile(t *testing.T) {
	store := newFakeUserStore()
	h := newUserHandler(store)

	body, _ := json.Marshal(User{DisplayName: "Impostor"})
	rec := httptest.NewRecorder()
	h.Put(rec, userRequest(http.MethodPut, "someone-else", "caller", body))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := store.users["someone-else"]; ok {
		t.Error("profile was written despite forbidden caller")
	}
}

func TestUserHandler_Put_RejectsEmptyDisplayName(t *testing.T) {
	h := newUserHandler(newFakeUserStore())

	body, _ := json.Marshal(User{DisplayName: "   "})
	rec := httptest.NewRecorder()
	h.Put(rec, userRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	decodeAPIError(t, rec)
}

func TestUserHandler_Delete_Owner(t *testing.T) {
	store := newFakeUserStore()
	store.users["caller"] = User{ID: "caller", DisplayName: "Ed"}
	h := newUserHandler(store)

	rec := httptest.NewRecorder()
	h.Delete(rec, userRequest(http.MethodDelete, "caller", "caller", nil))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, ok := store.users["caller"]; ok {
		t.Error("profile still present after delete")
	}
}

func TestUserHandler_Delete_ForbiddenForOtherProfile(t *testing.T) {
	store := newFakeUserStore()
	store.users["someone-else"] = User{ID: "someone-else", DisplayName: "Someone"}
	h := newUserHandler(store)

	rec := httptest.NewRecorder()
	h.Delete(rec, userRequest(http.MethodDelete, "someone-else", "caller", nil))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := store.users["someone-else"]; !ok {
		t.Error("profile was deleted despite forbidden caller")
	}
}

// Firestore deletes are idempotent, so the handler looks the profile up to report a real 404.
func TestUserHandler_Delete_NotFound(t *testing.T) {
	h := newUserHandler(newFakeUserStore())

	rec := httptest.NewRecorder()
	h.Delete(rec, userRequest(http.MethodDelete, "caller", "caller", nil))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}
