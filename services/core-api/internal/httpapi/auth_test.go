package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

type fixedAuthenticator struct {
	principal Principal
	err       error
}

func TestOIDCAssurancePolicyRequiresMFAClassMethodsAndRecentAuthentication(t *testing.T) {
	now := time.Date(2026, time.August, 6, 12, 0, 0, 0, time.UTC)
	policy := OIDCAssurancePolicy{RequiredACR: "urn:itemba:loa:2", RequiredAMR: []string{"pwd", "otp"}, MaxAuthAge: 30 * time.Minute, now: func() time.Time { return now }}
	claims := oidcClaims{AssuranceClass: "urn:itemba:loa:2", AuthenticationMethods: []string{"pwd", "otp"}, AuthenticationTime: now.Add(-10 * time.Minute).Unix()}
	if err := policy.validate(); err != nil {
		t.Fatal(err)
	}
	if err := policy.validateClaims(claims); err != nil {
		t.Fatalf("valid MFA claims rejected: %v", err)
	}
	claims.AuthenticationMethods = []string{"pwd"}
	if !errors.Is(policy.validateClaims(claims), ErrUnauthenticated) {
		t.Fatal("missing second factor was accepted")
	}
	claims.AuthenticationMethods = []string{"pwd", "otp"}
	claims.AuthenticationTime = now.Add(-31 * time.Minute).Unix()
	if !errors.Is(policy.validateClaims(claims), ErrUnauthenticated) {
		t.Fatal("stale authentication was accepted")
	}
}

func (a fixedAuthenticator) Authenticate(context.Context, *http.Request) (Principal, error) {
	return a.principal, a.err
}

func TestVerifiedPrincipalIgnoresSpoofedScopeHeaders(t *testing.T) {
	trusted := Principal{ActorID: "10000000-0000-4000-8000-000000000005", Scope: tenancy.Scope{TenantID: "10000000-0000-4000-8000-000000000001", CompanyID: "10000000-0000-4000-8000-000000000002", BranchID: "10000000-0000-4000-8000-000000000003", WarehouseID: "10000000-0000-4000-8000-000000000004"}}
	handler := &Handler{authenticator: fixedAuthenticator{principal: trusted}}
	request := httptest.NewRequest(http.MethodGet, "/v1/sales/sale-1", nil)
	request.Header.Set("X-Actor-ID", "spoofed-user")
	request.Header.Set("X-Tenant-ID", "spoofed-tenant")
	request.Header.Set("X-Company-ID", "spoofed-company")
	request.Header.Set("X-Branch-ID", "spoofed-branch")
	request.Header.Set("X-Warehouse-ID", "spoofed-warehouse")
	principal, ok := handler.requestContext(httptest.NewRecorder(), request)
	if !ok {
		t.Fatal("verified principal was rejected")
	}
	if principal != trusted {
		t.Fatalf("spoofed headers changed principal: %+v", principal)
	}
}

func TestRejectedAuthenticationDoesNotFallBackToScopeHeaders(t *testing.T) {
	handler := &Handler{authenticator: fixedAuthenticator{err: ErrUnauthenticated}}
	request := httptest.NewRequest(http.MethodGet, "/v1/sales/sale-1", nil)
	request.Header.Set("X-Actor-ID", "spoofed-user")
	request.Header.Set("X-Tenant-ID", "spoofed-tenant")
	request.Header.Set("X-Company-ID", "spoofed-company")
	request.Header.Set("X-Branch-ID", "spoofed-branch")
	request.Header.Set("X-Warehouse-ID", "spoofed-warehouse")
	recorder := httptest.NewRecorder()
	if _, ok := handler.requestContext(recorder, request); ok {
		t.Fatal("spoofed headers bypassed authentication")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestDevelopmentHeaderAuthenticatorIsExplicitAndValidatesAllScope(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/sales/sale-1", nil)
	request.Header.Set("X-Actor-ID", " 10000000-0000-4000-8000-00000000000A ")
	request.Header.Set("X-Tenant-ID", "10000000-0000-4000-8000-000000000001")
	request.Header.Set("X-Company-ID", "10000000-0000-4000-8000-000000000002")
	request.Header.Set("X-Branch-ID", "10000000-0000-4000-8000-000000000003")
	if _, err := (DevelopmentHeaderAuthenticator{}).Authenticate(context.Background(), request); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("incomplete scope accepted: %v", err)
	}
	request.Header.Set("X-Warehouse-ID", "10000000-0000-4000-8000-000000000004")
	principal, err := (DevelopmentHeaderAuthenticator{}).Authenticate(context.Background(), request)
	if err != nil || principal.ActorID != "10000000-0000-4000-8000-00000000000a" {
		t.Fatalf("development auth failed: %+v %v", principal, err)
	}
	request.Header.Set("X-Actor-ID", "opaque-dev-user")
	if _, err := (DevelopmentHeaderAuthenticator{}).Authenticate(context.Background(), request); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("non-UUID development actor accepted: %v", err)
	}
}

func TestOIDCClaimsRequireInternalUUIDAndDoNotUseOpaqueSubjectAsActorID(t *testing.T) {
	claims := oidcClaims{
		Subject: "identity-provider|opaque-subject", UserID: "00000000-0000-4000-8000-000000000005",
		TenantID: "00000000-0000-4000-8000-000000000001", CompanyID: "00000000-0000-4000-8000-000000000002",
		BranchID: "00000000-0000-4000-8000-000000000003", WarehouseID: "00000000-0000-4000-8000-000000000004",
	}
	principal, err := principalFromOIDCClaims(claims)
	if err != nil {
		t.Fatal(err)
	}
	if principal.ActorID != claims.UserID || principal.ActorID == claims.Subject || principal.Subject != claims.Subject {
		t.Fatalf("incorrect identity mapping: %+v", principal)
	}
	claims.UserID = claims.Subject
	if _, err := principalFromOIDCClaims(claims); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("opaque subject accepted as database UUID: %v", err)
	}
}
