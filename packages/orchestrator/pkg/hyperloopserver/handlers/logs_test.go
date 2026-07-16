//go:build linux

package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCorrectStaleTimestamp(t *testing.T) {
	t.Parallel()

	lifecycleStart, err := time.Parse(envdTimestampLayout, "2026-07-16T10:00:00Z")
	require.NoError(t, err)

	t.Run("clamps a timestamp from before the corrected clock", func(t *testing.T) {
		t.Parallel()

		staleRaw := "2026-06-09T08:00:00.123456789Z"
		payload := map[string]any{"timestamp": staleRaw}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, lifecycleStart.Format(envdTimestampLayout), payload["timestamp"])
		assert.Equal(t, staleRaw, payload["original_timestamp"])
	})

	t.Run("leaves a post-correction timestamp untouched", func(t *testing.T) {
		t.Parallel()

		freshRaw := lifecycleStart.Add(time.Second).Format(envdTimestampLayout)
		payload := map[string]any{"timestamp": freshRaw}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, freshRaw, payload["timestamp"])
		assert.NotContains(t, payload, "original_timestamp")
	})

	t.Run("leaves a timestamp exactly at lifecycleStart untouched", func(t *testing.T) {
		t.Parallel()

		boundaryRaw := lifecycleStart.Format(envdTimestampLayout)
		payload := map[string]any{"timestamp": boundaryRaw}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, boundaryRaw, payload["timestamp"])
		assert.NotContains(t, payload, "original_timestamp")
	})

	t.Run("leaves a timestamp exactly at the slack cutoff untouched", func(t *testing.T) {
		t.Parallel()

		// The comparison is strictly Before, so a timestamp exactly
		// staleTimestampSlack before lifecycleStart is the last one that must
		// NOT be clamped.
		cutoffRaw := lifecycleStart.Add(-staleTimestampSlack).Format(envdTimestampLayout)
		payload := map[string]any{"timestamp": cutoffRaw}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, cutoffRaw, payload["timestamp"])
		assert.NotContains(t, payload, "original_timestamp")
	})

	t.Run("leaves a slightly earlier timestamp within slack untouched", func(t *testing.T) {
		t.Parallel()

		// A record legitimately timestamped a hair before lifecycleStart due
		// to ordinary host/guest clock skew, not because the guest clock was
		// actually still wrong, must not be relabeled stale.
		withinSlackRaw := lifecycleStart.Add(-staleTimestampSlack / 2).Format(envdTimestampLayout)
		payload := map[string]any{"timestamp": withinSlackRaw}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, withinSlackRaw, payload["timestamp"])
		assert.NotContains(t, payload, "original_timestamp")
	})

	t.Run("clamps a timestamp just beyond the slack window", func(t *testing.T) {
		t.Parallel()

		staleRaw := lifecycleStart.Add(-staleTimestampSlack - time.Second).Format(envdTimestampLayout)
		payload := map[string]any{"timestamp": staleRaw}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, lifecycleStart.Format(envdTimestampLayout), payload["timestamp"])
		assert.Equal(t, staleRaw, payload["original_timestamp"])
	})

	t.Run("is a no-op when lifecycleStart is zero", func(t *testing.T) {
		t.Parallel()

		staleRaw := "2026-06-09T08:00:00Z"
		payload := map[string]any{"timestamp": staleRaw}

		correctStaleTimestamp(payload, time.Time{})

		assert.Equal(t, staleRaw, payload["timestamp"])
		assert.NotContains(t, payload, "original_timestamp")
	})

	t.Run("is a no-op when timestamp is missing", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{"message": "hello"}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.NotContains(t, payload, "timestamp")
		assert.NotContains(t, payload, "original_timestamp")
	})

	t.Run("is a no-op when timestamp is not a string", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{"timestamp": 12345}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, 12345, payload["timestamp"])
		assert.NotContains(t, payload, "original_timestamp")
	})

	t.Run("is a no-op when timestamp is unparseable", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{"timestamp": "not-a-time"}

		correctStaleTimestamp(payload, lifecycleStart)

		assert.Equal(t, "not-a-time", payload["timestamp"])
		assert.NotContains(t, payload, "original_timestamp")
	})
}
