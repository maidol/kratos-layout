package server

import (
	"github.com/go-kratos/kratos-layout/internal/conf"
	"github.com/go-kratos/kratos/contrib/registry/etcd/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/registry"
	"github.com/google/wire"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(NewGRPCServer, NewHTTPServer, NewRegistrar)

func NewRegistrar(conf *conf.Registry, logger log.Logger) registry.Registrar {
	// new etcd client
	client, err := clientv3.New(clientv3.Config{
		Endpoints: conf.Etcd.Address,
		Username:  conf.Etcd.Username,
		Password:  conf.Etcd.Username,
	})
	if err != nil {
		panic(err)
	}
	// new reg with etcd client
	r := etcd.New(client)
	return r
}
