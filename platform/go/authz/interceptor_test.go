package authz

import (
	"context"
	"errors"
	"testing"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeChecker struct {
	allowed bool
	err     error
}

func (checker fakeChecker) Check(
	_ context.Context,
	actorUserID string,
	tenantID string,
	permission tenantv1.Permission,
) (Decision, bool, error) {
	if checker.err != nil {
		return Decision{}, false, checker.err
	}

	return Decision{
		ActorUserID: actorUserID,
		TenantID:    tenantID,
		Permission:  permission,
	}, checker.allowed, nil
}

func TestInterceptorDeniesUnconfiguredMethod(t *testing.T) {
	t.Parallel()

	interceptor, err := UnaryServerInterceptor(
		fakeChecker{
			allowed: true,
		},
		map[string]MethodPolicy{
			"/example.Service/Configured": {
				Permission: tenantv1.
					Permission_PERMISSION_TENANT_READ,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"UnaryServerInterceptor() error = %v",
			err,
		)
	}

	_, err = interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{
			FullMethod: "/example.Service/Unknown",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal(
				"handler called for unconfigured method",
			)

			return nil, nil
		},
	)

	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf(
			"status.Code(error) = %v, want %v",
			status.Code(err),
			codes.PermissionDenied,
		)
	}
}

func TestInterceptorFailsClosed(t *testing.T) {
	t.Parallel()

	interceptor, err := UnaryServerInterceptor(
		fakeChecker{
			err: errors.New("tenant unavailable"),
		},
		map[string]MethodPolicy{
			"/example.Service/Get": {
				Permission: tenantv1.
					Permission_PERMISSION_TENANT_READ,
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"UnaryServerInterceptor() error = %v",
			err,
		)
	}

	ctx := grpcx.WithRequestMetadata(
		context.Background(),
		grpcx.RequestMetadata{
			ActorUserID: "0198abc0-0000-7000-8000-000000000001",
			TenantID:    "0198abc0-0000-7000-8000-000000000002",
		},
	)

	_, err = interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{
			FullMethod: "/example.Service/Get",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal(
				"handler called when authorization failed",
			)

			return nil, nil
		},
	)

	if status.Code(err) != codes.Unavailable {
		t.Fatalf(
			"status.Code(error) = %v, want %v",
			status.Code(err),
			codes.Unavailable,
		)
	}
}
