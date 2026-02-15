package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	// No configured key: pass-through.
	pass := AuthMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	passRR := httptest.NewRecorder()
	pass.ServeHTTP(passRR, httptest.NewRequest(http.MethodGet, "/", nil))
	if passRR.Code != http.StatusNoContent {
		t.Fatalf("pass-through status = %d, want %d", passRR.Code, http.StatusNoContent)
	}

	// Configured key: wrong key rejected.
	protected := AuthMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	wrongReq := httptest.NewRequest(http.MethodGet, "/", nil)
	wrongReq.Header.Set("X-Gasoline-Key", "nope")
	wrongRR := httptest.NewRecorder()
	protected.ServeHTTP(wrongRR, wrongReq)
	if wrongRR.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-key status = %d, want %d", wrongRR.Code, http.StatusUnauthorized)
	}
	var body map[string]string
	if err := json.Unmarshal(wrongRR.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("error body = %q, want unauthorized", body["error"])
	}

	okReq := httptest.NewRequest(http.MethodGet, "/", nil)
	okReq.Header.Set("X-Gasoline-Key", "secret")
	okRR := httptest.NewRecorder()
	protected.ServeHTTP(okRR, okReq)
	if okRR.Code != http.StatusAccepted {
		t.Fatalf("correct-key status = %d, want %d", okRR.Code, http.StatusAccepted)
	}
}

func TestHandleOpenAPIAndLogLevelRank(t *testing.T) {
	t.Parallel()

	badReq := httptest.NewRequest(http.MethodPost, "/openapi.json", nil)
	badRR := httptest.NewRecorder()
	handleOpenAPI(badRR, badReq)
	if badRR.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /openapi.json status = %d, want %d", badRR.Code, http.StatusMethodNotAllowed)
	}

	okReq := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	okRR := httptest.NewRecorder()
	handleOpenAPI(okRR, okReq)
	if okRR.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json status = %d, want %d", okRR.Code, http.StatusOK)
	}
	if got := okRR.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	if len(okRR.Body.Bytes()) == 0 {
		t.Fatal("openapi response body should not be empty")
	}

	cases := map[string]int{
		"debug": 0,
		"log":   1,
		"info":  2,
		"warn":  3,
		"error": 4,
		"other": -1,
	}
	for level, want := range cases {
		if got := logLevelRank(level); got != want {
			t.Fatalf("logLevelRank(%q) = %d, want %d", level, got, want)
		}
	}
}
