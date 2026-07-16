//go:build linux

package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/e2b-dev/infra/packages/shared/pkg/logger"
)

func (h *APIStore) Logs(c *gin.Context) {
	ctx := c.Request.Context()
	sbx, err := h.sandboxes.GetByHostPort(c.Request.RemoteAddr)
	if err != nil {
		h.sendAPIStoreError(c, http.StatusBadRequest, "Error when finding source sandbox")
		ip, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		h.logger.Error(ctx, "error finding sandbox for source addr", logger.WithSandboxIP(ip), zap.Error(err))

		return
	}

	sbxID := sbx.Runtime.SandboxID

	payload := make(map[string]any)
	if err := c.ShouldBindJSON(&payload); err != nil {
		h.sendAPIStoreError(c, http.StatusBadRequest, "Invalid body for logs")
		h.logger.Error(ctx, "error when parsing sandbox logs request", zap.Error(err), logger.WithSandboxID(sbxID))

		return
	}

	err = h.validatePayloadSandboxID(payload, sbxID)
	if err != nil {
		h.sendAPIStoreError(c, http.StatusBadRequest, "Invalid sandboxID in logs payload")
		// Inflight logs with old sandboxID from snapshotted sandbox
		// Change to error once we have a way how to tell sandbox to flush and stop sending logs when being paused
		h.logger.Warn(ctx, "error when parsing sandbox logs request", zap.Error(err), logger.WithSandboxID(sbxID))

		return
	}

	// Overwrite instanceID, envID, and teamID to avoid spoofing
	payload["instanceID"] = sbxID
	payload["envID"] = sbx.Runtime.TemplateID
	payload["teamID"] = sbx.Runtime.TeamID

	correctStaleTimestamp(payload, sbx.LifecycleStartedAt)

	logs, err := json.Marshal(payload)
	if err != nil {
		h.sendAPIStoreError(c, http.StatusInternalServerError, "Error when parsing logs payload")
		h.logger.Error(ctx, "error when parsing logs payload", zap.Error(err), logger.WithSandboxID(sbxID))

		return
	}

	request, err := http.NewRequestWithContext(c, http.MethodPost, h.collectorAddr, bytes.NewBuffer(logs))
	if err != nil {
		h.sendAPIStoreError(c, http.StatusInternalServerError, "Error when creating request to forwarding sandbox logs")
		h.logger.Error(ctx, "error when creating request to forwarding sandbox logs", zap.Error(err), logger.WithSandboxID(sbxID))

		return
	}

	request.Header.Set("Content-Type", "application/json")
	response, err := h.collectorClient.Do(request)
	if err != nil {
		h.sendAPIStoreError(c, http.StatusInternalServerError, "Error when forwarding sandbox logs")
		h.logger.Error(ctx, "error when forwarding sandbox logs", zap.Error(err), logger.WithSandboxID(sbxID))

		return
	}
	defer response.Body.Close()

	c.Status(http.StatusOK)
}

// validatePayloadSandboxID checks if the payload contains correct instanceID to prevent slow requests to contaminating the logs of other sandboxes.
func (h *APIStore) validatePayloadSandboxID(payload map[string]any, sbxID string) error {
	if payload["instanceID"] == nil {
		return errors.New("missing sandboxID in logs payload")
	}

	payloadSandboxID, ok := payload["instanceID"].(string)
	if !ok {
		return fmt.Errorf("instanceID in logs payload is not a string: %v", payload["instanceID"])
	}

	if payloadSandboxID != sbxID {
		return fmt.Errorf("sandboxID in logs payload does not match the sandboxID of the source sandbox (%s != %s)", payloadSandboxID, sbxID)
	}

	return nil
}

// envdTimestampLayout matches zerolog.TimeFieldFormat as configured by envd's
// logger (see packages/envd/internal/logs/logger.go), i.e. time.RFC3339Nano.
const envdTimestampLayout = time.RFC3339Nano

// staleTimestampSlack is a small safety margin so correctStaleTimestamp only
// clamps records that are genuinely stale (a snapshot's guest clock,
// potentially days/months off), not ones a hair before lifecycleStart from
// ordinary clock skew: lifecycleStart is recorded on the host before the
// guest even resumes, so a correctly-corrected guest timestamp should
// normally land after it, but host/guest clocks are never perfectly in sync
// down to the millisecond.
const staleTimestampSlack = time.Minute

// correctStaleTimestamp rewrites a pre-clock-correction "timestamp" field on
// an envd log record.
//
// A sandbox resumed from a memory snapshot boots with the guest wall clock
// still holding the template snapshot's time. On resume, envd's HTTP log
// exporter (packages/envd/internal/logs/exporter) is itself part of that
// restored memory image - its sending goroutine and cached MMDS options
// survive the snapshot - so it can flush startup/resume diagnostics through
// this same path immediately, before the new /init call even arrives, let
// alone corrects the clock (see envd's SetData/setSystemTime). Those records
// are legitimate but carry a stale event time.
//
// lifecycleStart is Sandbox.LifecycleStartedAt: the orchestrator's own
// host-clock instant this specific Firecracker lifecycle's Sandbox object was
// constructed (before the sandbox is registered in the network map, and
// therefore before this path can receive any log for it). Unlike
// Sandbox.GetStartedAt() - which is caller-seeded and can carry a value from
// a prior lifecycle or API request - LifecycleStartedAt never inherits a
// value from a caller or a prior lifecycle, so it can't itself be stale. A
// record timestamped more than staleTimestampSlack before it must have been
// emitted with the guest's pre-correction clock; it is clamped to
// lifecycleStart so it lands after the correction, and the original value is
// preserved under "original_timestamp" instead of being discarded.
//
// Genuinely queued/in-flight records from a *previous* lifecycle (rather than
// newly emitted ones replayed from the snapshotted exporter, as above) are a
// separate, deliberately out-of-scope edge case; this only re-times records
// already attributed to the current lifecycle by validatePayloadSandboxID.
//
// If lifecycleStart is zero (shouldn't happen for any sandbox reachable
// through GetByHostPort, but kept as a defensive no-op) or the payload has no
// parseable "timestamp" field, the payload is left untouched.
func correctStaleTimestamp(payload map[string]any, lifecycleStart time.Time) {
	if lifecycleStart.IsZero() {
		return
	}

	raw, ok := payload["timestamp"].(string)
	if !ok {
		return
	}

	ts, err := time.Parse(envdTimestampLayout, raw)
	if err != nil {
		return
	}

	if ts.Before(lifecycleStart.Add(-staleTimestampSlack)) {
		payload["original_timestamp"] = raw
		payload["timestamp"] = lifecycleStart.UTC().Format(envdTimestampLayout)
	}
}
