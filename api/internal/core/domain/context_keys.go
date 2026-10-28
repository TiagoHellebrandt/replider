package domain

import "context"

// impersonatorCtxKey is the typed key used to thread the operator
// email (when an impersonation session is active) through go contexts
// from the auth middleware down into use case audit writers. Defined
// here in the domain package so both `http/middleware` and
// `core/usecases` import the same type — context.Value compares keys
// by both type AND value, so duplicating the struct in each package
// would silently break the lookup.
type impersonatorCtxKey struct{}

// WithImpersonator returns a new context with the operator email
// stamped on it. Called by the impersonation-audit middleware after
// the JWT claim is decoded.
func WithImpersonator(ctx context.Context, actorEmail string) context.Context {
	if actorEmail == "" {
		return ctx
	}
	return context.WithValue(ctx, impersonatorCtxKey{}, actorEmail)
}

// GetImpersonator reads the operator email — empty when the session
// is a regular (non-impersonation) one.
func GetImpersonator(ctx context.Context) string {
	v, _ := ctx.Value(impersonatorCtxKey{}).(string)
	return v
}

// auditMetaCtxKey threads the HTTP request origin (client IP +
// User-Agent) from the audit-meta middleware down into the audit log
// writer (US-073). Like impersonatorCtxKey it lives in the domain
// package so both `http/middleware` and the repository layer compare
// against the same key type.
type auditMetaCtxKey struct{}

// AuditMeta carries the request origin stamped onto every audit row so
// an admin can see not just "who did what" but "from where".
type AuditMeta struct {
	IP        string
	UserAgent string
}

// WithAuditMeta returns a new context carrying the request IP +
// User-Agent. A no-op when both are empty (e.g. background workers with
// no HTTP request) so the audit row's columns stay NULL.
func WithAuditMeta(ctx context.Context, ip, userAgent string) context.Context {
	if ip == "" && userAgent == "" {
		return ctx
	}
	return context.WithValue(ctx, auditMetaCtxKey{}, AuditMeta{IP: ip, UserAgent: userAgent})
}

// GetAuditMeta reads the request origin — the zero AuditMeta when the
// work isn't tied to an inbound HTTP request.
func GetAuditMeta(ctx context.Context) AuditMeta {
	v, _ := ctx.Value(auditMetaCtxKey{}).(AuditMeta)
	return v
}
