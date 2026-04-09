package proxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/trevorashby/llamasitter/internal/config"
	"github.com/trevorashby/llamasitter/internal/model"
)

type recorderStore struct {
	ch chan model.RequestEvent
}

func (r *recorderStore) InsertRequest(_ context.Context, event *model.RequestEvent) error {
	r.ch <- *event
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHandleChatBufferedResponse(t *testing.T) {
	t.Parallel()

	store := &recorderStore{ch: make(chan model.RequestEvent, 1)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service, err := NewService([]config.ListenerConfig{
		{
			Name:        "alpha",
			ListenAddr:  "127.0.0.1:11435",
			UpstreamURL: "http://ollama.local",
			DefaultTags: map[string]string{
				"client_instance": "instance-a",
			},
		},
	}, store, logger)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/json"},
				},
				Body: io.NopCloser(strings.NewReader(`{"model":"llama3","done":true,"prompt_eval_count":5,"eval_count":7,"total_duration":900000000}`)),
			}, nil
		}),
	}

	req := httptest.NewRequest(http.MethodPost, "http://llamasitter/api/chat", strings.NewReader(`{"model":"llama3","stream":false}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler := service.listenerHandler(service.listeners[0])
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"llama3"`) {
		t.Fatalf("expected upstream body to pass through")
	}

	select {
	case event := <-store.ch:
		if event.TotalTokens != 12 {
			t.Fatalf("expected 12 total tokens, got %d", event.TotalTokens)
		}
		if event.ClientInstance != "instance-a" {
			t.Fatalf("expected default client instance, got %q", event.ClientInstance)
		}
		if event.Stream {
			t.Fatalf("expected buffered request to be marked non-stream")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for persisted event")
	}
}

func TestHandleChatStreamingResponse(t *testing.T) {
	t.Parallel()

	store := &recorderStore{ch: make(chan model.RequestEvent, 1)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service, err := NewService([]config.ListenerConfig{
		{
			Name:        "alpha",
			ListenAddr:  "127.0.0.1:11435",
			UpstreamURL: "http://ollama.local",
		},
	}, store, logger)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	service.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := strings.NewReader(
				`{"model":"llama3","done":false}` + "\n" +
					`{"model":"llama3","done":true,"prompt_eval_count":3,"eval_count":11,"total_duration":1100000000}` + "\n",
			)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header: http.Header{
					"Content-Type": []string{"application/x-ndjson"},
				},
				Body: io.NopCloser(body),
			}, nil
		}),
	}

	req := httptest.NewRequest(http.MethodPost, "http://llamasitter/api/chat", strings.NewReader(`{"model":"llama3","stream":true}`))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler := service.listenerHandler(service.listeners[0])
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"done":true`) {
		t.Fatalf("expected final streamed chunk in response body")
	}

	select {
	case event := <-store.ch:
		if !event.Stream {
			t.Fatalf("expected streaming request to be marked stream")
		}
		if event.TotalTokens != 14 {
			t.Fatalf("expected 14 total tokens, got %d", event.TotalTokens)
		}
		if event.Aborted {
			t.Fatalf("expected completed stream")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for persisted event")
	}
}
