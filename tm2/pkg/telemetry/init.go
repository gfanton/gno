package telemetry

// Inspired by the example here:
// https://github.com/open-telemetry/opentelemetry-go/blob/main/example/prometheus/main.go

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gnolang/gno/tm2/pkg/telemetry/config"
	"github.com/gnolang/gno/tm2/pkg/telemetry/metrics"
	"github.com/gnolang/gno/tm2/pkg/telemetry/tracer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	globalConfig         config.Config
	telemetryInitialized atomic.Bool
)

type TracerFactory func() trace.Tracer

func Tracer(name string, options ...trace.TracerOption) TracerFactory {
	var once sync.Once
	return func() (t trace.Tracer) {
		once.Do(func() {
			var provider trace.TracerProvider
			if MetricsEnabled() {
				provider = otel.GetTracerProvider()
			} else {
				provider = noop.NewTracerProvider()
			}

			t = provider.Tracer(name, options...)
		})

		return t
	}
}

// MetricsEnabled returns true if metrics have been initialized
func MetricsEnabled() bool {
	return globalConfig.MetricsEnabled
}

// Init initializes the global telemetry
func Init(c config.Config) error {
	// Check if the metrics are enabled at all
	if !c.MetricsEnabled {
		return nil
	}

	// Validate the configuration
	if err := c.ValidateBasic(); err != nil {
		return fmt.Errorf("unable to validate config, %w", err)
	}

	// Check if it's been enabled already
	if !telemetryInitialized.CompareAndSwap(false, true) {
		return nil
	}

	// Update the global configuration
	globalConfig = c

	if err := metrics.Init(c); err != nil {
		return fmt.Errorf("unable to init metrics: %w", err)
	}

	if err := tracer.Init(c); err != nil {
		return fmt.Errorf("unable to init tracer: %w", err)
	}

	return nil
}
