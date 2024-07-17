package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-redis/redis/extra/redisotel"
	"github.com/go-redis/redis/v8"
	"github.com/maidol/kratos-layout/ent"
	"github.com/maidol/kratos-layout/internal/conf"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/wire"
	hellov1 "github.com/maidol/kratos-layout/api/helloworld/v1"
	clientv3 "go.etcd.io/etcd/client/v3"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	// init mysql driver
	_ "github.com/go-sql-driver/mysql"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewGreeterRepo, NewDiscovery, NewHelloServiceClient)

// Data .
type Data struct {
	// TODO wrapped database client
	db  *ent.Client
	rdb *redis.Client

	log *log.Helper

	hc hellov1.GreeterClient
}

// NewData .
func NewData(c *conf.Data, logger log.Logger, hc hellov1.GreeterClient) (*Data, func(), error) {
	l := log.NewHelper(logger)
	drv, err := sql.Open(
		c.Database.Driver,
		c.Database.Source,
	)
	sqlDrv := dialect.DebugWithContext(drv, func(ctx context.Context, i ...interface{}) {
		l.WithContext(ctx).Info(i...)
		tracer := otel.Tracer("ent.")
		kind := trace.SpanKindServer
		_, span := tracer.Start(ctx,
			"Query",
			trace.WithAttributes(
				attribute.String("sql", fmt.Sprint(i...)),
			),
			trace.WithSpanKind(kind),
		)
		span.End()
	})
	client := ent.NewClient(ent.Driver(sqlDrv))
	if err != nil {
		l.Errorf("failed opening connection to sqlite: %v", err)
		return nil, nil, err
	}
	// Run the auto migration tool.
	if err := client.Schema.Create(context.Background()); err != nil {
		l.Errorf("failed creating schema resources: %v", err)
		return nil, nil, err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         c.Redis.Addr,
		Password:     c.Redis.Password,
		DB:           int(c.Redis.Db),
		DialTimeout:  c.Redis.DialTimeout.AsDuration(),
		WriteTimeout: c.Redis.WriteTimeout.AsDuration(),
		ReadTimeout:  c.Redis.ReadTimeout.AsDuration(),
	})
	rdb.AddHook(redisotel.TracingHook{})
	d := &Data{
		log: log.NewHelper(log.With(logger, "module", "data")),
		db:  client,
		rdb: rdb,
		hc:  hc,
	}

	cleanup := func() {
		l.Info("message", "closing the data resources")
		if err := d.db.Close(); err != nil {
			l.Error(err)
		}
		if err := d.rdb.Close(); err != nil {
			l.Error(err)
		}
	}
	return d, cleanup, nil
}

func NewDiscovery(conf *conf.Registry) registry.Discovery {
	// new etcd client
	client, err := clientv3.New(clientv3.Config{
		Endpoints: conf.Etcd.Address,
		Username:  conf.Etcd.Username,
		Password:  conf.Etcd.Password,
	})
	if err != nil {
		panic(err)
	}
	// new dis with etcd client
	dis := etcd.New(client)
	return dis
}

func NewHelloServiceClient(r registry.Discovery, tp *tracesdk.TracerProvider) hellov1.GreeterClient {
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint("discovery:///hello1"),
		grpc.WithDiscovery(r),
		grpc.WithMiddleware(
			tracing.Client(tracing.WithTracerProvider(tp)),
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	c := hellov1.NewGreeterClient(conn)
	return c
}
