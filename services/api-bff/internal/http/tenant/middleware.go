package tenant

import (
	"net/http"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"github.com/go-chi/chi/v5"
)

// RequireTenant validates membership and stores trusted tenant context.
//
// It is intended for future tenant-scoped BFF routes that call downstream
// services such as organization-agent or work-management.
func (handler *Handler) RequireTenant(
	next http.Handler,
) http.Handler {
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
				handler.client.GetTenantContext(
					request.Context(),
					&tenantv1.GetTenantContextRequest{
						ActorUserId: principal.UserID,
						TenantId:    tenantID,
					},
				)
			if err != nil {
				handler.writeGRPCError(writer, err)
				return
			}

			contextValue := response.GetContext()

			principal.TenantID = contextValue.
				GetTenant().
				GetTenantId()

			principal.TenantRole = contextValue.
				GetMembership().
				GetRole().
				String()

			ctx := platformauth.WithPrincipal(
				request.Context(),
				principal,
			)

			requestMetadata, _ :=
				grpcx.RequestMetadataFromContext(ctx)

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
