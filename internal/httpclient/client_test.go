package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_Defaults(t *testing.T) {
	c := New()
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", c.httpClient.Timeout)
	}
}

func TestNewClient_WithTimeout(t *testing.T) {
	want := 10 * time.Second
	c := New(WithTimeout(want))
	if c.httpClient.Timeout != want {
		t.Errorf("expected timeout %v, got %v", want, c.httpClient.Timeout)
	}
}

func TestGetJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	expected := payload{Name: "Alice", Age: 30}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	c := New()
	var got payload
	if err := c.GetJSON(context.Background(), srv.URL, &got); err != nil {
		t.Fatalf("GetJSON returned unexpected error: %v", err)
	}
	if got.Name != expected.Name || got.Age != expected.Age {
		t.Errorf("GetJSON decoded %+v, want %+v", got, expected)
	}
}

func TestGetJSON_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	c := New()
	var dst interface{}
	err := c.GetJSON(context.Background(), srv.URL, &dst)
	if err == nil {
		t.Fatal("expected error for HTTP 404, got nil")
	}
}

func TestGetBody(t *testing.T) {
	want := "hello, audbookdl"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(want))
	}))
	defer srv.Close()

	c := New()
	body, err := c.GetBody(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("GetBody returned unexpected error: %v", err)
	}
	if string(body) != want {
		t.Errorf("GetBody returned %q, want %q", string(body), want)
	}
}
