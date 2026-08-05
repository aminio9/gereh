package auth

import (
	"net/http"
	"time"

	bffconfig "github.com/aminio9/gereh/services/api-bff/internal/config"
)

func setTransactionCookie(
	writer http.ResponseWriter,
	config bffconfig.AuthConfig,
	value string,
	expiresAt time.Time,
) {
	http.SetCookie(
		writer,
		&http.Cookie{
			Name:     config.TransactionCookieName,
			Value:    value,
			Path:     "/v1/auth/callback",
			Expires:  expiresAt,
			MaxAge:   maxAge(expiresAt),
			HttpOnly: true,
			Secure:   config.CookieSecure,
			SameSite: http.SameSiteLaxMode,
		},
	)
}

func clearTransactionCookie(
	writer http.ResponseWriter,
	config bffconfig.AuthConfig,
) {
	http.SetCookie(
		writer,
		&http.Cookie{
			Name:     config.TransactionCookieName,
			Value:    "",
			Path:     "/v1/auth/callback",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   config.CookieSecure,
			SameSite: http.SameSiteLaxMode,
		},
	)
}

func setSessionCookies(
	writer http.ResponseWriter,
	config bffconfig.AuthConfig,
	sessionID string,
	csrfToken string,
	expiresAt time.Time,
) {
	http.SetCookie(
		writer,
		&http.Cookie{
			Name:     config.SessionCookieName,
			Value:    sessionID,
			Path:     "/",
			Expires:  expiresAt,
			MaxAge:   maxAge(expiresAt),
			HttpOnly: true,
			Secure:   config.CookieSecure,
			SameSite: config.CookieSameSite,
		},
	)

	http.SetCookie(
		writer,
		&http.Cookie{
			Name:     config.CSRFCookieName,
			Value:    csrfToken,
			Path:     "/",
			Expires:  expiresAt,
			MaxAge:   maxAge(expiresAt),
			HttpOnly: false,
			Secure:   config.CookieSecure,
			SameSite: http.SameSiteStrictMode,
		},
	)
}

func clearSessionCookies(
	writer http.ResponseWriter,
	config bffconfig.AuthConfig,
) {
	http.SetCookie(
		writer,
		&http.Cookie{
			Name:     config.SessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   config.CookieSecure,
			SameSite: config.CookieSameSite,
		},
	)

	http.SetCookie(
		writer,
		&http.Cookie{
			Name:     config.CSRFCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: false,
			Secure:   config.CookieSecure,
			SameSite: http.SameSiteStrictMode,
		},
	)
}

func maxAge(expiresAt time.Time) int {
	remaining := time.Until(expiresAt)

	if remaining <= 0 {
		return -1
	}

	return int(remaining.Seconds())
}
