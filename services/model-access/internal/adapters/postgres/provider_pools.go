package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
	"github.com/jackc/pgx/v5"
)

// selectPlatformManagedPool picks the best enabled Gereh provider pool.
//
// Ordering:
//
//  1. exact tenant-region pool
//  2. wildcard/global pool
//  3. highest operator priority
//  4. stable pool-key tie break
//
// The caller already runs inside the connection-create transaction.
func selectPlatformManagedPool(
	ctx context.Context,
	transaction pgx.Tx,
	providerKey string,
	region string,
) (string, error) {
	region = strings.ToLower(
		strings.TrimSpace(region),
	)

	if region == "" {
		return "", domain.ErrPlatformManagedPoolUnavailable
	}

	var poolKey string

	err := transaction.QueryRow(
		ctx,
		`
			SELECT pool_key
			FROM model_access_provider_pools
			WHERE provider_key = $1
			  AND enabled
			  AND (
				$2 = ANY(regions)
				OR '*' = ANY(regions)
			  )
			ORDER BY
				CASE
					WHEN $2 = ANY(regions)
						THEN 0
					ELSE 1
				END,
				priority DESC,
				pool_key
			LIMIT 1
			FOR SHARE
		`,
		providerKey,
		region,
	).Scan(&poolKey)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrPlatformManagedPoolUnavailable
	}

	if err != nil {
		return "", fmt.Errorf(
			"select platform-managed provider pool: %w",
			err,
		)
	}

	return poolKey, nil
}
