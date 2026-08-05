package authz

import (
	"context"
	"fmt"
	"strings"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MethodPolicy defines authorization for one complete gRPC method name.
type MethodPolicy struct {
	Public     bool
	Permission tenantv1.Permission
}

// UnaryServerInterceptor enforces explicit per-method authorization.
//
// Any method absent from policies is denied by default.
func UnaryServerInterceptor(
	checker Checker,
	policies map[string]MethodPolicy,
) (grpc.UnaryServerInterceptor, error) {
	if checker == nil {
		return nil, fmt.Errorf(
			"authorization checker is required",
		)
	}

	if len(policies) == 0 {
		return nil, fmt.Errorf(
			"authorization policies are required",
		)
	}

	return func(
		ctx context.Context,
		request any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		policy, configured :=
			policies[info.FullMethod]

		if !configured {
			return nil, status.Error(
				codes.PermissionDenied,
				"RPC method has no authorization policy",
			)
		}

		if policy.Public {
			return handler(ctx, request)
		}

		if policy.Permission ==
			tenantv1.Permission_PERMISSION_UNSPECIFIED {
			return nil, status.Error(
				codes.PermissionDenied,
				"RPC method has an invalid authorization policy",
			)
		}

		requestMetadata, ok :=
			grpcx.RequestMetadataFromContext(ctx)

		if !ok ||
			strings.TrimSpace(
				requestMetadata.ActorUserID,
			) == "" {
			return nil, status.Error(
				codes.Unauthenticated,
				"authenticated actor is required",
			)
		}

		if strings.TrimSpace(
			requestMetadata.TenantID,
		) == "" {
			return nil, status.Error(
				codes.InvalidArgument,
				"validated tenant context is required",
			)
		}

		decision, allowed, err := checker.Check(
			ctx,
			requestMetadata.ActorUserID,
			requestMetadata.TenantID,
			policy.Permission,
		)
		if err != nil {
			// Fail closed. An unavailable authorization service must not
			// silently grant access.
			return nil, status.Error(
				codes.Unavailable,
				"authorization service unavailable",
			)
		}

		if !allowed {
			return nil, status.Error(
				codes.PermissionDenied,
				"tenant permission denied",
			)
		}

		ctx = WithDecision(ctx, decision)

		return handler(ctx, request)
	}, nil
}
