package httptools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grantsy/grantsy/internal/httptools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSON_StatusAndContentType(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.JSON(w, r, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestJSON_ResponseFormat(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.JSON(w, r, http.StatusOK, map[string]string{"key": "value"})

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp, "data")
	assert.Contains(t, resp, "meta")

	var meta map[string]string
	require.NoError(t, json.Unmarshal(resp["meta"], &meta))
	assert.Equal(t, "1.0", meta["version"])
}

func TestJSON_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.JSON(w, r, http.StatusOK, nil)

	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotContains(t, resp, "data")
	assert.Contains(t, resp, "meta")
}

func TestError_RFC9457Format(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.Error(w, r, http.StatusBadRequest,
		"https://example.com/errors/test",
		"Test Error",
		"something went wrong",
	)

	var resp struct {
		Error struct {
			Type      string `json:"type"`
			Title     string `json:"title"`
			Detail    string `json:"detail"`
			Status    int    `json:"status"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "https://example.com/errors/test", resp.Error.Type)
	assert.Equal(t, "Test Error", resp.Error.Title)
	assert.Equal(t, "something went wrong", resp.Error.Detail)
	assert.Equal(t, http.StatusBadRequest, resp.Error.Status)
	assert.Empty(t, resp.Error.RequestID) // no request ID in context
}

func TestError_StatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.Error(w, r, http.StatusForbidden, "t", "t", "t")

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestValidationError_Fields(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	fields := []httptools.FieldError{
		{Field: "email", Message: "is required"},
		{Field: "name", Message: "too short"},
	}
	httptools.ValidationError(w, r, fields)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var resp struct {
		Error struct {
			Fields []httptools.FieldError `json:"fields"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp.Error.Fields, 2)
	assert.Equal(t, "email", resp.Error.Fields[0].Field)
	assert.Equal(t, "is required", resp.Error.Fields[0].Message)
}

func TestValidationError_EmptyFields(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.ValidationError(w, r, []httptools.FieldError{})

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var resp struct {
		Error struct {
			Fields []httptools.FieldError `json:"fields"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Empty(t, resp.Error.Fields)
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.InternalError(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp struct {
		Error struct {
			Type  string `json:"type"`
			Title string `json:"title"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "https://grantsy.example/errors/internal-error", resp.Error.Type)
	assert.Equal(t, "Internal Server Error", resp.Error.Title)
}

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	httptools.BadRequest(w, r, "missing parameter")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp struct {
		Error struct {
			Detail string `json:"detail"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "missing parameter", resp.Error.Detail)
}

func TestWriteStatus(t *testing.T) {
	w := httptest.NewRecorder()

	httptools.WriteStatus(w, http.StatusNoContent)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Body.String())
}
