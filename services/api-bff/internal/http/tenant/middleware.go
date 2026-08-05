package tenant

import (
	"net/http"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	platformauthz "github.com/aminio9/gereh/platform/go/authz"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/go-chi/chi/v5"
)

// RequireTenant validates basic tenant read access.
func (handler *Handler) RequireTenant(
	next http.Handler,
) http.Handler {
	return handler.RequirePermission(
		tenantv1.Permission_PERMISSION_TENANT_READ,
	)(next)
}

// RequirePermission validates one tenant permission and stores trusted
// authorization context for downstream handlers and gRPC calls.
func (handler *Handler) RequirePermission(
	permission tenantv1.Permission,
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

				tenantID := chi.URLParam(
					request,
					"tenantID",
				)

				response, err :=
					handler.client.CheckAuthorization(
						request.Context(),
						&tenantv1.CheckAuthorizationRequest{
							ActorUserId: principal.UserID,
							TenantId:    tenantID,
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

				if decision == nil ||
					!decision.GetAllowed() {
					writeProblem(
						writer,
						http.StatusForbidden,
						"forbidden",
						"Tenant operation forbidden",
					)
					return
				}

				principal.TenantID =
					decision.GetTenantId()

				principal.TenantRole =
					decision.GetRole().String()

				ctx := platformauth.WithPrincipal(
					request.Context(),
					principal,
				)

				ctx = platformauthz.WithDecision(
					ctx,
					platformauthz.Decision{
						ActorUserID: decision.
							GetActorUserId(),
						TenantID: decision.
							GetTenantId(),
						Role: decision.GetRole(),
						Permission: decision.
							GetPermission(),
						TenantVersion: decision.
							GetTenantVersion(),
						MembershipVersion: decision.
							GetMembershipVersion(),
					},
				)

				requestMetadata, _ :=
					grpcx.RequestMetadataFromContext(
						ctx,
					)

				requestMetadata.ActorUserID =
					principal.UserID

				requestMetadata.TenantID =
					principal.TenantID

				ctx = grpcx.WithRequestMetadata(
					ctx,
					requestMetadata,
				)

				next.ServeHTTP(
					writer,
					request.WithContext(ctx),
				)
			},
		)
	}
}
