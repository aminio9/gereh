package grpcx

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptors returns the standard unary server interceptor chain.
func UnaryServerInterceptors(
	logger *slog.Logger,
) []grpc.UnaryServerInterceptor {
	return []grpc.UnaryServerInterceptor{
		unaryRecoveryInterceptor(logger),
		unaryMetadataInterceptor(),
		unaryLoggingInterceptor(logger),
	}
}

// StreamServerInterceptors returns the standard stream interceptor chain.
func StreamServerInterceptors(
	logger *slog.Logger,
) []grpc.StreamServerInterceptor {
	return []grpc.StreamServerInterceptor{
		streamRecoveryInterceptor(logger),
		streamMetadataInterceptor(),
		streamLoggingInterceptor(logger),
	}
}

// UnaryClientInterceptors returns the standard unary client chain.
func UnaryClientInterceptors() []grpc.UnaryClientInterceptor {
	return []grpc.UnaryClientInterceptor{
		unaryClientMetadataInterceptor(),
	}
}

// StreamClientInterceptors returns the standard stream client chain.
func StreamClientInterceptors() []grpc.StreamClientInterceptor {
	return []grpc.StreamClientInterceptor{
		streamClientMetadataInterceptor(),
	}
}

func unaryMetadataInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		_ *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		requestMetadata := RequestMetadataFromIncoming(ctx)

		ctx = WithRequestMetadata(ctx, requestMetadata)

		return handler(ctx, request)
	}
}

func streamMetadataInterceptor() grpc.StreamServerInterceptor {
	return func(
		service any,
		stream grpc.ServerStream,
		_ *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		requestMetadata := RequestMetadataFromIncoming(
			stream.Context(),
		)

		ctx := WithRequestMetadata(
			stream.Context(),
			requestMetadata,
		)

		return handler(
			service,
			&contextServerStream{
				ServerStream: stream,
				ctx:          ctx,
			},
		)
	}
}

func unaryLoggingInterceptor(
	logger *slog.Logger,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (response any, handlerErr error) {
		startedAt := time.Now()

		response, handlerErr = handler(ctx, request)

		logger.LogAttrs(
			ctx,
			slog.LevelInfo,
			"gRPC unary request completed",
			slog.String("rpc.method", info.FullMethod),
			slog.String(
				"rpc.status",
				status.Code(handlerErr).String(),
			),
			slog.Duration(
				"rpc.duration",
				time.Since(startedAt),
			),
		)

		return response, handlerErr
	}
}

func streamLoggingInterceptor(
	logger *slog.Logger,
) grpc.StreamServerInterceptor {
	return func(
		service any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		startedAt := time.Now()

		handlerErr := handler(service, stream)

		logger.LogAttrs(
			stream.Context(),
			slog.LevelInfo,
			"gRPC stream completed",
			slog.String("rpc.method", info.FullMethod),
			slog.String(
				"rpc.status",
				status.Code(handlerErr).String(),
			),
			slog.Bool(
				"rpc.client_stream",
				info.IsClientStream,
			),
			slog.Bool(
				"rpc.server_stream",
				info.IsServerStream,
			),
			slog.Duration(
				"rpc.duration",
				time.Since(startedAt),
			),
		)

		return handlerErr
	}
}

func unaryRecoveryInterceptor(
	logger *slog.Logger,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (
		response any,
		handlerErr error,
	) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			logger.LogAttrs(
				ctx,
				slog.LevelError,
				"panic recovered from gRPC unary request",
				slog.String("rpc.method", info.FullMethod),
				slog.Any("panic", recovered),
				slog.String("stack", string(debug.Stack())),
			)

			handlerErr = status.Error(
				codes.Internal,
				"internal server error",
			)
		}()

		return handler(ctx, request)
	}
}

func streamRecoveryInterceptor(
	logger *slog.Logger,
) grpc.StreamServerInterceptor {
	return func(
		service any,
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (handlerErr error) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			logger.LogAttrs(
				stream.Context(),
				slog.LevelError,
				"panic recovered from gRPC stream",
				slog.String("rpc.method", info.FullMethod),
				slog.Any("panic", recovered),
				slog.String("stack", string(debug.Stack())),
			)

			handlerErr = status.Error(
				codes.Internal,
				"internal server error",
			)
		}()

		return handler(service, stream)
	}
}

func unaryClientMetadataInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		request any,
		response any,
		connection *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		options ...grpc.CallOption,
	) error {
		return invoker(
			InjectOutgoingMetadata(ctx),
			method,
			request,
			response,
			connection,
			options...,
		)
	}
}

func streamClientMetadataInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		description *grpc.StreamDesc,
		connection *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		options ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		return streamer(
			InjectOutgoingMetadata(ctx),
			description,
			connection,
			method,
			options...,
		)
	}
}

type contextServerStream struct {
	grpc.ServerStream

	ctx context.Context
}

func (stream *contextServerStream) Context() context.Context {
	return stream.ctx
}
