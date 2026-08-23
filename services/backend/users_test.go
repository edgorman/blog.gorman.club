package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

func (s *fakeUserStore) Set(ctx context.Context, user User) error {
	s.users[user.ID] = user
	return nil
}

func withUID(req *http.Request, uid string) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), uidContextKey, uid))
}

func TestUserHandler_Get_NotFound(t *testing.T) {
	h := newUserHandler(newFakeUserStore())

	req := httptest.NewRequest(http.MethodGet, "/users/missing", nil)
	req.SetPathValue("id", "missing")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
}

func TestUserHandler_Get_Found(t *testing.T) {
	store := newFakeUserStore()
	store.users["user-1"] = User{ID: "user-1", DisplayName: "Ed"}
	h := newUserHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/users/user-1", nil)
	req.SetPathValue("id", "user-1")
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.DisplayName != "Ed" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Ed")
	}
}

func TestUserHandler_Put_Forbidden(t *testing.T) {
	h := newUserHandler(newFakeUserStore())

	body, _ := json.Marshal(User{DisplayName: "Someone Else"})
	req := httptest.NewRequest(http.MethodPut, "/users/user-1", bytes.NewReader(body))
	req.SetPathValue("id", "user-1")
	req = withUID(req, "different-user")
	rec := httptest.NewRecorder()
	h.Put(rec, req)

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
}

func TestUserHandler_Put_Own(t *testing.T) {
	store := newFakeUserStore()
	h := newUserHandler(store)

	body, _ := json.Marshal(User{DisplayName: "Ed"})
	req := httptest.NewRequest(http.MethodPut, "/users/user-1", bytes.NewReader(body))
	req.SetPathValue("id", "user-1")
	req = withUID(req, "user-1")
	rec := httptest.NewRecorder()
	h.Put(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if stored := store.users["user-1"]; stored.DisplayName != "Ed" {
		t.Errorf("stored DisplayName = %q, want %q", stored.DisplayName, "Ed")
	}
}
