package capture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/NextChapterSoftware/chissl/share/settings"
)

// HTTP-aware tap that parses request/response when possible and publishes events to the Service.

const maxPeek = 4096

type TapImpl struct {
	svc       *Service
	meta      Meta
	tunnelID  string
	maxEvents int
	// state
	emittedReqHeaders bool
	emittedResHeaders bool
}

type Meta struct {
	Username string
	Remote   settings.Remote
	ConnID   string
}

func (t *TapImpl) OnOpen() {
	t.svc.AddEvent(t.tunnelID, Event{Time: time.Now(), TunnelID: t.tunnelID, User: t.meta.Username, ConnID: t.meta.ConnID, Type: ConnOpen, Meta: map[string]any{"conn_id": t.meta.ConnID}}, t.maxEvents)
	if t.svc != nil && t.svc.onConnDelta != nil {
		t.svc.onConnDelta(t.tunnelID, +1)
	}
}

func (t *TapImpl) OnClose(sent, received int64) {
	// Emit metrics and conn close
	t.svc.AddEvent(t.tunnelID, Event{Time: time.Now(), TunnelID: t.tunnelID, User: t.meta.Username, ConnID: t.meta.ConnID, Type: Metric, Meta: map[string]any{"conn_id": t.meta.ConnID, "sent": sent, "received": received}}, t.maxEvents)
	if t.svc != nil && t.svc.onMetric != nil {
		t.svc.onMetric(t.tunnelID, sent, received)
	}
	t.svc.AddEvent(t.tunnelID, Event{Time: time.Now(), TunnelID: t.tunnelID, User: t.meta.Username, ConnID: t.meta.ConnID, Type: ConnClose, Meta: map[string]any{"conn_id": t.meta.ConnID}}, t.maxEvents)
	if t.svc != nil && t.svc.onConnDelta != nil {
		t.svc.onConnDelta(t.tunnelID, -1)
	}
}

// Implement separate writers for src and dst directions

type dirWriter struct {
	t *TapImpl
	// true if src->dst (client to upstream); false for dst->src (upstream to client)
	src bool
}

// SrcWriter receives bytes from client -> upstream
func (t *TapImpl) SrcWriter() io.Writer { return &dirWriter{t: t, src: true} }

// DstWriter receives bytes from upstream -> client
func (t *TapImpl) DstWriter() io.Writer { return &dirWriter{t: t, src: false} }

func (w *dirWriter) Write(p []byte) (int, error) {
	// Best-effort: parse HTTP headers once per direction
	if isLikelyHTTP(p) {
		br := bufio.NewReader(bytes.NewReader(p))
		if w.src && !w.t.emittedReqHeaders {
			if req, err := http.ReadRequest(br); err == nil {
				j, _ := json.Marshal(map[string]any{
					"method": req.Method,
					"path":   req.URL.String(),
					"header": req.Header,
				})
				w.t.svc.AddEvent(w.t.tunnelID, Event{Time: time.Now(), TunnelID: w.t.tunnelID, User: w.t.meta.Username, ConnID: w.t.meta.ConnID, Type: ReqHeaders, Meta: map[string]any{"conn_id": w.t.meta.ConnID, "method": req.Method, "path": req.URL.String()}, Data: j}, w.t.maxEvents)
				w.t.emittedReqHeaders = true
			}
		} else if !w.src && !w.t.emittedResHeaders {
			if res, err := http.ReadResponse(br, nil); err == nil {
				j, _ := json.Marshal(map[string]any{
					"status": res.Status,
					"code":   res.StatusCode,
					"header": res.Header,
				})
				w.t.svc.AddEvent(w.t.tunnelID, Event{Time: time.Now(), TunnelID: w.t.tunnelID, User: w.t.meta.Username, ConnID: w.t.meta.ConnID, Type: ResHeaders, Meta: map[string]any{"conn_id": w.t.meta.ConnID, "status": res.Status, "code": res.StatusCode}, Data: j}, w.t.maxEvents)
				w.t.emittedResHeaders = true
			}
		}
	}
	// Always emit raw chunk (truncated)
	cp := make([]byte, len(p))
	copy(cp, p)
	etype := ReqBody
	if !w.src {
		etype = ResBody
	}
	w.t.svc.AddEvent(w.t.tunnelID, Event{Time: time.Now(), TunnelID: w.t.tunnelID, User: w.t.meta.Username, ConnID: w.t.meta.ConnID, Type: etype, Meta: map[string]any{"conn_id": w.t.meta.ConnID}, Data: cp}, w.t.maxEvents)
	return len(p), nil
}

func isLikelyHTTP(b []byte) bool {
	bb := bytes.ToUpper(bytes.TrimSpace(b))
	for _, m := range [][]byte{[]byte("GET "), []byte("POST "), []byte("PUT "), []byte("PATCH "), []byte("DELETE "), []byte("HEAD "), []byte("OPTIONS "), []byte("HTTP/")} {
		if bytes.HasPrefix(bb, m) {
			return true
		}
	}
	return false
}
