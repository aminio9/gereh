package modelaccess

import (
	"net/http"
	"strconv"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	"github.com/go-chi/chi/v5"
)

type setAgentModelBindingRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`

	PrimaryOfferingID   string                      `json:"primaryOfferingId"`
	FastOfferingID      *string                     `json:"fastOfferingId,omitempty"`
	FallbackOfferingIDs []string                    `json:"fallbackOfferingIds,omitempty"`
	FallbackPolicy      modelv1.ModelFallbackPolicy `json:"fallbackPolicy,omitempty"`

	MaxModelCostMicroUSD *int64 `json:"maxModelCostMicroUsd,omitempty"`
}

type removeAgentModelBindingRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

func agentID(request *http.Request) string {
	return chi.URLParam(request, "agentID")
}

func refreshID(request *http.Request) string {
	return chi.URLParam(request, "refreshID")
}

func (handler *Handler) listModelOfferings(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	var connID *string
	if cid := request.URL.Query().Get("connection_id"); cid != "" {
		connID = &cid
	}

	response, err := handler.client.ListModelOfferings(
		request.Context(),
		&modelv1.ListModelOfferingsRequest{
			ActorUserId:  principal.UserID,
			TenantId:     tenantID(request),
			ConnectionId: connID,
			PageSize:     int32(pageSize),
			PageToken:    request.URL.Query().Get("page_token"),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) refreshModelCatalog(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	key, ok := idempotencyKey(writer, request)
	if !ok {
		return
	}

	response, err := handler.client.RefreshModelCatalog(
		request.Context(),
		&modelv1.RefreshModelCatalogRequest{
			ActorUserId:    principal.UserID,
			TenantId:       tenantID(request),
			ConnectionId:   connectionID(request),
			IdempotencyKey: key,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusAccepted, response)
}

func (handler *Handler) getModelCatalogRefresh(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetModelCatalogRefresh(
		request.Context(),
		&modelv1.GetModelCatalogRefreshRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			RefreshId:   refreshID(request),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) getAgentModelBinding(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetAgentModelBinding(
		request.Context(),
		&modelv1.GetAgentModelBindingRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
			AgentId:     agentID(request),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) setAgentModelBinding(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	key, ok := idempotencyKey(writer, request)
	if !ok {
		return
	}

	var input setAgentModelBindingRequest
	if err := decodeJSON(writer, request, &input); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid agent model binding request",
		)
		return
	}

	response, err := handler.client.SetAgentModelBinding(
		request.Context(),
		&modelv1.SetAgentModelBindingRequest{
			ActorUserId:          principal.UserID,
			TenantId:             tenantID(request),
			AgentId:              agentID(request),
			IdempotencyKey:       key,
			ExpectedVersion:      input.ExpectedVersion,
			PrimaryOfferingId:    input.PrimaryOfferingID,
			FastOfferingId:       input.FastOfferingID,
			FallbackOfferingIds:  input.FallbackOfferingIDs,
			FallbackPolicy:       input.FallbackPolicy,
			MaxModelCostMicrousd: input.MaxModelCostMicroUSD,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) removeAgentModelBinding(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	key, ok := idempotencyKey(writer, request)
	if !ok {
		return
	}

	var input removeAgentModelBindingRequest
	if err := decodeJSON(writer, request, &input); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid remove agent model binding request",
		)
		return
	}

	response, err := handler.client.RemoveAgentModelBinding(
		request.Context(),
		&modelv1.RemoveAgentModelBindingRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			AgentId:         agentID(request),
			IdempotencyKey:  key,
			ExpectedVersion: input.ExpectedVersion,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}
