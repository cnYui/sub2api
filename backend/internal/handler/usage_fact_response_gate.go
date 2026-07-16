package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const usageFactPersistenceTimeout = 10 * time.Second

type usageFactResponseGate struct {
	gin.ResponseWriter
	stream bool

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

func newUsageFactResponseGate(writer gin.ResponseWriter, stream bool) *usageFactResponseGate {
	return &usageFactResponseGate{
		ResponseWriter: writer,
		stream:         stream,
		status:         http.StatusOK,
		size:           -1,
	}
}

func installUsageFactResponseGate(c *gin.Context, stream bool) (*usageFactResponseGate, func()) {
	original := c.Writer
	gate := newUsageFactResponseGate(original, stream)
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
		if g.heldTerminal || isUsageFactTerminalSSEFrame(frame) {
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

func isUsageFactTerminalSSEFrame(frame []byte) bool {
	return bytes.Contains(frame, []byte(`"response.completed"`)) ||
		bytes.Contains(frame, []byte(`"response.incomplete"`)) ||
		bytes.Contains(frame, []byte("[DONE]")) ||
		bytes.Contains(frame, []byte(`"message_stop"`))
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
	if gate.stream {
		c.Header("Content-Type", "text/event-stream")
		_, _ = c.Writer.WriteString("event: error\ndata: {\"type\":\"billing_persistence_error\",\"error\":{\"message\":\"Unable to persist usage record\"}}\n\n")
		c.Writer.Flush()
		return err
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
		"error": gin.H{
			"message": "Unable to persist usage record",
			"type":    "api_error",
			"code":    "billing_persistence_error",
		},
	})
	return err
}
