package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const maxInternalUsageEventBodyBytes = 1 << 20

type InternalUsageEventHandler struct {
	cfg     config.InternalUsageEventConfig
	service *service.InternalUsageEventService
}

func NewInternalUsageEventHandler(cfg *config.Config, service *service.InternalUsageEventService) *InternalUsageEventHandler {
	var eventCfg config.InternalUsageEventConfig
	if cfg != nil {
		eventCfg = cfg.InternalUsageEvent
	}
	return &InternalUsageEventHandler{cfg: eventCfg, service: service}
}

func (h *InternalUsageEventHandler) ReceiveUsageEvent(c *gin.Context) {
	if h == nil || h.service == nil || !h.cfg.Enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxInternalUsageEventBodyBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if len(body) > maxInternalUsageEventBodyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "body too large"})
		return
	}
	if err := h.verify(c, body); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var event service.CLIProxyUsageEvent
	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	result, err := h.service.RecordCLIProxyUsageEvent(c.Request.Context(), event, body)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrAPIKeyNotFound) || errors.Is(err, service.ErrInvalidInput) || errors.Is(err, service.ErrUsageBillingRequestIDRequired) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"request_id": result.RequestID,
		"created":    result.Created,
		"skipped":    result.Skipped,
	})
}

func (h *InternalUsageEventHandler) verify(c *gin.Context, body []byte) error {
	token := strings.TrimSpace(h.cfg.Token)
	secret := strings.TrimSpace(h.cfg.HMACSecret)
	if token == "" || secret == "" {
		return errors.New("internal usage event secret is not configured")
	}
	providedToken := strings.TrimSpace(c.GetHeader("x-internal-token"))
	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(token)) != 1 {
		return errors.New("invalid token")
	}
	timestamp := strings.TrimSpace(c.GetHeader("x-usage-timestamp"))
	if timestamp == "" {
		return errors.New("missing timestamp")
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return err
	}
	skew := h.cfg.MaxSkewSeconds
	if skew <= 0 {
		skew = 300
	}
	now := time.Now().Unix()
	if ts < now-int64(skew) || ts > now+int64(skew) {
		return errors.New("timestamp outside allowed skew")
	}
	providedSignature := strings.TrimSpace(c.GetHeader("x-usage-signature"))
	if providedSignature == "" {
		return errors.New("missing signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("\n"))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(providedSignature), []byte(expected)) != 1 {
		return errors.New("invalid signature")
	}
	return nil
}
