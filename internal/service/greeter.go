package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/metadata"
	"github.com/go-kratos/kratos/v2/transport"
	v1 "github.com/maidol/kratos-layout/api/helloworld/v1"
	"github.com/maidol/kratos-layout/internal/biz"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GreeterService is a greeter service.
type GreeterService struct {
	v1.UnimplementedGreeterServer
	log *log.Helper
	uc  *biz.GreeterUsecase
}

// NewGreeterService new a greeter service.
func NewGreeterService(uc *biz.GreeterUsecase, logger log.Logger) *GreeterService {
	return &GreeterService{uc: uc, log: log.NewHelper(logger)}
}

// SayHello implements helloworld.GreeterServer.
func (s *GreeterService) SayHello(ctx context.Context, in *v1.HelloRequest) (*v1.HelloReply, error) {
	s.log.WithContext(ctx).Info("greeterservice sayhello")
	if tr, ok := transport.FromServerContext(ctx); ok {
		tracer := otel.Tracer("GreeterService.")
		kind := trace.SpanKindServer
		var span trace.Span
		ctx, span := tracer.Start(
			ctx,
			tr.Operation(),
			trace.WithAttributes(
				attribute.String("method", "SayHello"),
			),
			trace.WithSpanKind(kind),
		)
		span.SetAttributes(attribute.String("tr.Endpoint", tr.Endpoint()))
		s.log.WithContext(ctx).Info("---start trace--- greeterservice sayhello")
		defer func() {
			s.log.WithContext(ctx).Info("---end trace--- greeterservice sayhello")
			span.End()
		}()
	}
	if md, ok := metadata.FromServerContext(ctx); ok {
		extra := md.Get("x-md-global-extra")
		s.log.WithContext(ctx).Info("x-md-global-extra: ", extra)
	}
	g, err := s.uc.CreateGreeter(ctx, &biz.Greeter{Hello: in.Name})
	if err != nil {
		return nil, err
	}
	return &v1.HelloReply{Message: "Hello " + g.Hello}, nil
}
