package client

import (
	"Server/service/common"
	"Server/service/rpc"
	"context"
	"fmt"
	"strconv"
	"time"

	etcdClient "github.com/rpcxio/rpcx-etcd/client"
	xclient "github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/protocol"
	"github.com/smallnest/rpcx/share"
)

// New 创建客户端
func NewClient(config *Config) (rpc.ClientInterface, error) {
	c := &Client{
		config: config,
	}

	// 创建服务发现
	discovery, err := etcdClient.NewEtcdV3Discovery(
		config.BasePath,    //基本路径
		config.ServicePath, //具体分支路径的服务，相同则是集群
		config.EtcdEndpoints,
		true, //监听节点上下线变化
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd service discovery: %v", err)
	}
	c.discovery = discovery

	// 创建客户端选项
	option := xclient.DefaultOption
	option.Heartbeat = true
	option.HeartbeatInterval = config.HeartbeatInterval
	option.ConnectTimeout = time.Second * 3
	option.SerializeType = protocol.MsgPack
	option.CompressType = protocol.None
	option.BackupLatency = time.Second // 设置故障转移延迟
	option.Retries = 3                 // 设置重试次数

	// 创建客户端
	var (
		poolSize   = 8
		failMode   = xclient.Failover   // 使用故障转移模式,挑选好的服务节点
		selectMode = xclient.RoundRobin // 使用轮询负载均衡
	)
	if config.FailMode > 0 {
		failMode = config.FailMode
	}
	if config.SelectMode > 0 {
		selectMode = config.SelectMode
	}
	if config.PoolSize > 0 {
		poolSize = config.PoolSize
	}

	pool := xclient.NewXClientPool(
		poolSize,
		config.ServicePath, //必须和保持和etcd一致
		failMode,
		selectMode,
		discovery,
		option,
	)

	if config.Selector != nil {
		for range poolSize {
			pool.Get().SetSelector(config.Selector)
		}
	}

	c.pool = pool

	return c, nil
}

// Wrap 指定 id 调用
func (c *Client) Wrap(id, versionMin, versionMax uint32) rpc.WrapClient {
	wc := wrapClientPool.Get()
	wc.reset(c, id, versionMin, versionMax)
	return wc
}

// Call 同步调用
func (c *Client) Call(ctx context.Context, serviceMethod string, args any, reply any) error {
	return c.pool.Get().Call(ctx, serviceMethod, args, reply)
}

// Go 异步调用
func (c *Client) Go(ctx context.Context, serviceMethod string, args any, reply any, done chan *xclient.Call) (*xclient.Call, error) {
	return c.pool.Get().Go(ctx, serviceMethod, args, reply, done)
}

// Close 关闭客户端
func (c *Client) Close() error {
	if c.pool != nil {
		c.pool.Close()
	}
	return nil
}

// Name rpc 服务提供者名称
func (c *Client) Name() string {
	return c.config.ServiceName
}

//// WrapClient 指定额外参数调用
//type WrapClient interface {
//	// Call 同步调用
//	Call(ctx context.Context, serviceMethod string, args any, reply any) error
//
//	// Go 异步调用
//	Go(ctx context.Context, serviceMethod string, args any, reply any, done chan *xclient.Call) (*xclient.Call, error)
//}

type wrapClient struct {
	c          *Client
	id         uint32
	versionMin uint32
	versionMax uint32
}

// Reset 重置
func (w *wrapClient) Reset() {
	w.c = nil
	w.id = 0
	w.versionMin = 0
	w.versionMax = 0
}

var wrapClientPool = common.NewPool(func() *wrapClient {
	return &wrapClient{}
})

// Call 同步调用
func (w *wrapClient) Call(ctx context.Context, serviceMethod string, args any, reply any) error {
	defer wrapClientPool.Put(w)
	return w.c.Call(w.buildCtx(ctx), serviceMethod, args, reply)
}

// Go 异步调用
func (w *wrapClient) Go(ctx context.Context, serviceMethod string, args any, reply any, done chan *xclient.Call) (*xclient.Call, error) {
	defer wrapClientPool.Put(w)
	return w.c.Go(w.buildCtx(ctx), serviceMethod, args, reply, done)
}

func (w *wrapClient) reset(c *Client, id, versionMin, versionMax uint32) {
	w.c = c
	w.id = id
	w.versionMin = versionMin
	w.versionMax = versionMax
}

func (w *wrapClient) buildCtx(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, share.ResMetaDataKey, map[string]string{
		"id":         strconv.Itoa(int(w.id)),
		"versionMin": strconv.Itoa(int(w.versionMin)),
		"versionMax": strconv.Itoa(int(w.versionMax)),
	})

	return ctx
}
