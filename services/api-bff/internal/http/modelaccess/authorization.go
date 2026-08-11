package modelaccess

import (
	"net/http"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	"github.com/go-chi/chi/v5"
)

// RequirePermission performs an early Tenant Service authorization check.
//
// Model Access performs the same authoritative check itself, preserving
// defense in depth.
func (handler *Handler) RequirePermission(
	permission tenantv1.Permission,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				principal, ok := platformauth.PrincipalFromContext(
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

				tenantID := chi.URLParam(request, "tenantID")

				response, err := handler.authorizer.CheckAuthorization(
					request.Context(),
					&tenantv1.CheckAuthorizationRequest{
						ActorUserId: principal.UserID,
						TenantId:    tenantID,
						Permission:  permission,
					},
				)
				if err != nil {
					handler.writeGRPCError(writer, err)

					return
				}

				if response.GetDecision() == nil ||
					!response.GetDecision().GetAllowed() {
					writeProblem(
						writer,
						http.StatusForbidden,
						"forbidden",
						"Model Access operation forbidden",
					)

					return
				}

				next.ServeHTTP(writer, request)
			},
		)
	}
}
