package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const usageFactPersistenceTimeout = 10 * time.Second

type usageFactResponseGate struct {
	gin.ResponseWriter
	stream   bool
	protocol usageFactResponseProtocol

	mu           sync.Mutex
	pending      bytes.Buffer
	streamBuffer bytes.Buffer
	heldTerminal bool
	released     bool
	discarded    bool
	wroteHeader  bool
	status       int
	size         int
}

type usageFactResponseProtocol string

const (
	usageFactProtocolOpenAI    usageFactResponseProtocol = "openai"
	usageFactProtocolAnthropic usageFactResponseProtocol = "anthropic"
	usageFactProtocolGemini    usageFactResponseProtocol = "gemini"
)

func newUsageFactResponseGate(writer gin.ResponseWriter, stream bool, protocol usageFactResponseProtocol) *usageFactResponseGate {
	return &usageFactResponseGate{
		ResponseWriter: writer,
		stream:         stream,
		protocol:       protocol,
		status:         http.StatusOK,
		size:           -1,
	}
}

func installUsageFactResponseGate(c *gin.Context, stream bool, protocol usageFactResponseProtocol) (*usageFactResponseGate, func()) {
	original := c.Writer
	gate := newUsageFactResponseGate(original, stream, protocol)
	c.Writer = gate
	return gate, func() {
		if c.Writer != gate {
			return
		}
		_ = gate.Release()
		c.Writer = original
	}
}

func (g *usageFactResponseGate) WriteHeader(code int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.wroteHeader || g.discarded {
		return
	}
	g.wroteHeader = true
	g.status = code
	if g.stream || g.released {
		g.ResponseWriter.WriteHeader(code)
	}
}

func (g *usageFactResponseGate) WriteHeaderNow() {
	g.WriteHeader(g.Status())
}

func (g *usageFactResponseGate) Write(data []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.discarded {
		return len(data), nil
	}
	if !g.wroteHeader {
		g.wroteHeader = true
		g.status = http.StatusOK
		if g.stream || g.released {
			g.ResponseWriter.WriteHeader(http.StatusOK)
		}
	}
	if g.size < 0 {
		g.size = 0
	}
	g.size += len(data)
	if g.released {
		_, err := g.ResponseWriter.Write(data)
		return len(data), err
	}
	if !g.stream {
		_, _ = g.pending.Write(data)
		return len(data), nil
	}
	return g.writeStreamLocked(data)
}

func (g *usageFactResponseGate) WriteString(value string) (int, error) {
	return g.Write([]byte(value))
}

func (g *usageFactResponseGate) Flush() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stream && !g.discarded {
		g.ResponseWriter.Flush()
	}
}

func (g *usageFactResponseGate) Status() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.wroteHeader {
		return g.status
	}
	return g.ResponseWriter.Status()
}

func (g *usageFactResponseGate) Size() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.size
}

func (g *usageFactResponseGate) Written() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.wroteHeader
}

func (g *usageFactResponseGate) Release() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.released || g.discarded {
		return nil
	}
	g.released = true
	if !g.stream && g.wroteHeader {
		g.ResponseWriter.WriteHeader(g.status)
	}
	if g.stream && g.streamBuffer.Len() > 0 {
		if g.heldTerminal {
			_, _ = g.pending.Write(g.streamBuffer.Bytes())
		} else if _, err := g.ResponseWriter.Write(g.streamBuffer.Bytes()); err != nil {
			return err
		}
		g.streamBuffer.Reset()
	}
	if g.pending.Len() > 0 {
		if _, err := g.ResponseWriter.Write(g.pending.Bytes()); err != nil {
			return err
		}
		g.pending.Reset()
	}
	if g.stream {
		g.ResponseWriter.Flush()
	}
	return nil
}

func (g *usageFactResponseGate) Discard() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.discarded = true
	g.pending.Reset()
	g.streamBuffer.Reset()
}

func (g *usageFactResponseGate) writeStreamLocked(data []byte) (int, error) {
	_, _ = g.streamBuffer.Write(data)
	for {
		frameEnd := nextSSEFrameEnd(g.streamBuffer.Bytes())
		if frameEnd < 0 {
			break
		}
		frame := append([]byte(nil), g.streamBuffer.Next(frameEnd)...)
		if g.heldTerminal || isUsageFactTerminalSSEFrame(frame, g.protocol) {
			g.heldTerminal = true
			_, _ = g.pending.Write(frame)
			continue
		}
		if _, err := g.ResponseWriter.Write(frame); err != nil {
			return len(data), err
		}
	}
	if g.heldTerminal && g.streamBuffer.Len() > 0 {
		_, _ = g.pending.Write(g.streamBuffer.Bytes())
		g.streamBuffer.Reset()
	}
	return len(data), nil
}

func nextSSEFrameEnd(data []byte) int {
	lf := bytes.Index(data, []byte("\n\n"))
	crlf := bytes.Index(data, []byte("\r\n\r\n"))
	switch {
	case lf < 0 && crlf < 0:
		return -1
	case lf < 0:
		return crlf + 4
	case crlf < 0:
		return lf + 2
	case lf < crlf:
		return lf + 2
	default:
		return crlf + 4
	}
}

func isUsageFactTerminalSSEFrame(frame []byte, protocol usageFactResponseProtocol) bool {
	event, data := parseSSEFrame(frame)
	switch protocol {
	case usageFactProtocolAnthropic:
		return strings.EqualFold(event, "message_stop") || jsonType(data) == "message_stop"
	case usageFactProtocolGemini:
		return geminiSSEHasFinishReason(data)
	default:
		if strings.EqualFold(event, "error") || strings.TrimSpace(string(data)) == "[DONE]" {
			return true
		}
		switch jsonType(data) {
		case "response.completed", "response.failed", "response.incomplete", "response.cancelled", "response.canceled",
			"image_generation.completed", "image_edit.completed", "error":
			return true
		default:
			return false
		}
	}
}

func parseSSEFrame(frame []byte) (string, []byte) {
	var event string
	dataLines := make([]string, 0, 1)
	for _, line := range strings.Split(strings.ReplaceAll(string(frame), "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return event, []byte(strings.Join(dataLines, "\n"))
}

func jsonType(data []byte) string {
	var payload struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return ""
	}
	return payload.Type
}

func geminiSSEHasFinishReason(data []byte) bool {
	var payload struct {
		Candidates []struct {
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return false
	}
	for _, candidate := range payload.Candidates {
		if strings.TrimSpace(candidate.FinishReason) != "" {
			return true
		}
	}
	return false
}

func finalizeUsageFactResponse(
	c *gin.Context,
	gate *usageFactResponseGate,
	persist func(context.Context) error,
) error {
	if c == nil || gate == nil || persist == nil {
		return errors.New("usage fact response finalizer is incomplete")
	}
	parent := context.Background()
	if c.Request != nil {
		parent = context.WithoutCancel(c.Request.Context())
	}
	persistCtx, cancel := context.WithTimeout(parent, usageFactPersistenceTimeout)
	err := persist(persistCtx)
	cancel()
	if err == nil {
		releaseErr := gate.Release()
		c.Writer = gate.ResponseWriter
		return releaseErr
	}

	gate.Discard()
	c.Writer = gate.ResponseWriter
	writeUsageFactPersistenceError(c, gate)
	return err
}

func writeUsageFactPersistenceError(c *gin.Context, gate *usageFactResponseGate) {
	const message = "Unable to persist usage record"
	if gate.stream {
		c.Header("Content-Type", "text/event-stream")
		switch gate.protocol {
		case usageFactProtocolAnthropic:
			_, _ = c.Writer.WriteString("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"" + message + "\"}}\n\n")
		case usageFactProtocolGemini:
			_, _ = c.Writer.WriteString("data: {\"error\":{\"code\":503,\"message\":\"" + message + "\",\"status\":\"UNAVAILABLE\"}}\n\n")
		default:
			_, _ = c.Writer.WriteString("event: error\ndata: {\"type\":\"billing_persistence_error\",\"error\":{\"message\":\"" + message + "\"}}\n\n")
		}
		c.Writer.Flush()
		return
	}
	switch gate.protocol {
	case usageFactProtocolAnthropic:
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"type": "error", "error": gin.H{"type": "api_error", "message": message}})
	case usageFactProtocolGemini:
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"code": http.StatusServiceUnavailable, "message": message, "status": "UNAVAILABLE"}})
	default:
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": gin.H{"message": message, "type": "api_error", "code": "billing_persistence_error"}})
	}
}
