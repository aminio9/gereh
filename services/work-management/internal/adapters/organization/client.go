// Package organization adapts the Organization Service company validation
// for the Work Management Service.
package organization

import (
	"context"
	"fmt"
	"time"

	organizationv1 "github.com/aminio9/gereh/gen/go/gereh/organization/v1"
	"github.com/aminio9/gereh/services/work-management/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Client validates companies through the Organization Service.
type Client struct {
	client  organizationv1.OrganizationServiceClient
	timeout time.Duration
}

// NewClient creates an Organization Service-backed company validator.
func NewClient(
	client organizationv1.OrganizationServiceClient,
	timeout time.Duration,
) *Client {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	return &Client{
		client:  client,
		timeout: timeout,
	}
}

// EnsureCompanyActive returns nil only when the company exists and is active.
func (client *Client) EnsureCompanyActive(
	ctx context.Context,
	actorUserID string,
	tenantID string,
	companyID string,
) error {
	callContext, cancel := context.WithTimeout(
		ctx,
		client.timeout,
	)
	defer cancel()

	response, err := client.client.GetCompany(
		callContext,
		&organizationv1.GetCompanyRequest{
			ActorUserId: actorUserID,
			TenantId:    tenantID,
			CompanyId:   companyID,
		},
	)
	if err != nil {
		switch status.Code(err) {
		case codes.NotFound:
			return domain.ErrNotFound

		case codes.PermissionDenied,
			codes.Unauthenticated:
			return domain.ErrForbidden

		case codes.DeadlineExceeded,
			codes.Unavailable:
			return fmt.Errorf(
				"organization service unavailable: %w",
				err,
			)

		default:
			return fmt.Errorf(
				"validate company: %w",
				err,
			)
		}
	}

	company := response.GetCompany()
	if company == nil {
		return fmt.Errorf(
			"organization service returned no company",
		)
	}

	if company.GetStatus() !=
		organizationv1.CompanyStatus_COMPANY_STATUS_ACTIVE {
		return domain.ErrCompanyNotActive
	}

	return nil
}
