package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/edgorman/blog.gorman.club/services/backend/internal/entity"
)

// userHTTPRequest sets the {id} path value and caller uid explicitly, to cover owner and non-owner.
func userHTTPRequest(method, id, uid string, body []byte) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, "/users/"+id, nil)
	} else {
		req = httptest.NewRequest(method, "/users/"+id, bytes.NewReader(body))
	}
	req.SetPathValue("id", id)
	return withUID(req, uid)
}

func userRequestBody(t *testing.T, body userRequest) []byte {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return encoded
}

func TestGetUser_AnySignedInCaller(t *testing.T) {
	repo := newFakeUserRepository()
	repo.users["someone"] = entity.User{ID: "someone", DisplayName: "Someone"}
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.GetUser(rec, userHTTPRequest(http.MethodGet, "someone", "a-different-caller", nil))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got entity.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.DisplayName != "Someone" {
		t.Errorf("DisplayName = %q, want %q", got.DisplayName, "Someone")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	s := newTestService(nil, nil)

	rec := httptest.NewRecorder()
	s.GetUser(rec, userHTTPRequest(http.MethodGet, "missing", "caller", nil))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

func TestPutUser_CreatesOwnProfile(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "Ed", Bio: "hello"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, userHTTPRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	stored := repo.users["caller"]
	if stored.DisplayName != "Ed" {
		t.Errorf("DisplayName = %q, want %q", stored.DisplayName, "Ed")
	}
	if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() {
		t.Error("timestamps must be stamped by the server")
	}
}

// A client trying to backdate its profile must not be able to: createdAt isn't part of
// userRequest, so it survives from the stored profile.
func TestPutUser_UpdatePreservesCreatedAt(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := newFakeUserRepository()
	repo.users["caller"] = entity.User{ID: "caller", DisplayName: "Ed", CreatedAt: created}
	s := newTestService(nil, repo)

	body := []byte(`{"displayName":"Edward","createdAt":"1999-01-01T00:00:00Z"}`)
	rec := httptest.NewRecorder()
	s.PutUser(rec, userHTTPRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (existing profile)", rec.Result().StatusCode, http.StatusOK)
	}
	stored := repo.users["caller"]
	if !stored.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt = %v, want %v (must be carried over, not taken from the body)", stored.CreatedAt, created)
	}
	if stored.DisplayName != "Edward" {
		t.Errorf("DisplayName = %q, want %q", stored.DisplayName, "Edward")
	}
}

func TestPutUser_ForbiddenForOtherProfile(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "Impostor"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, userHTTPRequest(http.MethodPut, "someone-else", "caller", body))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := repo.users["someone-else"]; ok {
		t.Error("profile was written despite forbidden caller")
	}
}

func TestPutUser_RejectsEmptyDisplayName(t *testing.T) {
	s := newTestService(nil, nil)

	body := userRequestBody(t, userRequest{DisplayName: "   "})
	rec := httptest.NewRecorder()
	s.PutUser(rec, userHTTPRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if got := decodeAPIError(t, rec).Error; !strings.Contains(got, "displayName") {
		t.Errorf("error = %q, want it to name the displayName field", got)
	}
}

func TestPutUser_RejectsOverlongBio(t *testing.T) {
	s := newTestService(nil, nil)

	body := userRequestBody(t, userRequest{DisplayName: "Ed", Bio: strings.Repeat("a", entity.MaxBioLength+1)})
	rec := httptest.NewRecorder()
	s.PutUser(rec, userHTTPRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if got := decodeAPIError(t, rec).Error; !strings.Contains(got, "bio") {
		t.Errorf("error = %q, want it to name the bio field", got)
	}
}

// The entity setters trim, so a padded name is stored clean rather than rejected.
func TestPutUser_TrimsDisplayName(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "  Ed  "})
	rec := httptest.NewRecorder()
	s.PutUser(rec, userHTTPRequest(http.MethodPut, "caller", "caller", body))

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	if got := repo.users["caller"].DisplayName; got != "Ed" {
		t.Errorf("DisplayName = %q, want %q", got, "Ed")
	}
}

func TestDeleteUser_Owner(t *testing.T) {
	repo := newFakeUserRepository()
	repo.users["caller"] = entity.User{ID: "caller", DisplayName: "Ed"}
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, userHTTPRequest(http.MethodDelete, "caller", "caller", nil))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, ok := repo.users["caller"]; ok {
		t.Error("profile still present after delete")
	}
}

func TestDeleteUser_ForbiddenForOtherProfile(t *testing.T) {
	repo := newFakeUserRepository()
	repo.users["someone-else"] = entity.User{ID: "someone-else", DisplayName: "Someone"}
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, userHTTPRequest(http.MethodDelete, "someone-else", "caller", nil))

	if rec.Result().StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusForbidden)
	}
	decodeAPIError(t, rec)
	if _, ok := repo.users["someone-else"]; !ok {
		t.Error("profile was deleted despite forbidden caller")
	}
}

// Firestore deletes are idempotent, so the handler looks the profile up to report a real 404.
func TestDeleteUser_NotFound(t *testing.T) {
	s := newTestService(nil, nil)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, userHTTPRequest(http.MethodDelete, "caller", "caller", nil))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}
