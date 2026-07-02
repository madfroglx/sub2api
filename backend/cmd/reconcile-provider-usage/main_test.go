package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildGatewayRequestChatCompletions(t *testing.T) {
	cfg := RunConfig{
		BaseURL:   "http://127.0.0.1:8080/",
		APIKey:    "sk-test",
		Endpoint:  EndpointChatCompletions,
		Model:     "gpt-5.4-mini",
		Prompt:    "say ok",
		RunID:     "reconcile-run",
		MaxTokens: 8,
	}

	req, err := buildGatewayRequest(context.Background(), cfg, 2)
	if err != nil {
		t.Fatalf("buildGatewayRequest returned error: %v", err)
	}
	if req.Method != "POST" {
		t.Fatalf("method = %s, want POST", req.Method)
	}
	if req.URL.String() != "http://127.0.0.1:8080/v1/chat/completions" {
		t.Fatalf("url = %s", req.URL.String())
	}
	if got := req.Header.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("X-Client-Request-ID"); got != "reconcile-run-0002" {
		t.Fatalf("X-Client-Request-ID = %q", got)
	}

	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["model"] != "gpt-5.4-mini" {
		t.Fatalf("model = %#v", body["model"])
	}
	messages, ok := body["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %#v", body["messages"])
	}
	msg, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("message[0] = %#v", messages[0])
	}
	content, _ := msg["content"].(string)
	if !strings.Contains(content, "say ok") || !strings.Contains(content, "reconcile_run_id=reconcile-run") {
		t.Fatalf("content = %q", content)
	}
}

func TestBuildGatewayRequestMixedPromptProfileAddsStructuredTokenWorkload(t *testing.T) {
	cfg := RunConfig{
		BaseURL:            "http://127.0.0.1:8080/",
		APIKey:             "sk-test",
		Endpoint:           EndpointMessages,
		Model:              "claude-sonnet-4",
		Prompt:             "Summarize the workload.",
		PromptProfile:      PromptProfileMixed,
		RunID:              "reconcile-run",
		MaxTokens:          96,
		OutputLines:        4,
		OutputWordsPerLine: 6,
	}

	req, err := buildGatewayRequest(context.Background(), cfg, 1)
	if err != nil {
		t.Fatalf("buildGatewayRequest returned error: %v", err)
	}

	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	messages, ok := body["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("messages = %#v", body["messages"])
	}
	msg, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("message[0] = %#v", messages[0])
	}
	content, _ := msg["content"].(string)
	for _, want := range []string{
		"Structured token-count reconciliation workload",
		"中文片段",
		"```json",
		"```go",
		"Return exactly 4 lines.",
		"Each line must contain exactly 6 words",
		"reconcile_run_id=reconcile-run",
		"reconcile_request_id=reconcile-run-0001",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q:\n%s", want, content)
		}
	}
}

func TestBuildGatewayRequestStreamChatCompletionsRequestsUsageInSSE(t *testing.T) {
	cfg := RunConfig{
		BaseURL:   "http://127.0.0.1:8080/",
		APIKey:    "sk-test",
		Endpoint:  EndpointChatCompletions,
		Model:     "deepseek-v4-flash",
		Prompt:    "generate controlled output",
		RunID:     "reconcile-run",
		MaxTokens: 128,
		Stream:    true,
	}

	req, err := buildGatewayRequest(context.Background(), cfg, 1)
	if err != nil {
		t.Fatalf("buildGatewayRequest returned error: %v", err)
	}

	var body map[string]any
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["stream"] != true {
		t.Fatalf("stream = %#v, want true", body["stream"])
	}
	streamOptions, ok := body["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v", body["stream_options"])
	}
	if streamOptions["include_usage"] != true {
		t.Fatalf("include_usage = %#v, want true", streamOptions["include_usage"])
	}
}

func TestUsageFromGatewayResponseParsesSSEUsageChunk(t *testing.T) {
	body := []byte(strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"alpha"}}],"usage":null}`,
		`data: {"choices":[],"usage":{"prompt_tokens":101,"completion_tokens":9500,"total_tokens":9601}}`,
		`data: [DONE]`,
		``,
	}, "\n\n"))

	usage := usageFromGatewayResponse(body)
	if usage.Requests != 1 {
		t.Fatalf("Requests = %d, want 1", usage.Requests)
	}
	if usage.InputTokens != 101 {
		t.Fatalf("InputTokens = %d, want 101", usage.InputTokens)
	}
	if usage.OutputTokens != 9500 {
		t.Fatalf("OutputTokens = %d, want 9500", usage.OutputTokens)
	}
}

func TestParseProviderUsageCSVAggregatesRows(t *testing.T) {
	input := strings.NewReader(`provider,account_id,date,model,request_count,input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,image_output_tokens,cost_usd
openai,42,2026-06-27,gpt-5.4-mini,1,10,5,0,2,0,0.010
openai,42,2026-06-27,gpt-5.4-mini,2,30,7,1,3,0,0.020
`)

	rows, err := parseProviderUsageCSV(input)
	if err != nil {
		t.Fatalf("parseProviderUsageCSV returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1: %#v", len(rows), rows)
	}
	row := rows[0]
	if row.Key.Provider != "openai" || row.Key.AccountID != "42" || row.Key.Date != "2026-06-27" || row.Key.Model != "gpt-5.4-mini" {
		t.Fatalf("key = %#v", row.Key)
	}
	if row.Requests != 3 || row.InputTokens != 40 || row.OutputTokens != 12 || row.CacheCreationTokens != 1 || row.CacheReadTokens != 5 {
		t.Fatalf("token/request aggregate = %#v", row)
	}
	if diff := row.CostUSD - 0.03; diff < -0.0000001 || diff > 0.0000001 {
		t.Fatalf("cost = %.10f, want 0.03", row.CostUSD)
	}
}

func TestCompareAggregatesReportsDifferencesBeyondTolerance(t *testing.T) {
	key := AggregateKey{Provider: "openai", AccountID: "42", Date: "2026-06-27", Model: "gpt-5.4-mini"}
	internal := []UsageAggregate{{
		Key:          key,
		Requests:     10,
		InputTokens:  100,
		OutputTokens: 50,
		CostUSD:      0.10,
	}}
	provider := []UsageAggregate{{
		Key:          key,
		Requests:     11,
		InputTokens:  105,
		OutputTokens: 50,
		CostUSD:      0.14,
	}}

	report := compareAggregates(internal, provider, ReconcileTolerance{
		RequestTolerance: 0,
		TokenTolerance:   10,
		CostTolerance:    0.01,
	})
	if report.OK {
		t.Fatalf("report.OK = true, want false")
	}
	if len(report.Diffs) != 1 {
		t.Fatalf("len(diffs) = %d, want 1", len(report.Diffs))
	}
	diff := report.Diffs[0]
	if !diff.Failed {
		t.Fatalf("diff.Failed = false, want true")
	}
	if diff.RequestsDelta != -1 {
		t.Fatalf("RequestsDelta = %d, want -1", diff.RequestsDelta)
	}
	if diff.CostDelta > -0.0399 || diff.CostDelta < -0.0401 {
		t.Fatalf("CostDelta = %.10f, want about -0.04", diff.CostDelta)
	}
	if report.Summary.InternalRequests != 10 || report.Summary.ProviderRequests != 11 {
		t.Fatalf("summary = %#v", report.Summary)
	}
}

func TestBuildInternalUsageQueryFiltersByRunIDAndWindow(t *testing.T) {
	start := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	query, args := buildInternalUsageQuery(InternalQueryOptions{
		RunID:     "reconcile-run",
		Start:     start,
		End:       end,
		Provider:  "openai",
		GroupBy:   []string{"day", "account", "model"},
		CostBasis: CostBasisAccount,
	})

	for _, want := range []string{
		"FROM usage_logs ul",
		"ul.request_id LIKE $1",
		"ul.created_at >= $2",
		"ul.created_at < $3",
		"COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS cost_usd",
		"GROUP BY provider, account_id, date, model",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("query missing %q:\n%s", want, query)
		}
	}
	if len(args) != 3 {
		t.Fatalf("len(args) = %d, want 3", len(args))
	}
	if args[0] != "reconcile-run-%" {
		t.Fatalf("args[0] = %#v", args[0])
	}
}
