package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasicAuth(t *testing.T) {
	protected := basicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), "admin", "correct-horse-battery-staple")

	tests := []struct {
		name          string
		username      string
		password      string
		wantStatus    int
		wantChallenge bool
	}{
		{name: "missing credentials", wantStatus: http.StatusUnauthorized, wantChallenge: true},
		{name: "wrong password", username: "admin", password: "wrong", wantStatus: http.StatusUnauthorized, wantChallenge: true},
		{name: "wrong username", username: "other", password: "correct-horse-battery-staple", wantStatus: http.StatusUnauthorized, wantChallenge: true},
		{name: "valid credentials", username: "admin", password: "correct-horse-battery-staple", wantStatus: http.StatusNoContent},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if test.username != "" || test.password != "" {
				request.SetBasicAuth(test.username, test.password)
			}
			response := httptest.NewRecorder()

			protected.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			hasChallenge := response.Header().Get("WWW-Authenticate") != ""
			if hasChallenge != test.wantChallenge {
				t.Fatalf("WWW-Authenticate present = %t, want %t", hasChallenge, test.wantChallenge)
			}
		})
	}
}

func TestBasicAuthAllowsHealthCheck(t *testing.T) {
	protected := basicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "admin", "correct-horse-battery-staple")
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	protected.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("health check status = %d, want %d", response.Code, http.StatusOK)
	}
}
