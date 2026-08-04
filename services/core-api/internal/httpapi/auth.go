package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

var ErrUnauthenticated = errors.New("request authentication is missing or invalid")
var internalUUIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Principal struct {
	ActorID string
	Subject string
	Scope   tenancy.Scope
}

func (p Principal) Validate() error {
	if strings.TrimSpace(p.ActorID) == "" {
		return ErrUnauthenticated
	}
	if err := p.Scope.Validate(); err != nil {
		return ErrUnauthenticated
	}
	return nil
}

type Authenticator interface {
	Authenticate(ctx context.Context, request *http.Request) (Principal, error)
}

// DevelopmentHeaderAuthenticator is intentionally unsafe for public or
// production traffic. It exists only for explicit local development mode.
type DevelopmentHeaderAuthenticator struct{}

func (DevelopmentHeaderAuthenticator) Authenticate(_ context.Context, request *http.Request) (Principal, error) {
	principal := Principal{
		ActorID: strings.TrimSpace(request.Header.Get("X-Actor-ID")),
		Scope: tenancy.Scope{
			TenantID: strings.TrimSpace(request.Header.Get("X-Tenant-ID")), CompanyID: strings.TrimSpace(request.Header.Get("X-Company-ID")),
			BranchID: strings.TrimSpace(request.Header.Get("X-Branch-ID")), WarehouseID: strings.TrimSpace(request.Header.Get("X-Warehouse-ID")),
		},
	}
	if err := principal.Validate(); err != nil {
		return Principal{}, err
	}
	return principal, nil
}

type OIDCAuthenticator struct{ verifier *oidc.IDTokenVerifier }

func NewOIDCAuthenticator(ctx context.Context, issuerURL, audience string) (*OIDCAuthenticator, error) {
	if strings.TrimSpace(issuerURL) == "" || strings.TrimSpace(audience) == "" {
		return nil, errors.New("OIDC issuer and audience are required")
	}
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	return &OIDCAuthenticator{verifier: provider.Verifier(&oidc.Config{ClientID: audience})}, nil
}

type oidcClaims struct {
	Subject     string `json:"sub"`
	UserID      string `json:"user_id"`
	TenantID    string `json:"tenant_id"`
	CompanyID   string `json:"company_id"`
	BranchID    string `json:"branch_id"`
	WarehouseID string `json:"warehouse_id"`
}

func (a *OIDCAuthenticator) Authenticate(ctx context.Context, request *http.Request) (Principal, error) {
	if a == nil || a.verifier == nil {
		return Principal{}, ErrUnauthenticated
	}
	parts := strings.Fields(request.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return Principal{}, ErrUnauthenticated
	}
	token, err := a.verifier.Verify(ctx, parts[1])
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	var claims oidcClaims
	if err := token.Claims(&claims); err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return principalFromOIDCClaims(claims)
}

func principalFromOIDCClaims(claims oidcClaims) (Principal, error) {
	if strings.TrimSpace(claims.Subject) == "" || !internalUUIDPattern.MatchString(claims.UserID) ||
		!internalUUIDPattern.MatchString(claims.TenantID) || !internalUUIDPattern.MatchString(claims.CompanyID) ||
		!internalUUIDPattern.MatchString(claims.BranchID) || !internalUUIDPattern.MatchString(claims.WarehouseID) {
		return Principal{}, ErrUnauthenticated
	}
	principal := Principal{ActorID: claims.UserID, Subject: claims.Subject, Scope: tenancy.Scope{
		TenantID: claims.TenantID, CompanyID: claims.CompanyID, BranchID: claims.BranchID, WarehouseID: claims.WarehouseID,
	}}
	if err := principal.Validate(); err != nil {
		return Principal{}, err
	}
	return principal, nil
}
