package usage

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/trevorashby/llamasitter/internal/model"
)

type ChatRequest struct {
	Model  string
	Stream bool
}

type UsageData struct {
	Model                 string
	Done                  bool
	PromptTokens          int64
	OutputTokens          int64
	TotalTokens           int64
	TotalDurationMs       int64
	PromptEvalDurationMs  int64
	EvalDurationMs        int64
}

type ollamaEnvelope struct {
	Model              string `json:"model"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`
	PromptEvalCount    int64  `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int64  `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

func ParseChatRequest(body []byte) (ChatRequest, error) {
	type payload struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}

	var decoded payload
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ChatRequest{}, err
	}

	stream := true
	if decoded.Stream != nil {
		stream = *decoded.Stream
	}

	return ChatRequest{
		Model:  decoded.Model,
		Stream: stream,
	}, nil
}

func ExtractNonStream(body []byte) (UsageData, error) {
	var decoded ollamaEnvelope
	if err := json.Unmarshal(body, &decoded); err != nil {
		return UsageData{}, err
	}
	return usageFromEnvelope(decoded), nil
}

func ExtractStreamPart(part []byte) (UsageData, bool, error) {
	part = bytes.TrimSpace(part)
	if len(part) == 0 {
		return UsageData{}, false, nil
	}

	part = bytes.TrimPrefix(part, []byte("data:"))
	part = bytes.TrimSpace(part)
	if bytes.Equal(part, []byte("[DONE]")) || len(part) == 0 {
		return UsageData{}, false, nil
	}

	var decoded ollamaEnvelope
	if err := json.Unmarshal(part, &decoded); err != nil {
		return UsageData{}, false, err
	}

	return usageFromEnvelope(decoded), true, nil
}

func IsStreamingResponse(resp *http.Response, chatReq ChatRequest) bool {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/x-ndjson") || strings.Contains(contentType, "text/event-stream") {
		return true
	}
	return chatReq.Stream
}

func Apply(event *model.RequestEvent, data UsageData) {
	if data.Model != "" && event.Model == "" {
		event.Model = data.Model
	}
	event.PromptTokens = data.PromptTokens
	event.OutputTokens = data.OutputTokens
	event.TotalTokens = data.TotalTokens
	event.PromptEvalDurationMs = data.PromptEvalDurationMs
	event.EvalDurationMs = data.EvalDurationMs
	event.UpstreamTotalDurationMs = data.TotalDurationMs
}

func usageFromEnvelope(decoded ollamaEnvelope) UsageData {
	totalTokens := decoded.PromptEvalCount + decoded.EvalCount
	return UsageData{
		Model:                decoded.Model,
		Done:                 decoded.Done,
		PromptTokens:         decoded.PromptEvalCount,
		OutputTokens:         decoded.EvalCount,
		TotalTokens:          totalTokens,
		TotalDurationMs:      decoded.TotalDuration / 1_000_000,
		PromptEvalDurationMs: decoded.PromptEvalDuration / 1_000_000,
		EvalDurationMs:       decoded.EvalDuration / 1_000_000,
	}
}
