package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	// "time"

	"github.com/maidol/kratos-layout/internal/conf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"

	cfg "github.com/go-kratos/kratos/contrib/config/etcd/v2"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus"
	clientv3 "go.etcd.io/etcd/client/v3"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	ggrpc "google.golang.org/grpc"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name = "hello"
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, rr registry.Registrar) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
		kratos.Registrar(rr),
	)
}

func main() {
	flag.Parse()
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var s conf.ConfigSource
	if err := c.Value("source").Scan(&s); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap

	switch s.Type {
	case "file":
		if err := c.Scan(&bc); err != nil {
			panic(err)
		}
	case "etcd":
		// create an etcd client
		client, err := clientv3.New(clientv3.Config{
			Endpoints:   s.Etcd.Address,
			Username:    s.Etcd.Username,
			Password:    s.Etcd.Password,
			DialTimeout: 5 * time.Second,
			DialOptions: []ggrpc.DialOption{ggrpc.WithBlock()},
		})
		if err != nil {
			log.Fatal(err)
		}

		// configure the source, "path" is required
		source, err := cfg.New(client, cfg.WithPath(s.Path), cfg.WithPrefix(true))
		if err != nil {
			log.Fatal(err)
		}

		// create a config instance with source
		appconf := config.New(
			config.WithSource(source),
		)
		defer c.Close()

		// load sources before get
		if err := appconf.Load(); err != nil {
			log.Fatal(err)
		}

		if err := appconf.Scan(&bc); err != nil {
			log.Fatal(err)
		}
	default:
		panic(fmt.Errorf("unsupported config source type: %s", s.Type))
	}

	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(bc.Trace.Endpoint)))
	if err != nil {
		panic(err)
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.ParentBased(tracesdk.TraceIDRatioBased(1.0))),
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewSchemaless(
			semconv.ServiceNameKey.String(Name),
		)),
	)
	otel.SetTracerProvider(tp)

	metricSeconds := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "server",
		Subsystem: "requests",
		Name:      "duration_sec",
		Help:      "server requests duration(sec).",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.250, 0.5, 1},
	}, []string{"kind", "operation"})

	metricRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "client",
		Subsystem: "requests",
		Name:      "code_total",
		Help:      "The total number of processed requests",
	}, []string{"kind", "operation", "code", "reason"})

	prometheus.MustRegister(metricSeconds, metricRequests)

	app, cleanup, err := wireApp(bc.Server, bc.Registry, bc.Data, logger, tp, metricSeconds, metricRequests)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
