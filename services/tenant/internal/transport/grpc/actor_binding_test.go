package grpc

import (
	"context"
	"testing"

	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	"github.com/aminio9/gereh/platform/go/grpcx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testActorID = "0198abc0-0000-7000-8000-000000000001"

func TestActorBindingAllowsMatchingActor(t *testing.T) {
	t.Parallel()

	interceptor := ActorBindingUnaryInterceptor()

	ctx := grpcx.WithRequestMetadata(
		context.Background(),
		grpcx.RequestMetadata{
			ActorUserID: testActorID,
		},
	)

	called := false

	_, err := interceptor(
		ctx,
		&tenantv1.GetTenantRequest{
			ActorUserId: testActorID,
			TenantId:    "0198abc0-0000-7000-8000-000000000002",
		},
		&grpc.UnaryServerInfo{
			FullMethod: "/gereh.tenant.v1.TenantService/GetTenant",
		},
		func(
			context.Context,
			any,
		) (any, error) {
			called = true

			return &tenantv1.GetTenantResponse{}, nil
		},
	)
	if err != nil {
		t.Fatalf(
			"interceptor() error = %v",
			err,
		)
	}

	if !called {
		t.Fatal("handler was not called")
	}
}

func TestActorBindingRejectsMismatchedActor(t *testing.T) {
	t.Parallel()

	interceptor := ActorBindingUnaryInterceptor()

	ctx := grpcx.WithRequestMetadata(
		context.Background(),
		grpcx.RequestMetadata{
			ActorUserID: testActorID,
		},
	)

	_, err := interceptor(
		ctx,
		&tenantv1.GetTenantRequest{
			ActorUserId: "0198abc0-0000-7000-8000-000000000099",
			TenantId:    "0198abc0-0000-7000-8000-000000000002",
		},
		&grpc.UnaryServerInfo{},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal(
				"handler was called for mismatched actor",
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

func TestActorBindingRejectsMissingMetadata(t *testing.T) {
	t.Parallel()

	interceptor := ActorBindingUnaryInterceptor()

	_, err := interceptor(
		context.Background(),
		&tenantv1.GetTenantRequest{
			ActorUserId: testActorID,
		},
		&grpc.UnaryServerInfo{},
		func(
			context.Context,
			any,
		) (any, error) {
			t.Fatal(
				"handler was called without actor metadata",
			)

			return nil, nil
		},
	)

	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf(
			"status.Code(error) = %v, want %v",
			status.Code(err),
			codes.Unauthenticated,
		)
	}
}
