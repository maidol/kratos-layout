package server

import (
	v1 "github.com/maidol/kratos-layout/api/helloworld/v1"
	"github.com/maidol/kratos-layout/internal/conf"
	"github.com/maidol/kratos-layout/internal/service"
	"github.com/prometheus/client_golang/prometheus"

	prom "github.com/go-kratos/kratos/contrib/metrics/prometheus/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metadata"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, greeter *service.GreeterService, logger log.Logger, tp *tracesdk.TracerProvider, metricSeconds *prometheus.HistogramVec, metricRequests *prometheus.CounterVec) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			metrics.Server(
				metrics.WithSeconds(prom.NewHistogram(metricSeconds)),
				metrics.WithRequests(prom.NewCounter(metricRequests)),
			),
			tracing.Server(tracing.WithTracerProvider(tp)),
			logging.Server(logger),
			metadata.Server(),
			validate.Validator(),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	v1.RegisterGreeterHTTPServer(srv, greeter)

	srv.Handle("/metrics", promhttp.Handler())
	return srv
}
