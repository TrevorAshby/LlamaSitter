package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/trevorashby/llamasitter/internal/config"
	"github.com/trevorashby/llamasitter/internal/identity"
	"github.com/trevorashby/llamasitter/internal/model"
	"github.com/trevorashby/llamasitter/internal/usage"
)

type Recorder interface {
	InsertRequest(context.Context, *model.RequestEvent) error
}

type Service struct {
	listeners []listener
	store     Recorder
	logger    *slog.Logger
	client    *http.Client
}

type listener struct {
	cfg      config.ListenerConfig
	upstream *url.URL
	server   *http.Server
}

func NewService(configs []config.ListenerConfig, store Recorder, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	service := &Service{
		store:  store,
		logger: logger,
		client: &http.Client{},
	}

	for _, cfg := range configs {
		upstream, err := url.Parse(cfg.UpstreamURL)
		if err != nil {
			return nil, fmt.Errorf("parse upstream for listener %q: %w", cfg.Name, err)
		}

		item := listener{
			cfg:      cfg,
			upstream: upstream,
		}
		handler := service.listenerHandler(item)
		item.server = &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}

		service.listeners = append(service.listeners, item)
	}

	return service, nil
}

func (s *Service) Serve(ctx context.Context) error {
	errCh := make(chan error, len(s.listeners))

	for i := range s.listeners {
		item := s.listeners[i]
		s.logger.Info("starting proxy listener", "listener", item.cfg.Name, "addr", item.cfg.ListenAddr, "upstream", item.cfg.UpstreamURL)
		go func(l listener) {
			err := l.server.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("proxy listener %q: %w", l.cfg.Name, err)
			}
		}(item)
	}

	select {
	case err := <-errCh:
		_ = s.shutdown()
		return err
	case <-ctx.Done():
		return s.shutdown()
	}
}

func (s *Service) shutdown() error {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error
	for _, item := range s.listeners {
		if err := item.server.Shutdown(timeoutCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Service) listenerHandler(item listener) http.Handler {
	fallback := httputil.NewSingleHostReverseProxy(item.upstream)
	fallback.FlushInterval = 100 * time.Millisecond
	fallback.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/chat" && r.Method == http.MethodPost {
			s.handleChat(item, w, r)
			return
		}
		fallback.ServeHTTP(w, r)
	})
}

func (s *Service) handleChat(item listener, w http.ResponseWriter, r *http.Request) {
	startedAt := time.Now().UTC()
	requestID := newRequestID()

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	chatReq, reqParseErr := usage.ParseChatRequest(requestBody)

	resolved := identity.Resolve(r.Header, item.cfg.DefaultTags)
	event := model.RequestEvent{
		RequestID:        requestID,
		ListenerName:     item.cfg.Name,
		StartedAt:        startedAt,
		Method:           r.Method,
		Endpoint:         r.URL.Path,
		Model:            chatReq.Model,
		RequestSizeBytes: int64(len(requestBody)),
		Identity:         resolved.Identity,
		Tags:             resolved.ExtraTags,
	}

	upstreamReq, err := s.buildUpstreamRequest(r, item.upstream, requestBody)
	if err != nil {
		http.Error(w, "failed to prepare upstream request", http.StatusInternalServerError)
		return
	}

	resp, err := s.client.Do(upstreamReq)
	if err != nil {
		event.FinishedAt = time.Now().UTC()
		event.HTTPStatus = http.StatusBadGateway
		event.ErrorMessage = err.Error()
		event.Success = false
		event.RequestDurationMs = event.FinishedAt.Sub(event.StartedAt).Milliseconds()
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		s.persistAsync(event)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	streaming := resp.StatusCode < http.StatusBadRequest && usage.IsStreamingResponse(resp, chatReq)
	if reqParseErr != nil && !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "application/x-ndjson") {
		streaming = false
	}
	event.Stream = streaming

	if streaming {
		s.handleStreamingResponse(w, resp, &event)
	} else {
		s.handleBufferedResponse(w, resp, &event)
	}

	event.FinishedAt = time.Now().UTC()
	event.RequestDurationMs = event.FinishedAt.Sub(event.StartedAt).Milliseconds()
	event.Success = event.HTTPStatus < http.StatusBadRequest && !event.Aborted && event.ErrorMessage == ""
	s.persistAsync(event)
}

func (s *Service) handleBufferedResponse(w http.ResponseWriter, resp *http.Response, event *model.RequestEvent) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		event.HTTPStatus = resp.StatusCode
		event.ErrorMessage = err.Error()
		return
	}

	event.ResponseSizeBytes = int64(len(body))
	event.HTTPStatus = resp.StatusCode

	if _, err := w.Write(body); err != nil {
		event.Aborted = true
		event.ErrorMessage = err.Error()
		return
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return
	}

	parsed, err := usage.ExtractNonStream(body)
	if err != nil {
		s.logger.Debug("unable to parse non-stream response", "err", err)
		return
	}
	usage.Apply(event, parsed)
}

func (s *Service) handleStreamingResponse(w http.ResponseWriter, resp *http.Response, event *model.RequestEvent) {
	event.HTTPStatus = resp.StatusCode

	flusher, _ := w.(http.Flusher)
	reader := bufio.NewReader(resp.Body)

	var finalUsage *usage.UsageData

	for {
		part, err := reader.ReadBytes('\n')
		if len(part) > 0 {
			event.ResponseSizeBytes += int64(len(part))
			if _, writeErr := w.Write(part); writeErr != nil {
				event.Aborted = true
				event.ErrorMessage = writeErr.Error()
				return
			}
			if flusher != nil {
				flusher.Flush()
			}

			parsed, ok, parseErr := usage.ExtractStreamPart(part)
			if parseErr != nil {
				s.logger.Debug("unable to parse stream chunk", "err", parseErr)
			} else if ok && parsed.Done {
				usageCopy := parsed
				finalUsage = &usageCopy
			}
		}

		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		event.ErrorMessage = err.Error()
		break
	}

	if finalUsage == nil {
		event.Aborted = true
		return
	}

	usage.Apply(event, *finalUsage)
}

func (s *Service) persistAsync(event model.RequestEvent) {
	if s.store == nil {
		return
	}

	go func(ev model.RequestEvent) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.store.InsertRequest(ctx, &ev); err != nil {
			s.logger.Error("failed to persist request", "request_id", ev.RequestID, "err", err)
		}
	}(event)
}

func (s *Service) buildUpstreamRequest(r *http.Request, upstream *url.URL, body []byte) (*http.Request, error) {
	targetURL := *upstream
	targetURL.Path = joinPath(upstream.Path, r.URL.Path)
	targetURL.RawQuery = r.URL.RawQuery

	req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header = r.Header.Clone()
	req.ContentLength = int64(len(body))
	req.Host = upstream.Host
	removeHopHeaders(req.Header)

	return req, nil
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		if isHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func removeHopHeaders(header http.Header) {
	for key := range header {
		if isHopHeader(key) {
			header.Del(key)
		}
	}
}

func isHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "proxy-connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailers", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return "req-" + hex.EncodeToString(buf)
}

func joinPath(basePath, requestPath string) string {
	if basePath == "" {
		return requestPath
	}
	return path.Join("/", basePath, requestPath)
}
