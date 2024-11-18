package tracer

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/gnolang/gno/tm2/pkg/telemetry/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func Init(config config.Config) error {
	ctx := context.Background()
	exp, err := httpExport(ctx, config)
	if err != nil {
		return fmt.Errorf("new http exporter: %w", err)
	}

	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("XXX_GNO"),
		),
	)
	if err != nil {
		return fmt.Errorf("unable to setup ressource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		trace.WithResource(res),
		trace.WithBatcher(exp,
			// Default is 5s. Set to 1s for demonstrative purposes.
			trace.WithBatchTimeout(time.Second),
		),
	)

	otel.SetTracerProvider(tp)
	return nil
}

func httpExport(ctx context.Context, config config.Config) (exp sdktrace.SpanExporter, err error) {
	fmt.Printf("creating exporter\r\n")
	u, err := url.Parse(config.TracingExporterEndpoint)
	if err != nil {
		return nil, fmt.Errorf("error parsing exporter endpoint: %s, %w", config.TracingExporterEndpoint, err)
	}

	// Use oltp trace exporter with http/https or grpc
	switch u.Scheme {
	case "http", "https":
		fmt.Printf("creating tracer exporter %s\r\n", config.TracingExporterEndpoint)
		exp, err = otlptracehttp.New(
			ctx,
			otlptracehttp.WithEndpointURL(config.TracingExporterEndpoint),
		)
		if err != nil {
			return nil, err
		}
		// default:
		// 	exp, err = otlptracegrpc.New(
		// 		ctx,
		// 		otlptracegrpc.WithEndpoint(config.ExporterEndpoint),
		// 		otlptracegrpc.WithInsecure(),
		// 	)
		// 	if err != nil {
		// 		return nil, fmt.Errorf("unable to create grpc traces exporter, %w", err)
		// 	}
	}

	return exp, nil
}
