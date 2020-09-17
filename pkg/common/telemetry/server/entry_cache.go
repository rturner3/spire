package server

import "github.com/spiffe/spire/pkg/common/telemetry"

// StartEntryCacheReload returns metric for
// reload of the server's registration entry cache.
func StartEntryCacheReload(m telemetry.Metrics) *telemetry.CallCounter {
	return telemetry.StartCall(m, telemetry.Entry, telemetry.CacheManager, telemetry.Sync)
}
