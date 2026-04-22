package observe

import (
	"context"
	"fmt"
	"strings"
	"time"

	dlog "github.com/shemic/dever/log"
)

type builtinProvider struct {
	slowRequest time.Duration
	slowSQL     time.Duration
}

func newBuiltinProvider(cfg Config) Provider {
	return &builtinProvider{
		slowRequest: cfg.SlowRequest,
		slowSQL:     cfg.SlowSQL,
	}
}

func (p *builtinProvider) OnStart(context.Context, Snapshot) {}

func (p *builtinProvider) OnFinish(_ context.Context, snapshot Snapshot) {
	switch snapshot.Kind {
	case KindRequest:
		p.logRequest(snapshot)
	case KindDB:
		p.logDB(snapshot)
	}
}

func (p *builtinProvider) logRequest(snapshot Snapshot) {
	if snapshot.Error != nil {
		fields := dlog.Fields{
			"kind":     snapshot.Kind,
			"trace_id": snapshot.TraceID,
			"span_id":  snapshot.SpanID,
			"name":     snapshot.Name,
			"duration": snapshot.Duration.String(),
			"error":    dlog.ErrorValue(snapshot.Error),
		}
		applyObserveRequestFields(fields, snapshot.Attributes)
		dlog.ErrorFields("observe_request", "request observe error", fields)
		return
	}
	if p.slowRequest > 0 && snapshot.Duration >= p.slowRequest {
		fields := dlog.Fields{
			"kind":     snapshot.Kind,
			"trace_id": snapshot.TraceID,
			"span_id":  snapshot.SpanID,
			"name":     snapshot.Name,
			"duration": snapshot.Duration.String(),
			"slow":     true,
		}
		applyObserveRequestFields(fields, snapshot.Attributes)
		dlog.AccessFields("observe_request", "slow request observed", fields)
	}
}

func (p *builtinProvider) logDB(snapshot Snapshot) {
	if snapshot.Error == nil && (p.slowSQL <= 0 || snapshot.Duration < p.slowSQL) {
		return
	}

	statement := truncateStatement(snapshot.Attributes["db.statement"])
	operation := strings.TrimSpace(fmt.Sprint(snapshot.Attributes["db.operation"]))
	if operation == "" {
		operation = snapshot.Name
	}

	fields := dlog.Fields{
		"kind":      snapshot.Kind,
		"trace_id":  snapshot.TraceID,
		"span_id":   snapshot.SpanID,
		"operation": operation,
		"duration":  snapshot.Duration.String(),
		"statement": statement,
		"error":     dlog.ErrorValue(snapshot.Error),
	}
	if snapshot.Error != nil {
		dlog.ErrorFields("observe_db", "database observe error", fields)
		return
	}
	dlog.AccessFields("observe_db", "slow database observed", fields)
}

func truncateStatement(raw any) string {
	statement := strings.Join(strings.Fields(fmt.Sprint(raw)), " ")
	if len(statement) <= 240 {
		return statement
	}
	return statement[:240] + "..."
}

func applyObserveRequestFields(fields dlog.Fields, attrs map[string]any) {
	if len(attrs) == 0 {
		return
	}
	if origin := strings.TrimSpace(fmt.Sprint(attrs["http.origin"])); origin != "" {
		fields["origin"] = origin
	}
	if referer := strings.TrimSpace(fmt.Sprint(attrs["http.referer"])); referer != "" {
		fields["referer"] = referer
	}
	if clientPage := strings.TrimSpace(fmt.Sprint(attrs["http.client_page"])); clientPage != "" {
		fields["client_page"] = clientPage
	}
}
