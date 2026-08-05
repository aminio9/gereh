package auth

import (
	"net/http"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
)

// RequireSession validates the browser session and stores a Principal.
func (handler *Handler) RequireSession(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			sessionCookie, err := request.Cookie(
				handler.config.SessionCookieName,
			)
			if err != nil {
				writeProblem(
					writer,
					http.StatusUnauthorized,
					"unauthenticated",
					"Authentication is required",
				)
				return
			}

			response, err := handler.client.GetSession(
				request.Context(),
				&identityv1.GetSessionRequest{
					SessionId: sessionCookie.Value,
				},
			)
			if err != nil {
				clearSessionCookies(
					writer,
					handler.config,
				)

				handler.writeGRPCError(
					writer,
					err,
				)
				return
			}

			user := response.GetSession().GetUser()

			principal := platformauth.Principal{
				UserID:        user.GetUserId(),
				Issuer:        user.GetIssuer(),
				Subject:       user.GetSubject(),
				Email:         user.GetEmail(),
				EmailVerified: user.GetEmailVerified(),
				DisplayName:   user.GetDisplayName(),
				PictureURL:    user.GetPictureUrl(),
			}

			ctx := platformauth.WithPrincipal(
				request.Context(),
				principal,
			)

			next.ServeHTTP(
				writer,
				request.WithContext(ctx),
			)
		},
	)
}
