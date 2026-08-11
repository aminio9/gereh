package modelaccess

import (
	"net/http"
	"strconv"
	"strings"

	modelv1 "github.com/aminio9/gereh/gen/go/gereh/model/v1"
	platformauth "github.com/aminio9/gereh/platform/go/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type createConnectionRequest struct {
	ProviderKey string `json:"providerKey"`

	ConnectionType modelv1.ModelConnectionType `json:"connectionType"`

	DisplayName string `json:"displayName"`

	// Write-only. Never log this struct.
	APIKey string `json:"apiKey,omitempty"`
}

type updateConnectionRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`

	DisplayName string `json:"displayName"`
}

type archiveConnectionRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`
}

type rotateBYOKCredentialRequest struct {
	ExpectedVersion int64 `json:"expectedVersion"`

	// Write-only.
	APIKey string `json:"apiKey"`
}

func principal(
	request *http.Request,
) (platformauth.Principal, bool) {
	return platformauth.PrincipalFromContext(request.Context())
}

func tenantID(request *http.Request) string {
	return chi.URLParam(request, "tenantID")
}

func connectionID(request *http.Request) string {
	return chi.URLParam(request, "connectionID")
}

func idempotencyKey(
	writer http.ResponseWriter,
	request *http.Request,
) (string, bool) {
	value := strings.TrimSpace(
		request.Header.Get("Idempotency-Key"),
	)

	if _, err := uuid.Parse(value); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"idempotency_key_required",
			"Idempotency-Key must be a UUID",
		)

		return "", false
	}

	return value, true
}

func (handler *Handler) listProviders(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.ListProviders(
		request.Context(),
		&modelv1.ListProvidersRequest{
			ActorUserId: principal.UserID,
			TenantId:    tenantID(request),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) createConnection(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	key, ok := idempotencyKey(writer, request)
	if !ok {
		return
	}

	var input createConnectionRequest

	if err := decodeJSON(writer, request, &input); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid model connection request",
		)
		return
	}

	if input.ConnectionType ==
		modelv1.ModelConnectionType_MODEL_CONNECTION_TYPE_BYOK {
		if strings.TrimSpace(input.APIKey) == "" {
			writeProblem(
				writer,
				http.StatusBadRequest,
				"credential_required",
				"API key is required for a BYOK connection",
			)
			return
		}

		response, err := handler.client.CreateBYOKConnection(
			request.Context(),
			&modelv1.CreateBYOKConnectionRequest{
				ActorUserId:    principal.UserID,
				TenantId:       tenantID(request),
				IdempotencyKey: key,
				ProviderKey:    input.ProviderKey,
				DisplayName:    input.DisplayName,
				ApiKey:         input.APIKey,
			},
		)
		if err != nil {
			handler.writeGRPCError(writer, err)
			return
		}

		writeProto(writer, http.StatusCreated, response)
		return
	}

	if input.APIKey != "" {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"unexpected_credential",
			"API key is only valid for a BYOK connection",
		)
		return
	}

	response, err := handler.client.CreateConnection(
		request.Context(),
		&modelv1.CreateConnectionRequest{
			ActorUserId:    principal.UserID,
			TenantId:       tenantID(request),
			IdempotencyKey: key,
			ProviderKey:    input.ProviderKey,
			ConnectionType: input.ConnectionType,
			DisplayName:    input.DisplayName,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusCreated, response)
}

func (handler *Handler) getConnection(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	response, err := handler.client.GetConnection(
		request.Context(),
		&modelv1.GetConnectionRequest{
			ActorUserId:  principal.UserID,
			TenantId:     tenantID(request),
			ConnectionId: connectionID(request),
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) listConnections(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	pageSize, _ := strconv.ParseInt(
		request.URL.Query().Get("page_size"),
		10,
		32,
	)

	includeArchived, _ := strconv.ParseBool(
		request.URL.Query().Get("include_archived"),
	)

	response, err := handler.client.ListConnections(
		request.Context(),
		&modelv1.ListConnectionsRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			PageSize:        int32(pageSize),
			PageToken:       request.URL.Query().Get("page_token"),
			IncludeArchived: includeArchived,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) updateConnection(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	key, ok := idempotencyKey(writer, request)
	if !ok {
		return
	}

	var input updateConnectionRequest

	if err := decodeJSON(writer, request, &input); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid model connection request",
		)
		return
	}

	response, err := handler.client.UpdateConnection(
		request.Context(),
		&modelv1.UpdateConnectionRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			ConnectionId:    connectionID(request),
			IdempotencyKey:  key,
			ExpectedVersion: input.ExpectedVersion,
			DisplayName:     &input.DisplayName,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}

func (handler *Handler) archiveConnection(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	key, ok := idempotencyKey(writer, request)
	if !ok {
		return
	}

	var input archiveConnectionRequest

	if err := decodeJSON(writer, request, &input); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid model connection archive request",
		)
		return
	}

	response, err := handler.client.ArchiveConnection(
		request.Context(),
		&modelv1.ArchiveConnectionRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			ConnectionId:    connectionID(request),
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

func (handler *Handler) rotateBYOKCredential(
	writer http.ResponseWriter,
	request *http.Request,
) {
	principal, _ := principal(request)

	key, ok := idempotencyKey(writer, request)
	if !ok {
		return
	}

	var input rotateBYOKCredentialRequest

	if err := decodeJSON(writer, request, &input); err != nil {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid credential rotation request",
		)
		return
	}

	if strings.TrimSpace(input.APIKey) == "" {
		writeProblem(
			writer,
			http.StatusBadRequest,
			"credential_required",
			"API key is required",
		)
		return
	}

	response, err := handler.client.RotateBYOKCredential(
		request.Context(),
		&modelv1.RotateBYOKCredentialRequest{
			ActorUserId:     principal.UserID,
			TenantId:        tenantID(request),
			ConnectionId:    connectionID(request),
			IdempotencyKey:  key,
			ExpectedVersion: input.ExpectedVersion,
			ApiKey:          input.APIKey,
		},
	)
	if err != nil {
		handler.writeGRPCError(writer, err)
		return
	}

	writeProto(writer, http.StatusOK, response)
}
