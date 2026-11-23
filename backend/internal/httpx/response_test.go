package httpx_test

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iliaatmazhitov/nexus/internal/httpx"
)

type errorBody struct {
	Code string `json:"code"`
}

type errorResponse struct {
	Error errorBody `json:"error"`
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	httpx.WriteJSON(context.Background(), rec, http.StatusOK, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Errorf("httpStatus: got %d, want %d", rec.Code, http.StatusOK)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status: got %q, want %q", body["status"], "ok")
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	testHTTPStatus := http.StatusTeapot
	testErrCode := "test_write"
	httpx.WriteError(context.Background(), rec, testHTTPStatus, testErrCode, "message")

	if rec.Code != testHTTPStatus {
		t.Errorf("httpStatus: got %d, want %d", rec.Code, testHTTPStatus)
	}

	var errResponse errorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResponse)
	if err != nil {
		t.Fatalf("error: got %v, want %v", err, nil)
	}

	if errResponse.Error.Code != testErrCode {
		t.Errorf("error.code: got %q, want %q", errResponse.Error.Code, testErrCode)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q, want %q", ct, "application/json")
	}
}

func TestWriteJSONEncodeFailure(t *testing.T) {
	rec := httptest.NewRecorder()
	sendHTTPStatus := http.StatusTeapot
	expectedHTTPStatus := http.StatusInternalServerError
	noJSONValue := math.NaN()

	httpx.WriteJSON(context.Background(), rec, sendHTTPStatus, noJSONValue)

	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("body is not valid JSON: %s", rec.Body.String())
	}

	if rec.Code != expectedHTTPStatus {
		t.Errorf("httpStatus: got %d, want %d", rec.Code, expectedHTTPStatus)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q, want %q", ct, "application/json")
	}

	var errResponse errorResponse
	err := json.NewDecoder(rec.Body).Decode(&errResponse)
	if err != nil {
		t.Fatalf("error: got %v, want %v", err, nil)
	}

	expectedErrCode := "internal"
	if errResponse.Error.Code != expectedErrCode {
		t.Errorf("error.code: got %q, want %q", errResponse.Error.Code, expectedErrCode)
	}
}
