package data

import (
	"context"
	"fmt"

	v1 "github.com/maidol/kratos-layout/api/helloworld/v1"
	"github.com/maidol/kratos-layout/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type greeterRepo struct {
	data *Data
	log  *log.Helper
}

// NewGreeterRepo .
func NewGreeterRepo(data *Data, logger log.Logger) biz.GreeterRepo {
	return &greeterRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *greeterRepo) Save(ctx context.Context, g *biz.Greeter) (*biz.Greeter, error) {
	return g, nil
}

func (r *greeterRepo) Update(ctx context.Context, g *biz.Greeter) (*biz.Greeter, error) {
	return g, nil
}

func (r *greeterRepo) FindByID(ctx context.Context, id int64) (*biz.Greeter, error) {
	u, err := r.data.db.User.Get(ctx, int(id))
	if err != nil {
		r.log.WithContext(ctx).Error(err)
		return nil, err
	}
	r.log.WithContext(ctx).Info(u)
	_, err = r.data.rdb.Set(ctx, fmt.Sprintf("user:%d", u.ID), u.Name, 0).Result()
	if err != nil {
		r.log.WithContext(ctx).Error(err)
		return nil, err
	}
	rpl, err := r.data.hc.SayHello(ctx, &v1.HelloRequest{Name: "call-hello"})
	if err != nil {
		r.log.WithContext(ctx).Error("hc.SayHello: ", err)
		// return nil, err
	}
	r.log.WithContext(ctx).Info(rpl)

	return nil, nil
}

func (r *greeterRepo) ListByHello(context.Context, string) ([]*biz.Greeter, error) {
	return nil, nil
}

func (r *greeterRepo) ListAll(context.Context) ([]*biz.Greeter, error) {
	return nil, nil
}
