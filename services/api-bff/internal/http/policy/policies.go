package policy

import (
	"net/http"
	"strconv"

	policyv1 "github.com/aminio9/gereh/gen/go/gereh/policy/v1"
	tenantv1 "github.com/aminio9/gereh/gen/go/gereh/tenant/v1"
	authhttp "github.com/aminio9/gereh/services/api-bff/internal/http/auth"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/structpb"
)

func (handler *Handler) registerPolicyRoutes(
	tenantRouter chi.Router,
	authHandler *authhttp.Handler,
) {
	tenantRouter.Route(
		"/policies",
		func(policyRouter chi.Router) {
			policyRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_POLICY_CREATE,
				),
				authHandler.RequireCSRF,
			).Post(
				"/",
				handler.createPolicy,
			)

			policyRouter.With(
				handler.RequirePermission(
					tenantv1.Permission_PERMISSION_POLICY_READ,
				),
			).Get(
				"/",
				handler.listPolicies,
			)

			policyRouter.Route(
				"/{policyID}",
				func(resource chi.Router) {
					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_POLICY_READ,
						),
					).Get(
						"/",
						handler.getPolicy,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_POLICY_UPDATE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/versions",
						handler.createPolicyVersion,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_POLICY_ACTIVATE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/activate",
						handler.activatePolicy,
					)

					resource.With(
						handler.RequirePermission(
							tenantv1.Permission_PERMISSION_POLICY_ARCHIVE,
						),
						authHandler.RequireCSRF,
					).Post(
						"/archive",
						handler.archivePolicy,
					)
				},
			)
		},
	)
}

type createPolicyRequest struct {
	ScopeType   int32   `json:"scopeType"`
	ScopeID     *string `json:"scopeId"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
}

type createPolicyVersionRequest struct {
	ExpectedResourceVersion int64               `json:"expectedResourceVersion"`
	DefaultEffect           int32               `json:"defaultEffect"`
	Rules                   []createRuleRequest `json:"rules"`
	Notes                   string              `json:"notes"`
}

type createRuleRequest struct {
	RuleID                    string         `json:"ruleId"`
	Priority                  int32          `json:"priority"`
	Name                      string         `json:"name"`
	Enabled                   bool           `json:"enabled"`
	Effect                    int32          `json:"effect"`
	Condition                 string         `json:"condition"`
	ActionPatterns            []string       `json:"actionPatterns"`
	ResourceTypes             []string       `json:"resourceTypes"`
	RiskLevels                []int32        `json:"riskLevels"`
	MaximumEstimatedCostMicro *int64         `json:"maximumEstimatedCostMicroUsd"`
	Constraints               map[string]any `json:"constraints"`
	Reason                    string         `json:"reason"`
}

type activatePolicyRequest struct {
	ExpectedResourceVersion int64 `json:"expectedResourceVersion"`
	PolicyVersion           int64 `json:"policyVersion"`
}

type archivePolicyRequest struct {
	ExpectedResourceVersion int64 `json:"expectedResourceVersion"`
}

func (handler *Handler) createPolicy(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, ok := principal(request)
	if !ok {
		writeProblem(
			writer,
			http.StatusUnauthorized,
			"unauthenticated",
			"Authentication is required",
		)
		return
	}

	var input createPolicyRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid policy request",
		)
		return
	}

	scope := &policyv1.PolicyScope{
		Type: policyv1.PolicyScopeType(
			input.ScopeType,
		),
	}

	if input.ScopeID != nil {
		scope.ScopeId = input.ScopeID
	}

	response, err := handler.client.CreatePolicy(
		request.Context(),
		&policyv1.CreatePolicyRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			Scope:       scope,
			Name:        input.Name,
			Description: input.Description,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(
		writer,
		http.StatusCreated,
		response,
	)
}

func (handler *Handler) getPolicy(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetPolicy(
		request.Context(),
		&policyv1.GetPolicyRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			PolicyId: chi.URLParam(
				request,
				"policyID",
			),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listPolicies(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	response, err := handler.client.ListPolicies(
		request.Context(),
		&policyv1.ListPoliciesRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			PageSize:    int32(pageSize),
			PageToken: request.URL.Query().
				Get("page_token"),
			IncludeArchived: request.URL.Query().
				Get("include_archived") == "true",
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) createPolicyVersion(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input createPolicyVersionRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid policy version request",
		)
		return
	}

	rules := make([]*policyv1.PolicyRule, 0, len(input.Rules))

	for _, value := range input.Rules {
		rule, err := toPolicyRule(value)
		if err != nil {
			writeProblem(
				writer,
				http.StatusBadRequest,
				"invalid_request",
				"Invalid policy rule constraints",
			)
			return
		}

		rules = append(rules, rule)
	}

	response, err := handler.client.CreatePolicyVersion(
		request.Context(),
		&policyv1.CreatePolicyVersionRequest{
			ActorUserId:             principal.UserID,
			TenantId:                tenantID(request),
			PolicyId:                chi.URLParam(request, "policyID"),
			ExpectedResourceVersion: input.ExpectedResourceVersion,
			DefaultEffect:           policyv1.PolicyEffect(input.DefaultEffect),
			Rules:                   rules,
			Notes:                   input.Notes,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusCreated, response)
}

func toPolicyRule(
	value createRuleRequest,
) (*policyv1.PolicyRule, error) {
	rule := &policyv1.PolicyRule{
		RuleId:         value.RuleID,
		Priority:       value.Priority,
		Name:           value.Name,
		Enabled:        value.Enabled,
		Effect:         policyv1.PolicyEffect(value.Effect),
		Condition:      value.Condition,
		ActionPatterns: value.ActionPatterns,
		ResourceTypes:  value.ResourceTypes,
		Reason:         value.Reason,
	}

	for _, level := range value.RiskLevels {
		rule.RiskLevels = append(
			rule.RiskLevels,
			policyv1.RiskLevel(level),
		)
	}

	if value.MaximumEstimatedCostMicro != nil {
		rule.MaximumEstimatedCostMicroUsd =
			value.MaximumEstimatedCostMicro
	}

	if value.Constraints != nil {
		constraints, err := structpb.NewStruct(
			value.Constraints,
		)
		if err != nil {
			return nil, err
		}

		rule.Constraints = constraints
	}

	return rule, nil
}

func (handler *Handler) activatePolicy(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input activatePolicyRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid policy activation request",
		)
		return
	}

	response, err := handler.client.ActivatePolicy(
		request.Context(),
		&policyv1.ActivatePolicyRequest{
			ActorUserId:             principal.UserID,
			TenantId:                tenantID(request),
			PolicyId:                chi.URLParam(request, "policyID"),
			ExpectedResourceVersion: input.ExpectedResourceVersion,
			PolicyVersion:           input.PolicyVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) archivePolicy(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	var input archivePolicyRequest

	if err := decodeJSON(
		writer,
		request,
		&input,
	); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid policy archival request",
		)
		return
	}

	response, err := handler.client.ArchivePolicy(
		request.Context(),
		&policyv1.ArchivePolicyRequest{
			ActorUserId:             principal.UserID,
			TenantId:                tenantID(request),
			PolicyId:                chi.URLParam(request, "policyID"),
			ExpectedResourceVersion: input.ExpectedResourceVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}
