// Package grpc exposes the identity-access application through gRPC.
package grpc

import (
	"context"
	"errors"

	identityv1 "github.com/aminio9/gereh/gen/go/gereh/identity/v1"
	"github.com/aminio9/gereh/services/identity-access/internal/application"
	"github.com/aminio9/gereh/services/identity-access/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Server implements the internal identity gRPC API.
type Server struct {
	identityv1.UnimplementedIdentityServiceServer

	service *application.Service
}

// New creates an identity gRPC server.
func New(service *application.Service) *Server {
	return &Server{
		service: service,
	}
}

// BeginLogin begins an OIDC login flow.
func (server *Server) BeginLogin(
	ctx context.Context,
	request *identityv1.BeginLoginRequest,
) (*identityv1.BeginLoginResponse, error) {
	result, err := server.service.BeginLogin(
		ctx,
		request.GetReturnTo(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &identityv1.BeginLoginResponse{
		AuthorizationUrl: result.AuthorizationURL,
		BrowserBinding:   result.BrowserBinding,
		ExpiresAt:        timestamppb.New(result.ExpiresAt),
	}, nil
}

// CompleteLogin completes an OIDC login flow.
func (server *Server) CompleteLogin(
	ctx context.Context,
	request *identityv1.CompleteLoginRequest,
) (*identityv1.CompleteLoginResponse, error) {
	result, err := server.service.CompleteLogin(
		ctx,
		request.GetState(),
		request.GetCode(),
		request.GetBrowserBinding(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &identityv1.CompleteLoginResponse{
		SessionId: result.SessionID,
		CsrfToken: result.CSRFToken,
		Session:   sessionToProto(result.Session),
		ReturnTo:  result.ReturnTo,
	}, nil
}

// GetSession returns an active browser session.
func (server *Server) GetSession(
	ctx context.Context,
	request *identityv1.GetSessionRequest,
) (*identityv1.GetSessionResponse, error) {
	session, err := server.service.GetSession(
		ctx,
		request.GetSessionId(),
	)
	if err != nil {
		return nil, mapError(err)
	}

	return &identityv1.GetSessionResponse{
		Session: sessionToProto(session),
	}, nil
}

// DeleteSession validates CSRF and revokes a browser session.
func (server *Server) DeleteSession(
	ctx context.Context,
	request *identityv1.DeleteSessionRequest,
) (*identityv1.DeleteSessionResponse, error) {
	if err := server.service.DeleteSession(
		ctx,
		request.GetSessionId(),
		request.GetCsrfToken(),
	); err != nil {
		return nil, mapError(err)
	}

	return &identityv1.DeleteSessionResponse{}, nil
}

func sessionToProto(
	session domain.Session,
) *identityv1.Session {
	return &identityv1.Session{
		User: &identityv1.AuthenticatedUser{
			UserId:        session.User.ID,
			Issuer:        session.User.Issuer,
			Subject:       session.User.Subject,
			Email:         session.User.Email,
			EmailVerified: session.User.EmailVerified,
			DisplayName:   session.User.DisplayName,
			PictureUrl:    session.User.PictureURL,
		},
		CreatedAt: timestamppb.New(session.CreatedAt),
		ExpiresAt: timestamppb.New(session.ExpiresAt),
	}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidRequest):
		return status.Error(
			codes.InvalidArgument,
			"invalid authentication request",
		)

	case errors.Is(err, domain.ErrAuthenticationFailed):
		return status.Error(
			codes.Unauthenticated,
			"authentication failed",
		)

	case errors.Is(err, domain.ErrSessionNotFound):
		return status.Error(
			codes.Unauthenticated,
			"session is not valid",
		)

	case errors.Is(err, domain.ErrCSRFValidation):
		return status.Error(
			codes.PermissionDenied,
			"csrf validation failed",
		)

	default:
		return status.Error(
			codes.Internal,
			"identity operation failed",
		)
	}
}
