
- To get the build info of the binary, you can use the `debug` package.. `debug.ReadRuntimeInfo`.
- you can create a `program` struct to abstract your whole app and specify the deps in the struct and have a `run` func to start the app, for ex: 
```
type program struct {
	log        *slog.Logger
	tracer     trace.TracerProvider
	propagator propagation.TextMapPropagator
	meter      metric.MeterProvider
}

func (p *program) Run() error {}
```

- If your program exists because of a `panic`, best to record the stack:
```
defer func() {
	if r := recover; r != nil {
		span.RecordError(fmt.Errorf("%v", r), trace.WithAttributes(attribute.String("stack_trace", string(debug.Stack()))))
		span.SetStatus(codes.Error, "program killed by a panic)
		span.End()
		panic(r)
	}

	// Any other error
	if err != nil {
		span.RecordError(err)	
		span.SetStatus(codes.Error, "program exited with error")
	} else {
		span.SetStatus(codes.Ok, "")	
	}
	span.End()
}
```

- you can use uber's `automaxprocs` to  dynamically monitor the container's Linux environment, particularly the CPU quotas and available CPUs, and automatically adjusts GOMAXPROCS accordingly. 
```
	// Set GOMAXPROCS to match Linux container CPU quota on Linux.
	if runtime.GOOS == "linux" {
		if _, err := maxprocs.Set(maxprocs.Logger(p.log.Info)); err != nil {
			p.log.Error("cannot set GOMAXPROCS", slog.Any("error", err))
		}
	}
```