package response

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func HandleGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		WriteJSONError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	switch st.Code() {
	case codes.InvalidArgument:
		WriteJSONError(w, http.StatusBadRequest, st.Message())
	case codes.NotFound:
		WriteJSONError(w, http.StatusNotFound, st.Message())
	case codes.AlreadyExists:
		WriteJSONError(w, http.StatusConflict, st.Message())
	case codes.PermissionDenied:
		WriteJSONError(w, http.StatusForbidden, st.Message())
	case codes.Unauthenticated:
		WriteJSONError(w, http.StatusUnauthorized, st.Message())
	default:
		WriteJSONError(w, http.StatusInternalServerError, "internal server error")
	}
}

func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	WriteJSON(w, statusCode, map[string]string{"error": message})
}

func WriteJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}
