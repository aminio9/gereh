package grpcx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestUnaryClientDefaultDeadline(t *testing.T) {
	t.Parallel()

	interceptor := unaryClientDefaultDeadlineInterceptor(
		250 * time.Millisecond,
	)

	var observed time.Time

	err := interceptor(
		context.Background(),
		"/test.Service/Test",
		nil,
		nil,
		nil,
		func(
			ctx context.Context,
			_ string,
			_ any,
			_ any,
			_ *grpc.ClientConn,
			_ ...grpc.CallOption,
		) error {
			deadline, ok := ctx.Deadline()

			require.True(t, ok)

			observed = deadline

			return nil
		},
	)

	require.NoError(t, err)

	require.WithinDuration(
		t,
		time.Now().Add(
			250*time.Millisecond,
		),
		observed,
		100*time.Millisecond,
	)
}

func TestUnaryClientPreservesCallerDeadline(t *testing.T) {
	t.Parallel()

	expected := time.Now().Add(
		50 * time.Millisecond,
	)

	ctx, cancel := context.WithDeadline(
		context.Background(),
		expected,
	)

	defer cancel()

	interceptor := unaryClientDefaultDeadlineInterceptor(
		5 * time.Second,
	)

	err := interceptor(
		ctx,
		"/test.Service/Test",
		nil,
		nil,
		nil,
		func(
			callContext context.Context,
			_ string,
			_ any,
			_ any,
			_ *grpc.ClientConn,
			_ ...grpc.CallOption,
		) error {
			actual, ok := callContext.Deadline()

			require.True(t, ok)

			require.WithinDuration(
				t,
				expected,
				actual,
				time.Millisecond,
			)

			return nil
		},
	)

	require.NoError(t, err)
}

func TestUnaryClientDefaultsToFiveSeconds(t *testing.T) {
	t.Parallel()

	// The zero value resolves to the conservative five-second fallback
	// in the public chain builder, which is what NewClient uses.
	interceptors := UnaryClientInterceptors(0)

	require.Len(t, interceptors, 2)

	interceptor := interceptors[0]

	started := time.Now()

	err := interceptor(
		context.Background(),
		"/test.Service/Test",
		nil,
		nil,
		nil,
		func(
			ctx context.Context,
			_ string,
			_ any,
			_ any,
			_ *grpc.ClientConn,
			_ ...grpc.CallOption,
		) error {
			deadline, ok := ctx.Deadline()

			require.True(t, ok)

			require.WithinDuration(
				t,
				started.Add(defaultUnaryTimeout),
				deadline,
				time.Second,
			)

			return nil
		},
	)

	require.NoError(t, err)
}

func TestUnaryClientPreservesShorterCallerDeadline(t *testing.T) {
	t.Parallel()

	expected := time.Now().Add(
		25 * time.Millisecond,
	)

	ctx, cancel := context.WithDeadline(
		context.Background(),
		expected,
	)

	defer cancel()

	interceptor := unaryClientDefaultDeadlineInterceptor(
		time.Second,
	)

	err := interceptor(
		ctx,
		"/test.Service/Test",
		nil,
		nil,
		nil,
		func(
			callContext context.Context,
			_ string,
			_ any,
			_ any,
			_ *grpc.ClientConn,
			_ ...grpc.CallOption,
		) error {
			actual, ok := callContext.Deadline()

			require.True(t, ok)

			require.WithinDuration(
				t,
				expected,
				actual,
				time.Millisecond,
			)

			return nil
		},
	)

	require.NoError(t, err)
}

func TestUnaryClientDeadlineCancellationPropagates(t *testing.T) {
	t.Parallel()

	interceptor := unaryClientDefaultDeadlineInterceptor(
		25 * time.Millisecond,
	)

	err := interceptor(
		context.Background(),
		"/test.Service/Test",
		nil,
		nil,
		nil,
		func(
			ctx context.Context,
			_ string,
			_ any,
			_ any,
			_ *grpc.ClientConn,
			_ ...grpc.CallOption,
		) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				t.Fatal("context deadline was not propagated")
				return nil
			}
		},
	)

	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestUnaryClientInterceptorsDefaultChain(t *testing.T) {
	t.Parallel()

	interceptors := UnaryClientInterceptors()

	require.Len(t, interceptors, 2)
}

func TestUnaryClientInterceptorsCustomTimeout(t *testing.T) {
	t.Parallel()

	interceptors := UnaryClientInterceptors(
		3 * time.Second,
	)

	require.Len(t, interceptors, 2)
}
