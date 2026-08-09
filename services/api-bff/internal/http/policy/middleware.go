package policy

import (
	"net/http"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
)

// RequirePermission validates one tenant permission and stores trusted
// authorization context for downstream handlers.
func (handler *Handler) RequirePermission(
	permission tenantv1.Permission,
) func(http.Handler) http.Handler {
	return handler.RequireAnyPermission(permission)
}

// RequireAnyPermission validates that the actor holds at least one of the
// given tenant permissions and stores trusted authorization context for
// downstream handlers.
func (handler *Handler) RequireAnyPermission(
	permissions ...tenantv1.Permission,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				principal, ok :=
					platformauth.PrincipalFromContext(
						request.Context(),
					)
				if !ok {
					writeProblem(
						writer,
						http.StatusUnauthorized,
						"unauthenticated",
						"Authentication is required",
					)
					return
				}

				for _, permission := range permissions {
					response, err :=
						handler.authorizer.CheckAuthorization(
							request.Context(),
							&tenantv1.CheckAuthorizationRequest{
								ActorUserId: principal.UserID,
								TenantId:    tenantID(request),
								Permission:  permission,
							},
						)
					if err != nil {
						handler.writeGRPCError(
							writer,
							err,
						)
						return
					}

					decision := response.GetDecision()

					if decision != nil &&
						decision.GetAllowed() {
						next.ServeHTTP(
							writer,
							request,
						)
						return
					}
				}

				writeProblem(
					writer,
					http.StatusForbidden,
					"forbidden",
					"Policy operation forbidden",
				)
			},
		)
	}
}
