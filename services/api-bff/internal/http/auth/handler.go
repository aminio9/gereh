package auth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	bffconfig "github.com/aminio9/gereh/services/api-bff/internal/config"
	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IdentityClient is implemented by the generated gRPC client.
type IdentityClient interface {
	BeginLogin(
		ctx context.Context,
		request *identityv1.BeginLoginRequest,
		options ...grpc.CallOption,
	) (*identityv1.BeginLoginResponse, error)

	CompleteLogin(
		ctx context.Context,
		request *identityv1.CompleteLoginRequest,
		options ...grpc.CallOption,
	) (*identityv1.CompleteLoginResponse, error)

	GetSession(
		ctx context.Context,
		request *identityv1.GetSessionRequest,
		options ...grpc.CallOption,
	) (*identityv1.GetSessionResponse, error)

	DeleteSession(
		ctx context.Context,
		request *identityv1.DeleteSessionRequest,
		options ...grpc.CallOption,
	) (*identityv1.DeleteSessionResponse, error)
}

// Handler exposes browser authentication endpoints.
type Handler struct {
	config bffconfig.AuthConfig
	client IdentityClient
	logger *slog.Logger
}

// NewHandler creates BFF authentication handlers.
func NewHandler(
	config bffconfig.AuthConfig,
	client IdentityClient,
	logger *slog.Logger,
) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		config: config,
		client: client,
		logger: logger,
	}
}

// Register registers public authentication routes.
func (handler *Handler) Register(router chi.Router) {
	router.Get(
		"/v1/auth/login",
		handler.beginLogin,
	)

	router.Get(
		"/v1/auth/callback",
		handler.completeLogin,
	)

	router.Get(
		"/v1/auth/session",
		handler.getSession,
	)

	router.Post(
		"/v1/auth/logout",
		handler.logout,
	)
}

func (handler *Handler) beginLogin(
	writer http.ResponseWriter,
	request *http.Request,
) {
	response, err := handler.client.BeginLogin(
		request.Context(),
		&identityv1.BeginLoginRequest{
			ReturnTo: request.URL.Query().Get("return_to"),
		},
	)
	if err != nil {
		handler.writeGRPCError(
			writer,
			err,
		)
		return
	}

	setTransactionCookie(
		writer,
		handler.config,
		response.GetBrowserBinding(),
		response.GetExpiresAt().AsTime(),
	)

	noStore(writer)

	http.Redirect(
		writer,
		request,
		response.GetAuthorizationUrl(),
		http.StatusFound,
	)
}

func (handler *Handler) completeLogin(
	writer http.ResponseWriter,
	request *http.Request,
) {
	noStore(writer)
	writer.Header().Set(
		"Referrer-Policy",
		"no-referrer",
	)

	if providerError := strings.TrimSpace(
		request.URL.Query().Get("error"),
	); providerError != "" {
		handler.logger.WarnContext(
			request.Context(),
			"OIDC provider returned an error",
			"provider_error",
			providerError,
		)

		writeProblem(
			writer,
			http.StatusUnauthorized,
			"authentication_failed",
			"Authentication failed",
		)
		return
	}

	transactionCookie, err := request.Cookie(
		handler.config.TransactionCookieName,
	)
	if err != nil {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"authentication_failed",
			"Authentication transaction is missing",
		)
		return
	}

	response, err := handler.client.CompleteLogin(
		request.Context(),
		&identityv1.CompleteLoginRequest{
			State:          request.URL.Query().Get("state"),
			Code:           request.URL.Query().Get("code"),
			BrowserBinding: transactionCookie.Value,
		},
	)

	clearTransactionCookie(
		writer,
		handler.config,
	)

	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	expiresAt := response.GetSession().
		GetExpiresAt().
		AsTime()

	setSessionCookies(
		writer,
		handler.config,
		response.GetSessionId(),
		response.GetCsrfToken(),
		expiresAt,
	)

	redirectURL, err := handler.returnURL(
		response.GetReturnTo(),
	)
	if err != nil {
		writeProblem(
			writer,
			http.StatusInternalServerError,
			"redirect_failed",
			"Authentication redirect failed",
		)
		return
	}

	http.Redirect(
		writer,
		request,
		redirectURL,
		http.StatusFound,
	)
}

func (handler *Handler) getSession(
	writer http.ResponseWriter,
	request *http.Request,
) {
	noStore(writer)

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
		if status.Code(err) == codes.Unauthenticated {
			clearSessionCookies(
				writer,
				handler.config,
			)
		}

		handler.writeGRPCError(writer, err)
		return
	}

	writeSessionJSON(
		writer,
		response.GetSession(),
	)
}

func (handler *Handler) logout(
	writer http.ResponseWriter,
	request *http.Request,
) {
	noStore(writer)

	sessionCookie, err := request.Cookie(
		handler.config.SessionCookieName,
	)
	if err != nil {
		clearSessionCookies(
			writer,
			handler.config,
		)

		writer.WriteHeader(http.StatusNoContent)
		return
	}

	csrfToken := strings.TrimSpace(
		request.Header.Get("X-CSRF-Token"),
	)

	_, err = handler.client.DeleteSession(
		request.Context(),
		&identityv1.DeleteSessionRequest{
			SessionId: sessionCookie.Value,
			CsrfToken: csrfToken,
		},
	)

	clearSessionCookies(
		writer,
		handler.config,
	)

	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) returnURL(
	returnTo string,
) (string, error) {
	origin, err := url.Parse(handler.config.WebOrigin)
	if err != nil {
		return "", err
	}

	reference, err := url.Parse(returnTo)
	if err != nil {
		return "", err
	}

	return origin.ResolveReference(reference).String(), nil
}

func (handler *Handler) writeGRPCError(
	writer http.ResponseWriter,
	err error,
) {
	switch status.Code(err) {
	case codes.InvalidArgument:
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid authentication request",
		)

	case codes.Unauthenticated:
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication failed",
		)

	case codes.PermissionDenied:
		writeProblem(
			writer,
			http.StatusForbidden,
			"forbidden",
			"Request validation failed",
		)

	case codes.Unavailable, codes.DeadlineExceeded:
		writeProblem(
			writer,
			http.StatusServiceUnavailable,
			"identity_unavailable",
			"Identity service is unavailable",
		)

	default:
		handler.logger.Error(
			"identity gRPC request failed",
			"error",
			err,
		)

		writeProblem(
			writer,
			http.StatusInternalServerError,
			"identity_error",
			"Identity operation failed",
		)
	}
}

func writeSessionJSON(
	writer http.ResponseWriter,
	session *identityv1.Session,
) {
	user := session.GetUser()

	writeJSON(
		writer,
		http.StatusOK,
		map[string]any{
			"user": map[string]any{
				"userId":        user.GetUserId(),
				"issuer":        user.GetIssuer(),
				"subject":       user.GetSubject(),
				"email":         user.GetEmail(),
				"emailVerified": user.GetEmailVerified(),
				"displayName":   user.GetDisplayName(),
				"pictureUrl":    user.GetPictureUrl(),
			},
			"createdAt": session.GetCreatedAt().
				AsTime().
				Format(time.RFC3339),
			"expiresAt": session.GetExpiresAt().
				AsTime().
				Format(time.RFC3339),
		},
	)
}

func writeProblem(
	writer http.ResponseWriter,
	statusCode int,
	problemType string,
	title string,
) {
	writer.Header().Set(
		"Content-Type",
		"application/problem+json",
	)

	writeJSON(
		writer,
		statusCode,
		map[string]any{
			"type":   problemType,
			"title":  title,
			"status": statusCode,
		},
	)
}

func writeJSON(
	writer http.ResponseWriter,
	statusCode int,
	value any,
) {
	writer.Header().Set(
		"Content-Type",
		"application/json",
	)
	writer.WriteHeader(statusCode)

	if err := json.NewEncoder(writer).Encode(value); err != nil {
		slog.Error(
			"encode HTTP response",
			"error",
			err,
		)
	}
}

func noStore(writer http.ResponseWriter) {
	writer.Header().Set(
		"Cache-Control",
		"no-store",
	)
	writer.Header().Set(
		"Pragma",
		"no-cache",
	)
}
