package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHomeHandler(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(homeHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// You could also check for specific strings in rr.Body.String()
}

func TestBoardHandlerNotFound(t *testing.T) {
	setupTestDB(t)
	defer teardownTestDB()

	req, err := http.NewRequest("GET", "/nonexistent/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(boardHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusNotFound)
	}
}

func TestBasicAuth(t *testing.T) {
	handler := BasicAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, "user", "pass")

	// Test without auth
	req, _ := http.NewRequest("GET", "/admin", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", rr.Code)
	}

	// Test with wrong auth
	req, _ = http.NewRequest("GET", "/admin", nil)
	req.SetBasicAuth("user", "wrong")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized with wrong pass, got %d", rr.Code)
	}

	// Test with correct auth
	req, _ = http.NewRequest("GET", "/admin", nil)
	req.SetBasicAuth("user", "pass")
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK with correct auth, got %d", rr.Code)
	}
}
