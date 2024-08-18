- You can use the _propagation_ package from _otel_ for _context propagators_ (https://pkg.go.dev/go.opentelemetry.io/otel/propagation). _Context Propagation_ ensures that trace info is carried across process boundaries. This helps you to stitch together traces that span multiple services.
- `TextMapCarrier` is teh carrier that holds the tracing ctx (generally, k-v pairs) as it gets passed across service boundaries.
- Example:
	- `TextMapPropagator` propagates cross-cutting concerns as key-value text pairs within a carrier that travels in-band across process boundaries, ie., it's used to define how context propagation should occur between different service or components in a DS. 
	- It allows tracing info such as trace ids, span ids etc.. to be injected into and exctracted from _carrier_ objects such as HTTP headers or message queues.

Use this when you need to trace requests e2e through various layers of the system.

