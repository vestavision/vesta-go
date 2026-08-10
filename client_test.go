package vesta_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	vesta "github.com/vestavision/vesta-go"
)

func TestClientListAndSubmit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/models", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Key vh_test" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "unauthorized", "message": "unauthorized"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"model_id": "vesta/mock-echo"}}})
	})
	mux.HandleFunc("/api/v1/models/vesta/mock-echo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"request_id": "req_1", "status": "queued", "model_id": "vesta/mock-echo"})
	})
	mux.HandleFunc("/api/v1/requests/req_1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"request_id": "req_1", "status": "completed", "output": map[string]any{"ok": true}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := vesta.NewClient("vh_test", vesta.WithBaseURL(srv.URL), vesta.WithHTTPClient(srv.Client()))
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ModelID != "vesta/mock-echo" {
		t.Fatalf("models=%v", models)
	}
	req, err := client.Submit(context.Background(), "vesta/mock-echo", vesta.SubmitOptions{
		Input: map[string]any{"ping": true}, IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatal(err)
	}
	done, err := client.Wait(context.Background(), req.RequestID, 10*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "completed" {
		t.Fatalf("status=%s", done.Status)
	}
}

func TestVerifyWebhookSignature(t *testing.T) {
	ok := vesta.VerifyWebhookSignature("secret", []byte(`{"a":1}`), "not-valid")
	if ok {
		t.Fatal("expected false")
	}
}
