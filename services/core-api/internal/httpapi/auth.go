package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/itemba-z/itemba-z/services/core-api/internal/platform/identity"
	"github.com/itemba-z/itemba-z/services/core-api/internal/tenancy"
)

var ErrUnauthenticated = errors.New("request authentication is missing or invalid")

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

func (p Principal) canonicalized() (Principal, error) {
	values := []*string{&p.ActorID, &p.Scope.TenantID, &p.Scope.CompanyID, &p.Scope.BranchID, &p.Scope.WarehouseID}
	for _, target := range values {
		canonical, err := identity.CanonicalUUID(*target)
		if err != nil {
			return Principal{}, ErrUnauthenticated
		}
		*target = canonical
	}
	if err := p.Validate(); err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return p, nil
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
	return principal.canonicalized()
}

type OIDCAssurancePolicy struct {
	RequiredACR string
	RequiredAMR []string
	MaxAuthAge  time.Duration
	now         func() time.Time
}

func (p OIDCAssurancePolicy) validate() error {
	if strings.TrimSpace(p.RequiredACR) == "" || len(p.RequiredAMR) == 0 {
		return errors.New("OIDC assurance class and authentication methods are required")
	}
	if p.MaxAuthAge < time.Minute || p.MaxAuthAge > 24*time.Hour {
		return errors.New("OIDC maximum authentication age must be between one minute and 24 hours")
	}
	for _, method := range p.RequiredAMR {
		if strings.TrimSpace(method) == "" {
			return errors.New("OIDC authentication methods must not be empty")
		}
	}
	return nil
}

func (p OIDCAssurancePolicy) validateClaims(claims oidcClaims) error {
	if claims.AssuranceClass != p.RequiredACR {
		return ErrUnauthenticated
	}
	methods := make(map[string]struct{}, len(claims.AuthenticationMethods))
	for _, method := range claims.AuthenticationMethods {
		methods[method] = struct{}{}
	}
	for _, required := range p.RequiredAMR {
		if _, ok := methods[required]; !ok {
			return ErrUnauthenticated
		}
	}
	now := time.Now().UTC()
	if p.now != nil {
		now = p.now().UTC()
	}
	authenticatedAt := time.Unix(claims.AuthenticationTime, 0).UTC()
	if claims.AuthenticationTime <= 0 || authenticatedAt.After(now.Add(time.Minute)) || now.Sub(authenticatedAt) > p.MaxAuthAge {
		return ErrUnauthenticated
	}
	return nil
}

type OIDCAuthenticator struct {
	verifier *oidc.IDTokenVerifier
	policy   OIDCAssurancePolicy
}

func NewOIDCAuthenticator(ctx context.Context, issuerURL, audience string, policy OIDCAssurancePolicy) (*OIDCAuthenticator, error) {
	if strings.TrimSpace(issuerURL) == "" || strings.TrimSpace(audience) == "" {
		return nil, errors.New("OIDC issuer and audience are required")
	}
	if err := policy.validate(); err != nil {
		return nil, err
	}
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider: %w", err)
	}
	return &OIDCAuthenticator{verifier: provider.Verifier(&oidc.Config{ClientID: audience}), policy: policy}, nil
}

type oidcClaims struct {
	Subject               string   `json:"sub"`
	UserID                string   `json:"user_id"`
	TenantID              string   `json:"tenant_id"`
	CompanyID             string   `json:"company_id"`
	BranchID              string   `json:"branch_id"`
	WarehouseID           string   `json:"warehouse_id"`
	AssuranceClass        string   `json:"acr"`
	AuthenticationMethods []string `json:"amr"`
	AuthenticationTime    int64    `json:"auth_time"`
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
	if err := a.policy.validateClaims(claims); err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return principalFromOIDCClaims(claims)
}

func principalFromOIDCClaims(claims oidcClaims) (Principal, error) {
	if strings.TrimSpace(claims.Subject) == "" {
		return Principal{}, ErrUnauthenticated
	}
	principal := Principal{ActorID: claims.UserID, Subject: claims.Subject, Scope: tenancy.Scope{
		TenantID: claims.TenantID, CompanyID: claims.CompanyID, BranchID: claims.BranchID, WarehouseID: claims.WarehouseID,
	}}
	return principal.canonicalized()
}
