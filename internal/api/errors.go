package api

import (
	"context"
	"errors"
	"net/http"
	"os"
)

func writeFailure(w http.ResponseWriter, fallbackStatus int, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		writeErrorCode(w, http.StatusGatewayTimeout, "timeout", "operation timed out", true, "check connectivity and retry with a longer timeout")
	case errors.Is(err, context.Canceled):
		writeErrorCode(w, http.StatusRequestTimeout, "cancelled", "operation was cancelled", false, "start a new request if the operation is still needed")
	case errors.Is(err, os.ErrPermission):
		writeErrorCode(w, http.StatusForbidden, "filesystem_permission_denied", "filesystem access was denied", false, "check the selected directory's permissions")
	case errors.Is(err, os.ErrNotExist):
		writeErrorCode(w, http.StatusNotFound, "path_not_found", "selected path or executable is unavailable", false, "check the path and selected engine")
	default:
		writeError(w, fallbackStatus, err.Error())
	}
}

func (a *Server) writeAppFailure(w http.ResponseWriter, fallbackStatus int, err error) {
	if health := a.app.Health(); !health.Ready {
		writeErrorCode(w, http.StatusServiceUnavailable, health.Code, health.Message, false, health.NextAction)
		return
	}
	writeFailure(w, fallbackStatus, err)
}
