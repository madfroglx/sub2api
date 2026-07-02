package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"gopkg.in/yaml.v3"
)

type Endpoint string

const (
	EndpointMessages        Endpoint = "messages"
	EndpointChatCompletions Endpoint = "chat_completions"
	EndpointResponses       Endpoint = "responses"
	EndpointGemini          Endpoint = "gemini"
)

type CostBasis string

const (
	CostBasisAccount CostBasis = "account"
	CostBasisUser    CostBasis = "user"
)

type PromptProfile string

const (
	PromptProfileSimple PromptProfile = "simple"
	PromptProfileMixed  PromptProfile = "mixed"
	PromptProfileJSON   PromptProfile = "json"
	PromptProfileCode   PromptProfile = "code"
	PromptProfileLong   PromptProfile = "long"
)

type RunConfig struct {
	BaseURL            string
	APIKey             string
	Endpoint           Endpoint
	Model              string
	Prompt             string
	PromptProfile      PromptProfile
	RunID              string
	MaxTokens          int
	OutputLines        int
	OutputWordsPerLine int
	Stream             bool
	RequestTimeout     time.Duration
}

type ReconcileTolerance struct {
	RequestTolerance  int64   `json:"request_tolerance"`
	TokenTolerance    int64   `json:"token_tolerance"`
	CostTolerance     float64 `json:"cost_tolerance"`
	CostRateTolerance float64 `json:"cost_rate_tolerance"`
}

type AggregateKey struct {
	Provider    string `json:"provider"`
	AccountID   string `json:"account_id"`
	Date        string `json:"date"`
	Model       string `json:"model"`
	BillingMode string `json:"billing_mode,omitempty"`
}

type UsageAggregate struct {
	Key                 AggregateKey `json:"key"`
	Requests            int64        `json:"requests"`
	InputTokens         int64        `json:"input_tokens"`
	OutputTokens        int64        `json:"output_tokens"`
	CacheCreationTokens int64        `json:"cache_creation_tokens"`
	CacheReadTokens     int64        `json:"cache_read_tokens"`
	ImageOutputTokens   int64        `json:"image_output_tokens"`
	CostUSD             float64      `json:"cost_usd"`
}

type DiffRow struct {
	Key                      AggregateKey   `json:"key"`
	Internal                 UsageAggregate `json:"internal"`
	Provider                 UsageAggregate `json:"provider"`
	RequestsDelta            int64          `json:"requests_delta"`
	InputTokensDelta         int64          `json:"input_tokens_delta"`
	OutputTokensDelta        int64          `json:"output_tokens_delta"`
	CacheCreationTokensDelta int64          `json:"cache_creation_tokens_delta"`
	CacheReadTokensDelta     int64          `json:"cache_read_tokens_delta"`
	ImageOutputTokensDelta   int64          `json:"image_output_tokens_delta"`
	CostDelta                float64        `json:"cost_delta"`
	CostDeltaRate            float64        `json:"cost_delta_rate"`
	Failed                   bool           `json:"failed"`
}

type ReportSummary struct {
	OK               bool    `json:"ok"`
	InternalRequests int64   `json:"internal_requests"`
	ProviderRequests int64   `json:"provider_requests"`
	InternalCostUSD  float64 `json:"internal_cost_usd"`
	ProviderCostUSD  float64 `json:"provider_cost_usd"`
	CostDelta        float64 `json:"cost_delta"`
	DiffRows         int     `json:"diff_rows"`
	FailedRows       int     `json:"failed_rows"`
}

type ReconcileReport struct {
	RunID     string             `json:"run_id,omitempty"`
	StartedAt string             `json:"started_at,omitempty"`
	EndedAt   string             `json:"ended_at,omitempty"`
	Calls     []CallResult       `json:"calls,omitempty"`
	Tolerance ReconcileTolerance `json:"tolerance"`
	Summary   ReportSummary      `json:"summary"`
	OK        bool               `json:"ok"`
	Diffs     []DiffRow          `json:"diffs"`
}

type CallResult struct {
	Seq             int            `json:"seq"`
	ClientRequestID string         `json:"client_request_id"`
	StatusCode      int            `json:"status_code"`
	Error           string         `json:"error,omitempty"`
	Usage           UsageAggregate `json:"usage,omitempty"`
}

type InternalQueryOptions struct {
	RunID     string
	Start     time.Time
	End       time.Time
	Provider  string
	GroupBy   []string
	CostBasis CostBasis
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer) error {
	var (
		cfgPath        string
		dbDSN          string
		providerFile   string
		outPath        string
		startRaw       string
		endRaw         string
		groupByRaw     string
		costBasisRaw   string
		settleWaitRaw  time.Duration
		requestTimeout time.Duration
		tolerance      ReconcileTolerance
		callCount      int
	)
	runCfg := RunConfig{
		Prompt:             "Analyze the supplied workload and follow the output contract.",
		PromptProfile:      PromptProfileMixed,
		MaxTokens:          96,
		OutputLines:        6,
		OutputWordsPerLine: 8,
		Endpoint:           EndpointChatCompletions,
	}

	fs := flag.NewFlagSet("reconcile-provider-usage", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&runCfg.BaseURL, "base-url", "", "gateway base URL, for example http://127.0.0.1:8080")
	fs.StringVar(&runCfg.APIKey, "api-key", "", "customer API key")
	fs.Var((*endpointValue)(&runCfg.Endpoint), "endpoint", "messages|chat_completions|responses|gemini")
	fs.StringVar(&runCfg.Model, "model", "", "model to call through the customer gateway")
	fs.StringVar(&runCfg.Prompt, "prompt", runCfg.Prompt, "task text embedded in the generated test prompt")
	fs.Var((*promptProfileValue)(&runCfg.PromptProfile), "prompt-profile", "simple|mixed|json|code|long")
	fs.StringVar(&runCfg.RunID, "run-id", "", "stable run id; default reconcile-<timestamp>")
	fs.IntVar(&runCfg.MaxTokens, "max-tokens", runCfg.MaxTokens, "max output tokens")
	fs.IntVar(&runCfg.OutputLines, "output-lines", runCfg.OutputLines, "requested output line count for complex prompt profiles")
	fs.IntVar(&runCfg.OutputWordsPerLine, "output-words-per-line", runCfg.OutputWordsPerLine, "requested words per output line for complex prompt profiles")
	fs.BoolVar(&runCfg.Stream, "stream", false, "request streaming output and parse final SSE usage when available")
	fs.IntVar(&callCount, "count", 1, "number of customer API calls to make; set 0 to only reconcile existing rows")
	fs.DurationVar(&requestTimeout, "request-timeout", 60*time.Second, "per-request timeout")
	fs.DurationVar(&settleWaitRaw, "settle-wait", 2*time.Second, "wait after calls before DB reconciliation")
	fs.StringVar(&cfgPath, "config", "", "config.yaml path or DATA_DIR containing config.yaml")
	fs.StringVar(&dbDSN, "db-dsn", "", "PostgreSQL DSN; overrides --config")
	fs.StringVar(&providerFile, "provider-file", "", "provider usage CSV or JSON export")
	fs.StringVar(&startRaw, "start", "", "inclusive start time, RFC3339 or YYYY-MM-DD")
	fs.StringVar(&endRaw, "end", "", "exclusive end time, RFC3339 or YYYY-MM-DD")
	fs.StringVar(&groupByRaw, "group-by", "day,account,model", "dimensions: day,account,model,billing_mode")
	fs.StringVar(&costBasisRaw, "cost-basis", string(CostBasisAccount), "account or user")
	fs.StringVar(&outPath, "out", "", "write JSON report to file instead of stdout")
	fs.Int64Var(&tolerance.RequestTolerance, "request-tolerance", 0, "allowed request-count delta")
	fs.Int64Var(&tolerance.TokenTolerance, "token-tolerance", 10, "allowed token delta per token field")
	fs.Float64Var(&tolerance.CostTolerance, "cost-tolerance", 0.01, "allowed absolute USD cost delta")
	fs.Float64Var(&tolerance.CostRateTolerance, "cost-rate-tolerance", 0, "allowed relative USD cost delta")
	if err := fs.Parse(args); err != nil {
		return err
	}
	runCfg.RequestTimeout = requestTimeout
	if runCfg.RunID == "" {
		runCfg.RunID = "reconcile-" + time.Now().Format("20060102150405")
	}
	costBasis := CostBasis(strings.ToLower(strings.TrimSpace(costBasisRaw)))
	if costBasis != CostBasisAccount && costBasis != CostBasisUser {
		return fmt.Errorf("invalid --cost-basis %q", costBasisRaw)
	}

	start := time.Now().Add(-time.Minute)
	var err error
	if strings.TrimSpace(startRaw) != "" {
		start, err = parseTimeArg(startRaw)
		if err != nil {
			return fmt.Errorf("parse --start: %w", err)
		}
	}

	calls := []CallResult{}
	if callCount > 0 {
		if err := validateCallConfig(runCfg); err != nil {
			return err
		}
		calls = executeGatewayCalls(ctx, runCfg, callCount)
		if settleWaitRaw > 0 {
			time.Sleep(settleWaitRaw)
		}
	}

	end := time.Now().Add(time.Second)
	if strings.TrimSpace(endRaw) != "" {
		end, err = parseTimeArg(endRaw)
		if err != nil {
			return fmt.Errorf("parse --end: %w", err)
		}
	}
	if !end.After(start) {
		return errors.New("--end must be after --start")
	}

	var internal []UsageAggregate
	if dbDSN != "" || cfgPath != "" {
		dsn, err := resolveDSN(dbDSN, cfgPath)
		if err != nil {
			return err
		}
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("connect database: %w", err)
		}
		internal, err = queryInternalUsage(ctx, db, InternalQueryOptions{
			RunID:     runCfg.RunID,
			Start:     start,
			End:       end,
			Provider:  "",
			GroupBy:   splitCSVList(groupByRaw),
			CostBasis: costBasis,
		})
		if err != nil {
			return err
		}
	}

	provider := []UsageAggregate{}
	if strings.TrimSpace(providerFile) != "" {
		provider, err = loadProviderUsage(providerFile)
		if err != nil {
			return err
		}
	}
	report := compareAggregates(internal, provider, tolerance)
	report.RunID = runCfg.RunID
	report.StartedAt = start.Format(time.RFC3339)
	report.EndedAt = end.Format(time.RFC3339)
	report.Calls = calls
	report.Tolerance = tolerance

	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if outPath != "" {
		return os.WriteFile(outPath, append(payload, '\n'), 0o644)
	}
	_, err = stdout.Write(append(payload, '\n'))
	return err
}

func validateCallConfig(cfg RunConfig) error {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return errors.New("--base-url is required when --count > 0")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return errors.New("--api-key is required when --count > 0")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return errors.New("--model is required when --count > 0")
	}
	return nil
}

func executeGatewayCalls(ctx context.Context, cfg RunConfig, count int) []CallResult {
	client := &http.Client{Timeout: cfg.RequestTimeout}
	results := make([]CallResult, 0, count)
	for i := 1; i <= count; i++ {
		clientRequestID := fmt.Sprintf("%s-%04d", cfg.RunID, i)
		res := CallResult{Seq: i, ClientRequestID: clientRequestID}
		req, err := buildGatewayRequest(ctx, cfg, i)
		if err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			res.Error = err.Error()
			results = append(results, res)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		closeErr := resp.Body.Close()
		res.StatusCode = resp.StatusCode
		if readErr != nil {
			res.Error = readErr.Error()
		} else if closeErr != nil {
			res.Error = closeErr.Error()
		} else if resp.StatusCode >= 400 {
			res.Error = strings.TrimSpace(string(body))
		} else {
			res.Usage = usageFromGatewayResponse(body)
		}
		results = append(results, res)
	}
	return results
}

func buildGatewayRequest(ctx context.Context, cfg RunConfig, seq int) (*http.Request, error) {
	base, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil {
		return nil, err
	}
	clientRequestID := fmt.Sprintf("%s-%04d", cfg.RunID, seq)
	prompt := buildPrompt(cfg, clientRequestID)

	path, body := gatewayPathAndBody(cfg, prompt)
	base.Path = strings.TrimRight(base.Path, "/") + path
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("x-goog-api-key", cfg.APIKey)
	req.Header.Set("X-Client-Request-ID", clientRequestID)
	return req, nil
}

func buildPrompt(cfg RunConfig, clientRequestID string) string {
	if cfg.OutputLines <= 0 {
		cfg.OutputLines = 6
	}
	if cfg.OutputWordsPerLine <= 0 {
		cfg.OutputWordsPerLine = 8
	}
	task := strings.TrimSpace(cfg.Prompt)
	if task == "" {
		task = "Analyze the supplied workload and follow the output contract."
	}
	markers := fmt.Sprintf("reconcile_run_id=%s\nreconcile_request_id=%s", cfg.RunID, clientRequestID)
	if cfg.PromptProfile == "" || cfg.PromptProfile == PromptProfileSimple {
		return fmt.Sprintf("%s\n\n%s", task, markers)
	}

	var body string
	switch cfg.PromptProfile {
	case PromptProfileJSON:
		body = jsonPromptWorkload(task)
	case PromptProfileCode:
		body = codePromptWorkload(task)
	case PromptProfileLong:
		body = strings.Join([]string{
			mixedPromptWorkload(task),
			mixedPromptWorkload("Repeat the same analysis with attention to token accounting stability."),
		}, "\n\n--- repeated workload boundary ---\n\n")
	default:
		body = mixedPromptWorkload(task)
	}

	return strings.Join([]string{
		body,
		outputContract(cfg.OutputLines, cfg.OutputWordsPerLine),
		markers,
	}, "\n\n")
}

func mixedPromptWorkload(task string) string {
	return strings.Join([]string{
		"Structured token-count reconciliation workload.",
		"Task: " + task,
		"中文片段：请比较“调用量”、“输入 token”、“输出 token”、“缓存 token”和“供应商费用”，并保持术语原样输出。",
		"Symbols and numerics: alpha=0.125, beta=-42, gamma=[1,2,3], ratio=17/31, currency=$0.000123.",
		"```json",
		`{"provider":"openai","account_id":42,"usage":{"input_tokens":1287,"output_tokens":233,"cache_read_tokens":144},"tags":["billing","reconcile","token-test"]}`,
		"```",
		"```go",
		"func reconcileCost(inputTokens, outputTokens int, inputPrice, outputPrice float64) float64 {",
		"    return float64(inputTokens)*inputPrice + float64(outputTokens)*outputPrice",
		"}",
		"```",
		"| metric | internal | provider | delta |",
		"|---|---:|---:|---:|",
		"| requests | 7 | 7 | 0 |",
		"| cost_usd | 0.1932 | 0.1931 | 0.0001 |",
	}, "\n")
}

func jsonPromptWorkload(task string) string {
	return strings.Join([]string{
		"Structured JSON token-count reconciliation workload.",
		"Task: " + task,
		"Read this nested JSON and preserve every field name exactly in your reasoning:",
		"```json",
		`{"run":{"kind":"provider_usage_reconcile","timezone":"Asia/Shanghai"},"records":[{"date":"2026-06-27","model":"gpt-5.4-mini","input_tokens":1201,"output_tokens":305},{"date":"2026-06-27","model":"gpt-5.4-mini","input_tokens":1199,"output_tokens":307}],"notes":"中文、ASCII、数字和嵌套结构同时出现。"}`,
		"```",
	}, "\n")
}

func codePromptWorkload(task string) string {
	return strings.Join([]string{
		"Structured code token-count reconciliation workload.",
		"Task: " + task,
		"Inspect the following snippets without executing them.",
		"```sql",
		"SELECT account_id, model, SUM(input_tokens) AS input_tokens, SUM(actual_cost) AS actual_cost FROM usage_logs GROUP BY account_id, model;",
		"```",
		"```go",
		"type UsageRow struct { AccountID int64; Model string; InputTokens int64; ActualCost float64 }",
		"```",
		"中文片段：重点检查聚合维度是否与供应商账单维度一致。",
	}, "\n")
}

func outputContract(lines, wordsPerLine int) string {
	return fmt.Sprintf(
		"Output contract:\nReturn exactly %d lines.\nEach line must contain exactly %d words from this lowercase vocabulary only: alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu.\nDo not use numbering, punctuation, markdown, JSON, code fences, or explanation.",
		lines,
		wordsPerLine,
	)
}

func gatewayPathAndBody(cfg RunConfig, prompt string) (string, map[string]any) {
	withStream := func(body map[string]any, includeUsage bool) map[string]any {
		if !cfg.Stream {
			return body
		}
		body["stream"] = true
		if includeUsage {
			body["stream_options"] = map[string]any{"include_usage": true}
		}
		return body
	}
	switch cfg.Endpoint {
	case EndpointMessages:
		return "/v1/messages", withStream(map[string]any{
			"model":      cfg.Model,
			"max_tokens": cfg.MaxTokens,
			"messages": []map[string]string{{
				"role":    "user",
				"content": prompt,
			}},
		}, false)
	case EndpointResponses:
		return "/v1/responses", withStream(map[string]any{
			"model":             cfg.Model,
			"max_output_tokens": cfg.MaxTokens,
			"input":             prompt,
		}, false)
	case EndpointGemini:
		return "/v1beta/models/" + url.PathEscape(cfg.Model) + ":generateContent", withStream(map[string]any{
			"contents": []map[string]any{{
				"role": "user",
				"parts": []map[string]string{{
					"text": prompt,
				}},
			}},
			"generationConfig": map[string]any{
				"maxOutputTokens": cfg.MaxTokens,
			},
		}, false)
	default:
		return "/v1/chat/completions", withStream(map[string]any{
			"model":      cfg.Model,
			"max_tokens": cfg.MaxTokens,
			"messages": []map[string]string{{
				"role":    "user",
				"content": prompt,
			}},
		}, true)
	}
}

func usageFromGatewayResponse(body []byte) UsageAggregate {
	if usage := usageFromJSONBody(body); usage.Requests > 0 || usage.InputTokens > 0 || usage.OutputTokens > 0 {
		return usage
	}
	return usageFromSSEBody(body)
}

func usageFromJSONBody(body []byte) UsageAggregate {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return UsageAggregate{}
	}
	usage := objectAt(raw, "usage")
	if len(usage) == 0 {
		usage = objectAt(raw, "response", "usage")
	}
	if len(usage) == 0 {
		return UsageAggregate{}
	}
	return UsageAggregate{
		Requests:            1,
		InputTokens:         int64FromAny(firstValue(usage, "input_tokens", "prompt_tokens", "totalTokenCount", "promptTokenCount")),
		OutputTokens:        int64FromAny(firstValue(usage, "output_tokens", "completion_tokens", "candidatesTokenCount")),
		CacheCreationTokens: int64FromAny(firstValue(usage, "cache_creation_input_tokens")),
		CacheReadTokens:     int64FromAny(firstValue(usage, "cache_read_input_tokens", "cached_tokens")),
	}
}

func usageFromSSEBody(body []byte) UsageAggregate {
	var out UsageAggregate
	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		usage := usageFromJSONBody([]byte(data))
		if usage.Requests > 0 || usage.InputTokens > 0 || usage.OutputTokens > 0 ||
			usage.CacheCreationTokens > 0 || usage.CacheReadTokens > 0 || usage.ImageOutputTokens > 0 {
			out = usage
		}
	}
	return out
}

func objectAt(root map[string]any, path ...string) map[string]any {
	var cur any = root
	for _, part := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[part]
	}
	obj, _ := cur.(map[string]any)
	return obj
}

func firstValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func loadProviderUsage(path string) ([]UsageAggregate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return parseProviderUsageJSON(f)
	default:
		return parseProviderUsageCSV(f)
	}
}

func parseProviderUsageCSV(r io.Reader) ([]UsageAggregate, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(header))
	for i, h := range header {
		index[normalizeFieldName(h)] = i
	}
	rowsByKey := map[AggregateKey]UsageAggregate{}
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		row := providerRecordToAggregate(func(names ...string) string {
			for _, name := range names {
				if pos, ok := index[normalizeFieldName(name)]; ok && pos < len(record) {
					return strings.TrimSpace(record[pos])
				}
			}
			return ""
		})
		addAggregate(rowsByKey, row)
	}
	return sortedAggregates(rowsByKey), nil
}

func parseProviderUsageJSON(r io.Reader) ([]UsageAggregate, error) {
	var records []map[string]any
	if err := json.NewDecoder(r).Decode(&records); err != nil {
		return nil, err
	}
	rowsByKey := map[AggregateKey]UsageAggregate{}
	for _, rec := range records {
		row := providerRecordToAggregate(func(names ...string) string {
			for _, name := range names {
				if v, ok := rec[name]; ok {
					return strings.TrimSpace(fmt.Sprint(v))
				}
				norm := normalizeFieldName(name)
				for k, v := range rec {
					if normalizeFieldName(k) == norm {
						return strings.TrimSpace(fmt.Sprint(v))
					}
				}
			}
			return ""
		})
		addAggregate(rowsByKey, row)
	}
	return sortedAggregates(rowsByKey), nil
}

func providerRecordToAggregate(get func(...string) string) UsageAggregate {
	return UsageAggregate{
		Key: AggregateKey{
			Provider:    strings.ToLower(get("provider", "platform")),
			AccountID:   get("account_id", "account", "supplier_account_id", "provider_account_id"),
			Date:        normalizeDate(get("date", "day", "created_at", "start_time")),
			Model:       get("model", "upstream_model"),
			BillingMode: get("billing_mode", "mode"),
		},
		Requests:            parseInt64(get("request_count", "requests", "count")),
		InputTokens:         parseInt64(get("input_tokens", "prompt_tokens")),
		OutputTokens:        parseInt64(get("output_tokens", "completion_tokens")),
		CacheCreationTokens: parseInt64(get("cache_creation_tokens", "cache_creation_input_tokens", "cache_write_tokens")),
		CacheReadTokens:     parseInt64(get("cache_read_tokens", "cache_read_input_tokens")),
		ImageOutputTokens:   parseInt64(get("image_output_tokens")),
		CostUSD:             parseFloat64(get("cost_usd", "cost", "amount_usd", "total_cost")),
	}
}

func addAggregate(rows map[AggregateKey]UsageAggregate, row UsageAggregate) {
	existing := rows[row.Key]
	existing.Key = row.Key
	existing.Requests += row.Requests
	existing.InputTokens += row.InputTokens
	existing.OutputTokens += row.OutputTokens
	existing.CacheCreationTokens += row.CacheCreationTokens
	existing.CacheReadTokens += row.CacheReadTokens
	existing.ImageOutputTokens += row.ImageOutputTokens
	existing.CostUSD += row.CostUSD
	rows[row.Key] = existing
}

func compareAggregates(internal, provider []UsageAggregate, tol ReconcileTolerance) ReconcileReport {
	internalMap := aggregateSliceToMap(internal)
	providerMap := aggregateSliceToMap(provider)
	keySet := map[AggregateKey]struct{}{}
	for key := range internalMap {
		keySet[key] = struct{}{}
	}
	for key := range providerMap {
		keySet[key] = struct{}{}
	}
	keys := make([]AggregateKey, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return aggregateKeyString(keys[i]) < aggregateKeyString(keys[j]) })

	report := ReconcileReport{Tolerance: tol, Diffs: make([]DiffRow, 0)}
	for _, key := range keys {
		in := internalMap[key]
		prov := providerMap[key]
		diff := DiffRow{
			Key:                      key,
			Internal:                 in,
			Provider:                 prov,
			RequestsDelta:            in.Requests - prov.Requests,
			InputTokensDelta:         in.InputTokens - prov.InputTokens,
			OutputTokensDelta:        in.OutputTokens - prov.OutputTokens,
			CacheCreationTokensDelta: in.CacheCreationTokens - prov.CacheCreationTokens,
			CacheReadTokensDelta:     in.CacheReadTokens - prov.CacheReadTokens,
			ImageOutputTokensDelta:   in.ImageOutputTokens - prov.ImageOutputTokens,
			CostDelta:                in.CostUSD - prov.CostUSD,
		}
		if prov.CostUSD != 0 {
			diff.CostDeltaRate = diff.CostDelta / prov.CostUSD
		}
		diff.Failed = diffExceedsTolerance(diff, tol)
		if diff.Failed || !aggregateEqual(in, prov) {
			report.Diffs = append(report.Diffs, diff)
		}
		report.Summary.InternalRequests += in.Requests
		report.Summary.ProviderRequests += prov.Requests
		report.Summary.InternalCostUSD += in.CostUSD
		report.Summary.ProviderCostUSD += prov.CostUSD
		if diff.Failed {
			report.Summary.FailedRows++
		}
	}
	report.Summary.CostDelta = report.Summary.InternalCostUSD - report.Summary.ProviderCostUSD
	report.Summary.DiffRows = len(report.Diffs)
	report.OK = report.Summary.FailedRows == 0
	report.Summary.OK = report.OK
	return report
}

func diffExceedsTolerance(diff DiffRow, tol ReconcileTolerance) bool {
	if absInt64(diff.RequestsDelta) > tol.RequestTolerance {
		return true
	}
	for _, value := range []int64{
		diff.InputTokensDelta,
		diff.OutputTokensDelta,
		diff.CacheCreationTokensDelta,
		diff.CacheReadTokensDelta,
		diff.ImageOutputTokensDelta,
	} {
		if absInt64(value) > tol.TokenTolerance {
			return true
		}
	}
	if math.Abs(diff.CostDelta) > tol.CostTolerance {
		return true
	}
	if tol.CostRateTolerance > 0 && math.Abs(diff.CostDeltaRate) > tol.CostRateTolerance {
		return true
	}
	return false
}

func buildInternalUsageQuery(opts InternalQueryOptions) (string, []any) {
	args := []any{strings.TrimRight(strings.TrimSpace(opts.RunID), "-") + "-%", opts.Start, opts.End}
	costExpr := "COALESCE(SUM(ul.actual_cost), 0) AS cost_usd"
	if opts.CostBasis == CostBasisAccount {
		costExpr = "COALESCE(SUM(COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1)), 0) AS cost_usd"
	}
	billingModeExpr := "'' AS billing_mode"
	groupByClause := "provider, account_id, date, model"
	orderByClause := "provider, account_id, date, model"
	if hasGroupBy(opts.GroupBy, "billing_mode") {
		billingModeExpr = "COALESCE(NULLIF(TRIM(ul.billing_mode), ''), '') AS billing_mode"
		groupByClause += ", billing_mode"
		orderByClause += ", billing_mode"
	}
	query := fmt.Sprintf(`
SELECT
	COALESCE(NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS provider,
	ul.account_id::text AS account_id,
	TO_CHAR(ul.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS date,
	COALESCE(NULLIF(TRIM(ul.upstream_model), ''), NULLIF(TRIM(ul.model), ''), '') AS model,
	%s,
	COUNT(*) AS requests,
	COALESCE(SUM(ul.input_tokens), 0) AS input_tokens,
	COALESCE(SUM(ul.output_tokens), 0) AS output_tokens,
	COALESCE(SUM(ul.cache_creation_tokens), 0) AS cache_creation_tokens,
	COALESCE(SUM(ul.cache_read_tokens), 0) AS cache_read_tokens,
	COALESCE(SUM(ul.image_output_tokens), 0) AS image_output_tokens,
	%s
FROM usage_logs ul
LEFT JOIN accounts a ON a.id = ul.account_id
LEFT JOIN groups g ON g.id = ul.group_id
WHERE ul.request_id LIKE $1
	AND ul.created_at >= $2
	AND ul.created_at < $3
	AND ul.actual_cost > 0
GROUP BY %s
ORDER BY %s`, billingModeExpr, costExpr, groupByClause, orderByClause)
	return query, args
}

func queryInternalUsage(ctx context.Context, db *sql.DB, opts InternalQueryOptions) ([]UsageAggregate, error) {
	query, args := buildInternalUsageQuery(opts)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UsageAggregate{}
	for rows.Next() {
		var row UsageAggregate
		if err := rows.Scan(
			&row.Key.Provider,
			&row.Key.AccountID,
			&row.Key.Date,
			&row.Key.Model,
			&row.Key.BillingMode,
			&row.Requests,
			&row.InputTokens,
			&row.OutputTokens,
			&row.CacheCreationTokens,
			&row.CacheReadTokens,
			&row.ImageOutputTokens,
			&row.CostUSD,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

type fileConfig struct {
	Timezone string `yaml:"timezone"`
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`
}

func resolveDSN(directDSN, cfgPath string) (string, error) {
	if strings.TrimSpace(directDSN) != "" {
		return directDSN, nil
	}
	if strings.TrimSpace(cfgPath) == "" {
		return "", errors.New("--db-dsn or --config is required for internal reconciliation")
	}
	path := cfgPath
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		path = filepath.Join(path, "config.yaml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var cfg fileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", err
	}
	db := cfg.Database
	if db.Port == 0 {
		db.Port = 5432
	}
	if db.SSLMode == "" {
		db.SSLMode = "prefer"
	}
	if cfg.Timezone == "" {
		cfg.Timezone = "Asia/Shanghai"
	}
	if db.Password == "" {
		return fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=%s TimeZone=%s", db.Host, db.Port, db.User, db.DBName, db.SSLMode, cfg.Timezone), nil
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s TimeZone=%s", db.Host, db.Port, db.User, db.Password, db.DBName, db.SSLMode, cfg.Timezone), nil
}

func parseTimeArg(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func normalizeDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if t, err := parseTimeArg(value); err == nil {
		return t.Format("2006-01-02")
	}
	if len(value) >= len("2006-01-02") {
		return value[:len("2006-01-02")]
	}
	return value
}

func splitCSVList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func hasGroupBy(groups []string, target string) bool {
	target = normalizeFieldName(target)
	for _, group := range groups {
		if normalizeFieldName(group) == target {
			return true
		}
	}
	return false
}

func sortedAggregates(rows map[AggregateKey]UsageAggregate) []UsageAggregate {
	out := make([]UsageAggregate, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		return aggregateKeyString(out[i].Key) < aggregateKeyString(out[j].Key)
	})
	return out
}

func aggregateSliceToMap(rows []UsageAggregate) map[AggregateKey]UsageAggregate {
	out := map[AggregateKey]UsageAggregate{}
	for _, row := range rows {
		addAggregate(out, row)
	}
	return out
}

func aggregateEqual(a, b UsageAggregate) bool {
	return a.Requests == b.Requests &&
		a.InputTokens == b.InputTokens &&
		a.OutputTokens == b.OutputTokens &&
		a.CacheCreationTokens == b.CacheCreationTokens &&
		a.CacheReadTokens == b.CacheReadTokens &&
		a.ImageOutputTokens == b.ImageOutputTokens &&
		math.Abs(a.CostUSD-b.CostUSD) < 0.0000000001
}

func aggregateKeyString(key AggregateKey) string {
	return key.Provider + "\x00" + key.AccountID + "\x00" + key.Date + "\x00" + key.Model + "\x00" + key.BillingMode
}

func normalizeFieldName(value string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", "_"))
}

func parseInt64(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i
	}
	f, _ := strconv.ParseFloat(value, 64)
	return int64(f)
}

func parseFloat64(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(value, 64)
	return f
}

func int64FromAny(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	case json.Number:
		i, _ := t.Int64()
		return i
	case string:
		return parseInt64(t)
	default:
		return 0
	}
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

type endpointValue Endpoint

func (v *endpointValue) Set(raw string) error {
	endpoint := Endpoint(strings.ToLower(strings.TrimSpace(raw)))
	switch endpoint {
	case EndpointMessages, EndpointChatCompletions, EndpointResponses, EndpointGemini:
		*v = endpointValue(endpoint)
		return nil
	default:
		return fmt.Errorf("unsupported endpoint %q", raw)
	}
}

func (v *endpointValue) String() string {
	return string(*v)
}

type promptProfileValue PromptProfile

func (v *promptProfileValue) Set(raw string) error {
	profile := PromptProfile(strings.ToLower(strings.TrimSpace(raw)))
	switch profile {
	case PromptProfileSimple, PromptProfileMixed, PromptProfileJSON, PromptProfileCode, PromptProfileLong:
		*v = promptProfileValue(profile)
		return nil
	default:
		return fmt.Errorf("unsupported prompt profile %q", raw)
	}
}

func (v *promptProfileValue) String() string {
	return string(*v)
}
