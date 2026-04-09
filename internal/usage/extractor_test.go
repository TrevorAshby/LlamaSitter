package usage

import "testing"

func TestExtractNonStream(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"model":"llama3",
		"done":true,
		"total_duration":1200000000,
		"prompt_eval_count":12,
		"prompt_eval_duration":400000000,
		"eval_count":34,
		"eval_duration":700000000
	}`)

	data, err := ExtractNonStream(body)
	if err != nil {
		t.Fatalf("extract non-stream: %v", err)
	}

	if data.Model != "llama3" {
		t.Fatalf("unexpected model %q", data.Model)
	}
	if data.TotalTokens != 46 {
		t.Fatalf("expected total tokens 46, got %d", data.TotalTokens)
	}
	if data.TotalDurationMs != 1200 {
		t.Fatalf("expected total duration 1200, got %d", data.TotalDurationMs)
	}
}

func TestExtractStreamPart(t *testing.T) {
	t.Parallel()

	part := []byte(`{"model":"llama3","done":true,"prompt_eval_count":4,"eval_count":9,"total_duration":900000000}` + "\n")
	data, ok, err := ExtractStreamPart(part)
	if err != nil {
		t.Fatalf("extract stream part: %v", err)
	}
	if !ok {
		t.Fatalf("expected chunk to parse")
	}
	if !data.Done {
		t.Fatalf("expected done chunk")
	}
	if data.TotalTokens != 13 {
		t.Fatalf("expected total tokens 13, got %d", data.TotalTokens)
	}
}
