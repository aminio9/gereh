package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

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

// RequireCSRF validates the double-submit CSRF cookie and request origin.
func (handler *Handler) RequireCSRF(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(
		func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			origin := strings.TrimSuffix(
				request.Header.Get("Origin"),
				"/",
			)

			expectedOrigin := strings.TrimSuffix(
				handler.config.WebOrigin,
				"/",
			)

			if origin == "" ||
				origin != expectedOrigin {
				writeProblem(
					writer,
					http.StatusForbidden,
					"csrf_failed",
					"Request origin validation failed",
				)
				return
			}

			fetchSite := strings.ToLower(
				request.Header.Get("Sec-Fetch-Site"),
			)

			if fetchSite != "" &&
				fetchSite != "same-origin" {
				writeProblem(
					writer,
					http.StatusForbidden,
					"csrf_failed",
					"Cross-site request rejected",
				)
				return
			}

			cookie, err := request.Cookie(
				handler.config.CSRFCookieName,
			)
			if err != nil {
				writeProblem(
					writer,
					http.StatusForbidden,
					"csrf_failed",
					"CSRF token is missing",
				)
				return
			}

			headerValue := request.Header.Get(
				"X-CSRF-Token",
			)

			if headerValue == "" ||
				len(headerValue) != len(cookie.Value) ||
				subtle.ConstantTimeCompare(
					[]byte(headerValue),
					[]byte(cookie.Value),
				) != 1 {
				writeProblem(
					writer,
					http.StatusForbidden,
					"csrf_failed",
					"CSRF token is invalid",
				)
				return
			}

			next.ServeHTTP(writer, request)
		},
	)
}
