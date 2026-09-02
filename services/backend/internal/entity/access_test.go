package entity

import "testing"

// The three modes are the whole scale, and each one admits exactly who it says it does. The zero
// uid is the anonymous caller and holds nothing but public access, which is the one way this could
// have failed open: an ownerless thing must not make a signed-out request its owner.
func TestPermission_Allows(t *testing.T) {
	for _, tt := range []struct {
		name       string
		permission Permission
		uid        string
		want       bool
	}{
		{"public admits a stranger", Permission{Access: AccessPublic, OwnerID: "owner"}, "stranger", true},
		{"public admits nobody at all", Permission{Access: AccessPublic, OwnerID: "owner"}, "", true},
		{"private admits its owner", Permission{Access: AccessPrivate, OwnerID: "owner"}, "owner", true},
		{"private refuses a stranger", Permission{Access: AccessPrivate, OwnerID: "owner"}, "stranger", false},
		{"private refuses the anonymous caller", Permission{Access: AccessPrivate}, "", false},
		{
			"a whitelist admits its owner",
			Permission{Access: AccessWhitelist, OwnerID: "owner", AllowedUserIDs: []string{"friend"}},
			"owner", true,
		},
		{
			"a whitelist admits a named uid",
			Permission{Access: AccessWhitelist, OwnerID: "owner", AllowedUserIDs: []string{"friend"}},
			"friend", true,
		},
		{
			"a whitelist refuses everybody else",
			Permission{Access: AccessWhitelist, OwnerID: "owner", AllowedUserIDs: []string{"friend"}},
			"stranger", false,
		},
		{
			"a whitelist naming nobody refuses the anonymous caller",
			Permission{Access: AccessWhitelist, AllowedUserIDs: []string{""}},
			"", false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.permission.Allows(tt.uid); got != tt.want {
				t.Errorf("Allows(%q) = %v, want %v", tt.uid, got, tt.want)
			}
		})
	}
}

// A pair the policy does not declare has no audience, so it denies rather than defaulting open.
// That is what makes adding a resource without a line in the table a closed feature rather than a
// hole, and it holds for a caller who owns the thing as much as for a stranger.
func TestPermissionFor_UndeclaredPairsDenyEverybody(t *testing.T) {
	permission := PermissionFor(ResourceComment, ActionUpdate)
	permission.OwnerID = "owner"

	if permission.Access.Valid() {
		t.Fatalf("Access = %q, want an undeclared pair to carry none", permission.Access)
	}
	for _, uid := range []string{"owner", "stranger", ""} {
		if permission.Allows(uid) {
			t.Errorf("Allows(%q) = true, want an undeclared pair to allow nobody", uid)
		}
	}

	if _, declared := policy[Resource("nothing")]; declared {
		t.Error("policy declares a resource that does not exist")
	}
	if PermissionFor(Resource("nothing"), ActionRead).Allows("owner") {
		t.Error("an unknown resource allows somebody, want nobody")
	}
}

// Every pair the table does declare carries a mode Allows understands. A typo here would be a
// silently closed route rather than a compile error, so it is checked rather than trusted.
func TestPolicyDeclaresValidAccess(t *testing.T) {
	for resource, actions := range policy {
		if len(actions) == 0 {
			t.Errorf("%q declares no actions", resource)
		}
		for action, access := range actions {
			if !access.Valid() {
				t.Errorf("%q/%q is %q, which is not a mode", resource, action, access)
			}
		}
	}
}
