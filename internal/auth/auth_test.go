package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if !LooksHashedPassword(hash) {
		t.Fatalf("hash = %q", hash)
	}
	if strings.Contains(hash, "correct-horse") {
		t.Fatal("plaintext leaked into hash")
	}
	if !VerifyPassword("correct-horse", hash) {
		t.Fatal("verify true")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("verify false")
	}
}

func TestAPIKeyHashNotRaw(t *testing.T) {
	raw, prefix, hash, err := GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, APIKeyPrefix) {
		t.Fatalf("raw = %q", raw)
	}
	if hash == raw || strings.Contains(hash, raw) {
		t.Fatal("raw key stored as hash")
	}
	if prefix == raw {
		t.Fatal("prefix should not be the full secret")
	}
	if HashAPIKey(raw) != hash {
		t.Fatal("hash mismatch")
	}
	if HashAPIKey("pl_live_other") == hash {
		t.Fatal("different keys collided")
	}
}

func TestRBAC(t *testing.T) {
	if !HasPermission(RoleOwner, PermMembersManage) {
		t.Fatal("owner members")
	}
	if HasPermission(RoleAdmin, PermMembersManage) {
		t.Fatal("admin should not manage members")
	}
	if !HasPermission(RoleAdmin, PermAPIKeysManage) {
		t.Fatal("admin keys")
	}
	if HasPermission(RoleMember, PermAPIKeysManage) {
		t.Fatal("member keys")
	}
	if !HasPermission(RoleViewer, PermLogsRead) {
		t.Fatal("viewer read")
	}
	if HasPermission(RoleViewer, PermServicesManage) {
		t.Fatal("viewer services")
	}
}

func TestJWTRoundTripAndDeny(t *testing.T) {
	iss, err := NewIssuer("test-secret-not-for-prod", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	tok, jti, _, err := iss.Issue(id, "a@example.com", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c, err := iss.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if c.Subject != id.String() || c.ID != jti {
		t.Fatalf("%+v", c)
	}
	if _, err := iss.Parse("nope"); err == nil {
		t.Fatal("bad token")
	}
	deny := NewMemoryDeny()
	if err := deny.Deny(context.Background(), jti, time.Hour); err != nil {
		t.Fatal(err)
	}
	ok, err := deny.Denied(context.Background(), jti)
	if err != nil || !ok {
		t.Fatal("denied")
	}
}
