package auth

import "context"

type principalContextKey struct{}

// WithPrincipal stores an authenticated principal.
func WithPrincipal(
	ctx context.Context,
	principal Principal,
) context.Context {
	return context.WithValue(
		ctx,
		principalContextKey{},
		principal,
	)
}

// PrincipalFromContext retrieves an authenticated principal.
func PrincipalFromContext(
	ctx context.Context,
) (Principal, bool) {
	principal, ok := ctx.Value(
		principalContextKey{},
	).(Principal)

	return principal, ok
}
