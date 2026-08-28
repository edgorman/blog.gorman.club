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
	"github.com/edgorman/blog.gorman.club/services/backend/internal/repository"
)

// selfHTTPRequest builds a request against /users/me carrying uid as the verified caller. There is
// no path value to set: the handler takes the owner from the credential, which is the whole point
// of addressing writes this way.
func selfHTTPRequest(method, uid string, body []byte) *http.Request {
	var req *http.Request
	if body == nil {
		req = httptest.NewRequest(method, "/users/me", nil)
	} else {
		req = httptest.NewRequest(method, "/users/me", bytes.NewReader(body))
	}
	return withUID(req, uid)
}

// usernameHTTPRequest sets the {username} path value the way the router would.
func usernameHTTPRequest(username string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/users/"+username, nil)
	req.SetPathValue("username", username)
	return req
}

func userRequestBody(t *testing.T, body userRequest) []byte {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return encoded
}

func TestPutUser_CreatesOwnProfile(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "Ed", Bio: "hello"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

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
	repo.seed(entity.User{ID: "caller", Username: "sly-dancing-monkey", DisplayName: "Ed", CreatedAt: created})
	s := newTestService(nil, repo)

	body := []byte(`{"displayName":"Edward","createdAt":"1999-01-01T00:00:00Z"}`)
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

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

// There is no id in the path to forge, so a write can only ever land on the caller's own profile.
// This pins that: two different callers writing the same body get two separate profiles, and
// neither can name the other.
func TestPutUser_AlwaysWritesTheCallersOwnProfile(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone-else", Username: "bold-leaping-lynx", DisplayName: "Someone"})
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "Impostor"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	if got := repo.users["caller"].DisplayName; got != "Impostor" {
		t.Errorf("caller's DisplayName = %q, want the write to have landed on the caller", got)
	}
	if got := repo.users["someone-else"].DisplayName; got != "Someone" {
		t.Errorf("someone-else's DisplayName = %q, want it untouched", got)
	}
}

func TestPutUser_RejectsEmptyDisplayName(t *testing.T) {
	s := newTestService(nil, nil)

	body := userRequestBody(t, userRequest{DisplayName: "   "})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

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
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

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
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}
	if got := repo.users["caller"].DisplayName; got != "Ed" {
		t.Errorf("DisplayName = %q, want %q", got, "Ed")
	}
}

// The body is decoded before the profile is looked up, so malformed input costs no datastore read.
func TestPutUser_MalformedBodySkipsTheLookup(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", []byte("not json")))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if repo.gets != 0 {
		t.Errorf("repository Get calls = %d, want 0 for a malformed body", repo.gets)
	}
}

func TestDeleteUser_Owner(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "caller", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, selfHTTPRequest(http.MethodDelete, "caller", nil))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, ok := repo.users["caller"]; ok {
		t.Error("profile still present after delete")
	}
}

// The delete counterpart: the caller's credential picks the target, so another profile is never
// reachable however the request is shaped.
func TestDeleteUser_AlwaysDeletesTheCallersOwnProfile(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone-else", Username: "bold-leaping-lynx", DisplayName: "Someone"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, selfHTTPRequest(http.MethodDelete, "caller", nil))

	// The caller has no profile of its own, so its own delete is the 404 - someone-else survives.
	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	if _, ok := repo.users["someone-else"]; !ok {
		t.Error("another caller's profile was deleted")
	}
}

// Firestore deletes are idempotent, so the handler looks the profile up to report a real 404.
func TestDeleteUser_NotFound(t *testing.T) {
	s := newTestService(nil, nil)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, selfHTTPRequest(http.MethodDelete, "caller", nil))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

// A profile is named by the server at sign-up: the client sends no username and gets a three-word
// one back.
func TestPutUser_AssignsAGeneratedUsername(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "Ed"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusCreated)
	}

	stored := repo.users["caller"]
	if strings.Count(stored.Username, "-") != 2 {
		t.Errorf("Username = %q, want a generated three-word name", stored.Username)
	}
	var candidate entity.User
	if err := candidate.SetUsername(stored.Username); err != nil {
		t.Errorf("stored username %q is not valid: %v", stored.Username, err)
	}

	// The name is reserved as part of the write, so the profile is immediately findable by it.
	if repo.usernames[stored.UsernameKey()] != "caller" {
		t.Error("generated username was not reserved for the profile")
	}

	var got entity.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Username != stored.Username {
		t.Errorf("response Username = %q, want the stored %q", got.Username, stored.Username)
	}
}

// The generator can collide with a name already taken, so the handler draws again rather than
// failing the sign-up.
func TestPutUser_RetriesWhenAGeneratedUsernameCollides(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	// Reject the first two draws, standing in for names already held by other users.
	rejected := 0
	repo.beforePut = func(entity.User) error {
		if rejected < 2 {
			rejected++
			return repository.ErrUsernameTaken
		}
		return nil
	}

	body := userRequestBody(t, userRequest{DisplayName: "Ed"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d after retrying past collisions", rec.Result().StatusCode, http.StatusCreated)
	}
	if rejected != 2 {
		t.Errorf("rejected draws = %d, want both to have been retried", rejected)
	}
	if _, ok := repo.users["caller"]; !ok {
		t.Error("profile was not written after the retries succeeded")
	}
}

// Giving up is better than looping forever. It is reported as a server error, not a conflict: the
// caller sent no username, so it has nothing to correct.
func TestPutUser_GivesUpAfterRepeatedCollisions(t *testing.T) {
	repo := newFakeUserRepository()
	repo.beforePut = func(entity.User) error { return repository.ErrUsernameTaken }
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "Ed"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusInternalServerError)
	}
	decodeAPIError(t, rec)
	if _, ok := repo.users["caller"]; ok {
		t.Error("profile was written despite every draw colliding")
	}
}

// A request that says nothing about the username keeps the one the profile already holds, so
// editing a bio cannot silently rename anybody.
func TestPutUser_OmittedUsernameIsUnchanged(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "caller", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{DisplayName: "Ed", Bio: "new bio"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if got := repo.users["caller"].Username; got != "sly-dancing-monkey" {
		t.Errorf("Username = %q, want it left alone", got)
	}
}

func TestPutUser_RenameReleasesTheOldUsername(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "caller", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{Username: "bold-leaping-lynx", DisplayName: "Ed"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	if got := repo.users["caller"].Username; got != "bold-leaping-lynx" {
		t.Errorf("Username = %q, want %q", got, "bold-leaping-lynx")
	}
	if _, ok := repo.usernames["sly-dancing-monkey"]; ok {
		t.Error("the old username is still reserved, so nobody else could ever claim it")
	}
	if repo.usernames["bold-leaping-lynx"] != "caller" {
		t.Error("the new username was not reserved")
	}
}

func TestPutUser_RejectsAUsernameHeldByAnotherUser(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone", Username: "sly-dancing-monkey", DisplayName: "Someone"})
	s := newTestService(nil, repo)

	body := userRequestBody(t, userRequest{Username: "Sly-Dancing-Monkey", DisplayName: "Ed"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	// Differing only in case is still the same name, since uniqueness folds case.
	if rec.Result().StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusConflict)
	}
	decodeAPIError(t, rec)
	if _, ok := repo.users["caller"]; ok {
		t.Error("profile was written despite the username being taken")
	}
}

func TestPutUser_RejectsAMalformedUsername(t *testing.T) {
	s := newTestService(nil, nil)

	body := userRequestBody(t, userRequest{Username: "sly dancing monkey", DisplayName: "Ed"})
	rec := httptest.NewRecorder()
	s.PutUser(rec, selfHTTPRequest(http.MethodPut, "caller", body))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if got := decodeAPIError(t, rec).Error; !strings.Contains(got, "username") {
		t.Errorf("error = %q, want it to name the username field", got)
	}
}

func TestGetUser(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone", Username: "sly-dancing-monkey", DisplayName: "Someone"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.GetUser(rec, usernameHTTPRequest("sly-dancing-monkey"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got entity.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "someone" {
		t.Errorf("ID = %q, want %q", got.ID, "someone")
	}
}

// Lookup folds case the same way uniqueness does, so a name typed any way finds its owner.
func TestGetUser_IgnoresCase(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone", Username: "Sly-Dancing-Monkey", DisplayName: "Someone"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.GetUser(rec, usernameHTTPRequest("sly-DANCING-monkey"))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
}

// An unclaimed name is a 404, which is also how a client asks whether one is free.
func TestGetUser_NotFound(t *testing.T) {
	s := newTestService(nil, nil)

	rec := httptest.NewRecorder()
	s.GetUser(rec, usernameHTTPRequest("sly-dancing-monkey"))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}

// A name that could never be claimed is answered with the rule it breaks, not a bare 404, and
// costs no datastore lookup.
func TestGetUser_RejectsAMalformedName(t *testing.T) {
	repo := newFakeUserRepository()
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.GetUser(rec, usernameHTTPRequest("no"))

	if rec.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusBadRequest)
	}
	if got := decodeAPIError(t, rec).Error; !strings.Contains(got, "username") {
		t.Errorf("error = %q, want it to name the username field", got)
	}
	if repo.gets != 0 {
		t.Errorf("repository lookups = %d, want 0 for a name that cannot exist", repo.gets)
	}
}

// Deleting a profile frees its username for somebody else.
func TestDeleteUser_ReleasesTheUsername(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "caller", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.DeleteUser(rec, selfHTTPRequest(http.MethodDelete, "caller", nil))

	if rec.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNoContent)
	}
	if _, ok := repo.usernames["sly-dancing-monkey"]; ok {
		t.Error("username is still reserved after the profile was deleted")
	}
}

// GET /users/me and GET /users/{username} overlap: "me" is a value the wildcard would match. This
// pins that ServeMux prefers the literal, so a client asking for its own profile is never answered
// with a lookup for a user named "me".
func TestHandler_RoutesMeAheadOfTheUsernameWildcard(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "caller", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	repo.seed(entity.User{ID: "someone", Username: "me", DisplayName: "Someone Called Me"})
	handler := newTestService(nil, repo).Handler()

	// fakeVerifier resolves any credential to uid "caller", so this reaches GetCurrentUser as the
	// signed-in caller rather than as a lookup of the profile named "me".
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	req.Header.Set(authorizationHeader, bearerPrefix+"token")
	req.Header.Set(authorizationProviderHeader, string(providerGoogle))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got entity.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "caller" {
		t.Errorf("ID = %q, want the caller's own profile, not the one named \"me\"", got.ID)
	}
}

// The wildcard still serves everybody else, anonymously.
func TestHandler_RoutesUsernameLookups(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "someone", Username: "sly-dancing-monkey", DisplayName: "Someone"})
	handler := newTestService(nil, repo).Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/sly-dancing-monkey", nil))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got entity.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "someone" {
		t.Errorf("ID = %q, want %q", got.ID, "someone")
	}
}

// A profile is never addressable by the Google sub it is keyed by.
func TestHandler_RejectsLookupByID(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "123456789", Username: "sly-dancing-monkey", DisplayName: "Someone"})
	handler := newTestService(nil, repo).Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users/123456789", nil))

	// The id matches the username wildcard, so it reaches GetUser and finds nothing claiming it.
	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
}

func TestGetCurrentUser(t *testing.T) {
	repo := newFakeUserRepository()
	repo.seed(entity.User{ID: "caller", Username: "sly-dancing-monkey", DisplayName: "Ed"})
	s := newTestService(nil, repo)

	rec := httptest.NewRecorder()
	s.GetCurrentUser(rec, selfHTTPRequest(http.MethodGet, "caller", nil))

	if rec.Result().StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusOK)
	}
	var got entity.User
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// This route is how a client learns the name it was given, so the username has to come back.
	if got.Username != "sly-dancing-monkey" {
		t.Errorf("Username = %q, want %q", got.Username, "sly-dancing-monkey")
	}
}

// A signed-in caller who has not created a profile yet gets a 404, which is how the frontend knows
// to send them through profile setup.
func TestGetCurrentUser_NotFound(t *testing.T) {
	s := newTestService(nil, nil)

	rec := httptest.NewRecorder()
	s.GetCurrentUser(rec, selfHTTPRequest(http.MethodGet, "caller", nil))

	if rec.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Result().StatusCode, http.StatusNotFound)
	}
	decodeAPIError(t, rec)
}
