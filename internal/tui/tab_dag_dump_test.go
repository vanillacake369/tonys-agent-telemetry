package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vanillacake369/tonys-agent-telemetry/internal/telemetry"
)

func TestDAGDump_NarrowWidth(t *testing.T) {
	spans := []telemetry.Span{
		{TraceID: "t", SpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "c1", ParentSpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "c2", ParentSpanID: "root", System: "anthropic"},
		{TraceID: "t", SpanID: "c3", ParentSpanID: "root", System: "anthropic"},
	}
	d := dagFromSpans(t, spans, 40, 30)
	out := d.renderGraph(36)

	lines := strings.Split(out, "\n")
	t.Logf("renderGraph(36) returned %d lines", len(lines))
	for i, line := range lines {
		plain := stripAnsi(line)
		t.Logf("[%2d] len=%2d  %q", i, len([]rune(plain)), plain)
		if i > 20 {
			break
		}
	}
	_ = fmt.Sprintf // keep import
}
