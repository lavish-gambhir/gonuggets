tldr:
- A `trace` gives you a view of the lifespan of a request -- it represents a user flow through the app. 
- You can also have tracers for long running tasks, and have them running for multiple days.
- It's a collection of `Span`s - some unit operation you'd like to document and measure. It contains:
	- _Attributes_: k-v pairs; tags; metadata.
	- _Events_: named strings.
	- _Parent_: previous span that encapsulates this one.
- Now, you don't need to collect all Spans, you can use a different collection mechanism as per needs -> _Sampler_: always, probabilistic, errors only etc.
- _Exporter_: the backend receiving the traces, metrics etc.. and processing them.

---
# Instrumentation
- adding obeservability code.

Why trace?A
- Improve the **observability** of your app.
- E2E visibility across DS.
- Understand service to service deps.
- Identify perf. bottlenecks.

There are 3 main steps to initialize a tracer:
1. Identify the app as a resource: `resource.New`
2. Init an Exported (depends on the backend): `jaeger.New`
3. Init a tracer provider: `trace.NewTracerProvider`

---

#### Resources
Repr the entity (your app) producing tel. as resource attrs. For ex., a process producing telemetry is running in a container ona a k8s node has a process name, a Pod name, a namespace, and possible a Deployment name. All four of these attrs can be included in the `resource`:
```
res := resource.NewWithAttributes(
	semconv.SechamURL,
	semconv.ServiceNameKey.String("nucleus"),
	semconv.ServiceVersionKey.String("1.0.0"),
	semconv.ServiceInstanceIDKey.String("adbsdf"),
)

//...

provider := sdktrace.NewTracerProvider(
	...,
	sdktrace.WithResource(res),
)

// Resources can also be detected auto. using `resource.Detector` impls.
// These detectors may discover info about the currently running process, the OS it's running on, 
// the cp hosting the OS instance.. etc.
res, err := resource.New(
	ctx,
	resource.WithFromEnv(),
	resource.WithTelemetrySDK(),
	resource.WithProcess(),
	resource.WithOS(),
	resource.WithContainer(),
	resource.WithHost(),
	resource.WithAttributes(...)
	// resource.WithDetectors(thirdpart.Detector{})	you can use your own `Detector` impl as well.
)
```
#### Span Attributes
k-v pairs that are applied as metadata to your spans and are useful for aggregating, filtering, and grouping traces. Attrs can be added at span creation, or at any other time during the lifecycle of a span before it's completed
-  The `context` package is used to store the active span. When you start a span, you'll get handle on not only the span that's created, but the modified context that contains it: `ctx, span := tracer.Start(r.Context(), "q-handler")`. To get the current span, you can pull it out of context: `span := tracer.SpanFromContext(ctx)`
```
ctx, span := tracer.Start(ctx, "q-span", trace.WithAttributes(attributes.String("foo", "bar")))

//....
// ....

span.SetAttributes(attribute.Bool("isAuthenticated", true), attributes.String("user-id", "hex"))

var authKey = attribute.Key("authKey") // precomputed attr
span.SetAttributes(authKey.String("hex"))
```

#### Events
This is a msg on a span that indicates "something happening" during it's lifetime.. esentially, if some important 'Event' you want to capture. For events, the timestamps are displayed as offsets from the beginning of the span, so you can how much time elapsed between them.
```
span.AddEvent("acquiring main lock")
mutex.Lock()
span.AddEvent("main lock aquired")
span.AddEvent("unlocking)
mutex.Unlock()

// Events can also have attributes
span.AddEvent("cancelling wait due to context cancellation", trace.WithAttributes(attribute.Int("pid", 312), attributes.String("signal", "SIGUP"))))
```


#### Span Status
You can set a _status_ on a _span_ to specify that a Span has not completed successfully -- `Error`. By def, all Spans are `Unset`, a span completed without error. The `Ok` status is reserved when you need to EXPLICITLY mark a span as successful rather than stick with the default of `Unset`.  This goes along with `Record` error.
```
span.SetStatus(codes.Error, "failed to fetch contentID")
span.RecordError(err)

NOTE: `RecordError` func doesn't auto set a span status when called!
```


#### Trace Propagation
Traces can extend beyond a single process -- _Context Propagation_, a mechanism where identifiers for a trace are sent to a remote processes. To do this, a `propagator` must be registered with the otel API:
- You can use the _propagation_ package from _otel_ for _context propagators_ (https://pkg.go.dev/go.opentelemetry.io/otel/propagation). _Context Propagation_ ensures that trace info is carried across process boundaries. This helps you to stitch together traces that span multiple services.
- `TextMapCarrier` is teh carrier that holds the tracing ctx (generally, k-v pairs) as it gets passed across service boundaries.
- Example:
	- `TextMapPropagator` propagates cross-cutting concerns as key-value text pairs within a carrier that travels in-band across process boundaries, ie., it's used to define how context propagation should occur between different service or components in a DS. 
	- It allows tracing info such as trace ids, span ids etc.. to be injected into and exctracted from _carrier_ objects such as HTTP headers or message queues.
Use this when you need to trace requests e2e through various layers of the system.
```
otel.SetTextMapPropagator(propagation.TraceContext{})
```
>[!note] after configuring context propagation, you can rely on auto. instrumentation to handle serializing of context.


## Metrics
To start producing actionable metrics, you need to init `MeterProvider` that gives you access to `Meter`. If `MeterProvider` is not created, the otel Metrics API will use a no-op impl and fail to generate data. 
Supported metrics: `Counter (sync), Counter (async), Histogram (sync), Gauge (async), UpDownCounter(sync), UpDownCounter(async)`.
- So, the init components are appox. same as for tracing: register the app as a resource, init metrics exporter, and metrics provider:
```
	res, _ := newResource()
	meterProvider, _ := newMeterProvider(res)
	otel.SetMeterProvider(meterProvider) // internally takes care of creating a metric exporter.

	// shutdown
	defer func() {
		if err := meterProvider.Shutdown(ctx); err != nil { log.Fatal(err) }	
	}()

	meter := otel.Meter("github.com/observer/inventory")
```

- otel instruments are eiter sync or async.
	- Sync instruments takes a measurement _when they're called_. Periodically, the aggregation of these measurements is exported by a configured exported. Because the measurements are decoupled from exporting values, an export cycle may contain zero or multiple aggregated measurements.
	- Async instruments provide measurement _at the request of the SDK_. When the SDK exports, a callback that was provided to the instrument on creation is invoked. This callback func provides the SDK with a measurement that is immediatly exported. All measurements on async instruments are performed once per export cycle. When to use async instruments:
		1. When updating a couter is computationally expensive, and you don't want the current thread of execution to wait for measurement.
		2. Observations need to happen at frequencies unrelated to program execution.

### Exporters
Send telemetry data to the otel collector to make sure it's exported correctly. There're a bunch of exporters available: `stdouttrace`, `stdoutmetrics`, `stdoutlog`, `otlptracehttp`, `otlptracegrpc`, `otlpmetrichttp`, `otlpmetricgrpc`, `prometheus`, `otlploghttp`. The exporters which send data over the wire to some backend follow the OTLP:
- OpenTelemetry Protocol spec describes the encoding, transport, and delivery mechanism of telemetry data between telemetry sources, intermediate nodes such as collectors and telemetry backends. OTLP is a general-purpose telemetry data delivery proto designed in the scope of OTel project.

### Sampling
A process that restricts the amount of spans  that are generated by a system. The exact Sampler to use depends on your specific needs.  When you're getting started, or in a dev env. use `AlwaysSample`:
```
provider := trace.NewTracerProvider(
	trace.WithSampler(trace.AlwaysSample()),
	...
)
```