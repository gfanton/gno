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

var noopTracerProvider = noop.NewTracerProvider()

func Tracer(name string, options ...trace.TracerOption) TracerFactory {
	var once sync.Once
	var t trace.Tracer = noopTracerProvider.Tracer(name, options...) // Initilize noop tracer as default
	return func() trace.Tracer {
		if TracingEnabled() {
			once.Do(func() {
				provider := otel.GetTracerProvider()
				t = provider.Tracer(name, options...)
			})
		}

		return t
	}
}

// MetricsEnabled returns true if metrics have been initialized
func MetricsEnabled() bool {
	return globalConfig.MetricsEnabled
}

// MetricsEnabled returns true if metrics have been initialized
func TracingEnabled() bool {
	return globalConfig.TracingEnabled
}

// Init initializes the global telemetry
func Init(c config.Config) error {
	// Validate the configuration
	if err := c.ValidateBasic(); err != nil {
		return fmt.Errorf("unable to validate config, %w", err)
	}

	if c.ExporterEndpoint != "" {
		if err := metrics.Init(c); err != nil {
			return fmt.Errorf("unable to init metrics: %w", err)
		}
	}

	if c.TracingExporterEndpoint != "" {
		if err := tracer.Init(c); err != nil {
			return fmt.Errorf("unable to init tracer: %w", err)
		}

	}

	// Update the global configuration
	globalConfig = c

	return nil
}
