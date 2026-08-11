package modelaccess

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func decodeJSON(
	writer http.ResponseWriter,
	request *http.Request,
	target any,
) error {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}

	return nil
}

func writeProto(
	writer http.ResponseWriter,
	statusCode int,
	message interface {
		ProtoReflect() protoreflect.Message
	},
) {
	value, err := protojson.MarshalOptions{
		UseProtoNames:   false,
		EmitUnpopulated: false,
	}.Marshal(message)
	if err != nil {
		writeProblem(
			writer,
			http.StatusInternalServerError,
			"encoding_failed",
			"Response encoding failed",
		)

		return
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")

	writer.WriteHeader(statusCode)

	_, _ = writer.Write(value)
}

func (handler *Handler) writeGRPCError(
	writer http.ResponseWriter,
	err error,
) {
	switch status.Code(err) {
	case codes.InvalidArgument:
		writeProblem(
			writer,
			http.StatusBadRequest,
			"invalid_request",
			"Invalid Model Access request",
		)

	case codes.NotFound:
		writeProblem(
			writer,
			http.StatusNotFound,
			"not_found",
			"Model Access resource not found",
		)

	case codes.PermissionDenied,
		codes.Unauthenticated:
		writeProblem(
			writer,
			http.StatusForbidden,
			"forbidden",
			"Model Access operation forbidden",
		)

	case codes.AlreadyExists:
		writeProblem(
			writer,
			http.StatusConflict,
			"conflict",
			"Model Access request conflicts with an existing resource",
		)

	case codes.Aborted:
		writeProblem(
			writer,
			http.StatusConflict,
			"version_conflict",
			"Model connection changed; reload and retry",
		)

	case codes.FailedPrecondition:
		writeProblem(
			writer,
			http.StatusConflict,
			"failed_precondition",
			"Model Access operation conflicts with the current state",
		)

	case codes.Unavailable,
		codes.DeadlineExceeded:
		writeProblem(
			writer,
			http.StatusServiceUnavailable,
			"model_access_unavailable",
			"Model Access Service is unavailable",
		)

	default:
		handler.logger.ErrorContext(
			context.Background(),
			"Model Access request failed",
			"error",
			err,
		)

		writeProblem(
			writer,
			http.StatusInternalServerError,
			"model_access_error",
			"Model Access operation failed",
		)
	}
}

func writeProblem(
	writer http.ResponseWriter,
	statusCode int,
	problemType string,
	title string,
) {
	writer.Header().Set("Content-Type", "application/problem+json")
	writer.Header().Set("Cache-Control", "no-store")

	writer.WriteHeader(statusCode)

	_ = json.NewEncoder(writer).Encode(
		map[string]any{
			"type":   problemType,
			"title":  title,
			"status": statusCode,
		},
	)
}
