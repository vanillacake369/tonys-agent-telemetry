// Package otlp implements an OpenTelemetry HTTP/JSON receiver as a
// telemetry.ProviderIngestor. Any tool that exports OTel traces over OTLP
// (LangGraph + OpenLLMetry, CrewAI, AutoGen, OpenAI Agents SDK, LiteLLM,
// TGI, etc.) can point its exporter at this endpoint and the spans flow
// into the same telemetry.Span channel as Claude/vLLM data.
//
// JSON over HTTP is chosen over gRPC+protobuf to avoid a heavyweight
// dependency. Per the OTLP spec, both encodings are first-class.
package otlp

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/provider"
	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

// DefaultAddr is the OTLP/HTTP standard port bound to localhost only.
// Binding to 0.0.0.0 by default would allow any process on the same Docker
// bridge or LAN to inject spans without authentication — see PIVOT_PLAN.md
// security gate (QA finding #3, P0).
const DefaultAddr = "127.0.0.1:4318"

// MaxRequestBytes caps individual export payloads to defend against runaway
// clients.
const MaxRequestBytes = 16 << 20

// resolveBindAddr returns the address to listen on. If the environment
// variable TONYS_OTLP_BIND is set and non-empty, it takes precedence over
// DefaultAddr. This opt-in is intentional: operators who need LAN-accessible
// ingest (e.g. sidecar containers) must explicitly set TONYS_OTLP_BIND=0.0.0.0:4318.
// See PIVOT_PLAN.md security gate (QA finding #3, P0).
func resolveBindAddr() string {
	if v := os.Getenv("TONYS_OTLP_BIND"); v != "" {
		return v
	}
	return DefaultAddr
}

// Ingestor runs an HTTP server accepting OTLP/JSON exports at /v1/traces.
type Ingestor struct {
	Addr string // default "127.0.0.1:4318"
}

// New returns an Ingestor bound to the resolved address (TONYS_OTLP_BIND env
// var if set, otherwise DefaultAddr "127.0.0.1:4318").
func New() *Ingestor { return &Ingestor{Addr: resolveBindAddr()} }

// ProviderID returns "otlp-http".
func (i *Ingestor) ProviderID() string { return "otlp-http" }

// Detect checks whether the configured port is available to bind. False
// means another process already owns :4318 (another OTLP receiver, the
// Grafana Alloy agent, etc.) — we yield rather than fight for it.
func (i *Ingestor) Detect(ctx context.Context) bool {
	ln, err := net.Listen("tcp", i.Addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// Ingest binds the configured address and serves /v1/traces until ctx
// cancellation. Each incoming OTLP export is parsed and translated into
// telemetry.Span values streamed to out.
func (i *Ingestor) Ingest(ctx context.Context, out chan<- telemetry.Span) error {
	defer provider.RecoverIngest(i.ProviderID(), log.Printf)()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBytes))
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		spans, err := ParseTracesJSON(body)
		if err != nil {
			http.Error(w, "parse error", http.StatusBadRequest)
			return
		}
		for _, sp := range spans {
			select {
			case out <- sp:
			case <-ctx.Done():
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              i.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("otlp: listening on %s (set TONYS_OTLP_BIND to override)", i.Addr)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		return ctx.Err()
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// ParseTracesJSON parses an OTLP/JSON ExportTraceServiceRequest payload and
// returns the contained spans as telemetry.Span values.
//
// Spec: https://opentelemetry.io/docs/specs/otlp/#otlphttp-request
func ParseTracesJSON(raw []byte) ([]telemetry.Span, error) {
	var req exportRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	var out []telemetry.Span
	for _, rs := range req.ResourceSpans {
		resAttrs := flattenAttrs(rs.Resource.Attributes)
		for _, ss := range rs.ScopeSpans {
			for _, s := range ss.Spans {
				sp := convertSpan(s, resAttrs)
				out = append(out, sp)
			}
		}
	}
	return out, nil
}

// convertSpan maps an OTLP span (with resource-scope attribute fallbacks)
// to a telemetry.Span. GenAI semconv attributes promote into stable struct
// fields; everything else flows into Attrs.
func convertSpan(s otlpSpan, resAttrs map[string]string) telemetry.Span {
	merged := map[string]string{}
	for k, v := range resAttrs {
		merged[k] = v
	}
	for _, kv := range s.Attributes {
		if kv.Key == "" {
			continue
		}
		merged[kv.Key] = anyValueToString(kv.Value)
	}

	span := telemetry.Span{
		TraceID:      s.TraceID,
		SpanID:       s.SpanID,
		ParentSpanID: s.ParentSpanID,
		Status:       "done",
		Attrs:        map[string]string{},
	}

	// Promote GenAI semconv attributes into struct fields.
	if v, ok := merged["gen_ai.system"]; ok {
		span.System = v
		delete(merged, "gen_ai.system")
	}
	if v, ok := merged["gen_ai.request.model"]; ok {
		span.Model = v
		delete(merged, "gen_ai.request.model")
	}
	if v, ok := merged["gen_ai.usage.input_tokens"]; ok {
		span.InputTokens = atoi(v)
		delete(merged, "gen_ai.usage.input_tokens")
	}
	if v, ok := merged["gen_ai.usage.output_tokens"]; ok {
		span.OutputTokens = atoi(v)
		delete(merged, "gen_ai.usage.output_tokens")
	}

	// Span.Name often carries the operation when no explicit attribute does.
	if span.Attrs["gen_ai.operation.name"] == "" && s.Name != "" {
		merged["gen_ai.operation.name"] = s.Name
	}

	if t := parseUnixNano(s.StartTimeUnixNano); !t.IsZero() {
		span.StartTime = t
	}
	if t := parseUnixNano(s.EndTimeUnixNano); !t.IsZero() {
		span.EndTime = t
	}

	// Status code: 2 = ERROR per OTLP spec.
	if s.Status != nil && s.Status.Code == 2 {
		span.Status = "error"
	}

	span.Attrs = merged
	return span
}

// flattenAttrs converts a list of OTLP key-value pairs into a flat map of
// string values. Used for resource-level attributes that all spans inherit.
func flattenAttrs(kvs []otlpKV) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv.Key == "" {
			continue
		}
		m[kv.Key] = anyValueToString(kv.Value)
	}
	return m
}

// anyValueToString flattens an OTLP AnyValue oneof to a string. Numeric and
// bool variants are stringified via strconv.
func anyValueToString(v otlpAnyValue) string {
	switch {
	case v.StringValue != nil:
		return *v.StringValue
	case v.IntValue != nil:
		return *v.IntValue // already a string in OTLP/JSON
	case v.DoubleValue != nil:
		return strconv.FormatFloat(*v.DoubleValue, 'g', -1, 64)
	case v.BoolValue != nil:
		return strconv.FormatBool(*v.BoolValue)
	}
	return ""
}

// parseUnixNano accepts the OTLP/JSON convention where timestamps are
// strings in nanoseconds since unix epoch. Returns zero Time on parse error.
func parseUnixNano(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(0, n)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// HexTraceID is exposed for testing — converts a 16-byte trace id to its
// canonical 32-character lowercase hex string. The OTLP/JSON wire format
// already uses hex strings so this helper is mainly for synthetic fixtures.
func HexTraceID(b []byte) string { return hex.EncodeToString(b) }

// --- OTLP/JSON wire types (subset we care about) ---

type exportRequest struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type resourceSpans struct {
	Resource   resource     `json:"resource"`
	ScopeSpans []scopeSpans `json:"scopeSpans"`
}

type resource struct {
	Attributes []otlpKV `json:"attributes"`
}

type scopeSpans struct {
	Spans []otlpSpan `json:"spans"`
}

type otlpSpan struct {
	TraceID           string      `json:"traceId"`
	SpanID            string      `json:"spanId"`
	ParentSpanID      string      `json:"parentSpanId"`
	Name              string      `json:"name"`
	StartTimeUnixNano string      `json:"startTimeUnixNano"`
	EndTimeUnixNano   string      `json:"endTimeUnixNano"`
	Attributes        []otlpKV    `json:"attributes"`
	Status            *otlpStatus `json:"status,omitempty"`
}

type otlpKV struct {
	Key   string       `json:"key"`
	Value otlpAnyValue `json:"value"`
}

type otlpAnyValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	IntValue    *string  `json:"intValue,omitempty"` // string in JSON per spec
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
}

type otlpStatus struct {
	Code int `json:"code"`
}
