package organization

import (
	"net/http"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	"google.golang.org/protobuf/types/known/structpb"
)

// RequirePermission validates one tenant permission and stores trusted
// authorization context for downstream handlers.
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

				if decision == nil ||
					!decision.GetAllowed() {
					writeProblem(
						writer,
						http.StatusForbidden,
						"forbidden",
						"Organization operation forbidden",
					)
					return
				}

				next.ServeHTTP(
					writer,
					request,
				)
			},
		)
	}
}

// configurationStruct converts a JSON configuration object into a Protobuf
// struct. A nil or empty value becomes an empty struct.
func configurationStruct(
	value map[string]any,
) *structpb.Struct {
	result, err := structpb.NewStruct(value)
	if err != nil {
		return &structpb.Struct{}
	}

	return result
}
