package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.19.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials/insecure"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

var (
	httpAddr  = flag.String("http", "localhost:8080", "HTTP service address to listen for incoming request on")
	grpcAddr  = flag.String("grpc", "localhost:8081", "gRPC service address to listen for incoming request on")
	probeAddr = flag.String("probe", "localhost:7070", "probe (inspected) HTTP service address")
	version   = flag.Bool("version", false, "Print build info")

	buildInfo, _ = debug.ReadBuildInfo()
)

func buildTelemetryInfo() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.ServiceName("api"),
		semconv.ServiceVersion("1.0.0"),
		attribute.Key("build.go").String(runtime.Version()),
	}
	for _, s := range attrs {
		switch s.Key {
		case "vcs.revision", "vsc.time":
			attrs = append(attrs, attribute.Key("build."+s.Key).String(s.Value.AsString()))
		case "vcs.modified":
			attrs = append(attrs, attribute.Key("build.vcs.modified").Bool(s.Value.AsBool()))
		}
	}
	return attrs
}

func main() {
	flag.Parse()
	if *version {
		fmt.Println(buildInfo)
		os.Exit(2)
	}

	p := &program{log: slog.Default()}
	haltTelemetry, err := p.telemetry()
	if err != nil {
		p.log.Error("cannot initialize telemetry", slog.Any("error", err))
		os.Exit(1)
	}

	// Setting catch-all global telemetry providers
	otel.SetTracerProvider(p.tracer)
	otel.SetTextMapPropagator(p.propagator)
	otel.SetMeterProvider(p.meter)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		p.log.Error("irremediable OpenTelemetry event", slog.Any("error", err))
	}))

	defer func() {
		if err != nil {
			os.Exit(1)
		}
	}()
	defer haltTelemetry()

}

type program struct {
	log        *slog.Logger
	tracer     trace.TracerProvider
	propagator propagation.TextMapPropagator
	meter      metric.MeterProvider
}

func (p *program) telemetry() (halt func(), err error) {
	ctx := context.Background()
	p.propagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	var (
		tr sdktrace.SpanExporter
		mt sdkmetric.Exporter
	)

	switch exporter, ok := os.LookupEnv("OTEL_EXPORTER"); {
	case exporter == "stdout":
		if tr, err = stdouttrace.New(); err != nil {
			return nil, fmt.Errorf("stdouttrace: %w", err)
		}
		if mt, err = stdoutmetric.New(stdoutmetric.WithEncoder(json.NewEncoder(os.Stdout))); err != nil {
			return nil, fmt.Errorf("stdoutmetric: %w", err)
		}
	case exporter == "otlp":
		if tr, err = otlptracegrpc.New(ctx, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials())); err != nil {
			return nil, fmt.Errorf("otlptracegrpc: %w", err)
		}
		if mt, err = otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithTLSCredentials(insecure.NewCredentials())); err != nil {
			return nil, fmt.Errorf("otlpmetricgrpc: %w", err)
		}
	case ok:
		p.log.Warn("unknown OTEL_EXPORTER value")
		fallthrough
	default:
		p.tracer = tracenoop.NewTracerProvider()
		p.meter = noop.NewMeterProvider()
		return func() {}, nil
	}

	res, err := resource.New(ctx, resource.WithAttributes(buildTelemetryInfo()...))
	if err != nil {
		return nil, fmt.Errorf("cannot init tracer resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithResource(res), sdktrace.WithBatcher(tr))
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewPeriodicReader(mt)))
	p.tracer = tp
	p.meter = mp

	return func() {
		haltCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if err := tp.Shutdown(haltCtx); err != nil {
				p.log.Error("telemetry tracer shutdown", slog.Any("error", err))
			}
		}()
		go func() {
			defer wg.Done()
			if err := mp.Shutdown(haltCtx); err != nil {
				p.log.Error("telemetry meter shutdown", slog.Any("error", err))
			}
		}()
		wg.Wait()
	}, nil
}
