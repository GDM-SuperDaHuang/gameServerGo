package ssdb

import (
	"errors"

	"github.com/seefan/gossdb/v2"
	"github.com/seefan/gossdb/v2/conf"
	"github.com/seefan/gossdb/v2/pool"
)

//Host string //ssdb的ip或主机名
//Port int //ssdb的端口
//GetClientTimeout int //获取连接超时时间，单位为秒。默认值: 5
//ReadWriteTimeout int //连接读写超时时间，单位为秒。默认值: 60
//WriteTimeout int //连接写超时时间，单位为秒，如果不设置与ReadWriteTimeout会保持一致。默认值: 0
//ReadTimeout int //连接读超时时间，单位为秒，如果不设置与ReadWriteTimeout会保持一致。默认值: 0
//MaxPoolSize int //最大连接个数。默认值: 100，PoolSize的整数倍，不足的话自动补足。
//MinPoolSize int //最小连接个数。默认值: 20，PoolSize的整数倍，不足的话自动补足。
//PoolSize int //连接池块的连接数。默认值: 20，连接池扩展和收缩时，以此值步进，可根据机器性能调整。
//MaxWaitSize int //最大等待数目，当连接池满后，新建连接将等待池中连接释放后才可以继续，本值限制最大等待的数量，超过本值后将抛出异常。默认值: 1000
//HealthSecond int //连接池内缓存的连接状态检查时间隔，单位为秒。默认值: 30
//Password string //连接的密钥
//WriteBufferSize int //连接写缓冲，默认为8k，单位为kb
//ReadBufferSize int //连接读缓冲，默认为8k，单位为kb
//RetryEnabled bool //是否启用重试，设置为true时，如果请求失败会再重试一次。默认值: false
//ConnectTimeout int //创建连接的超时时间，单位为秒。默认值: 5
//AutoClose bool //是否自动回收连接，如果开启后，获取的连接在使用后立即会被回收，所以不要重复使用。
//Encoding bool //是否开启自动序列化

var globalSSDBClient *pool.Client

func New(cfg *conf.Config) error {
	err := gossdb.Start(cfg)
	if err != nil {
		return err
	}
	return nil
}

func Init(cfg *conf.Config) error {
	err := New(cfg)
	if err != nil {
		return err
	}
	// 测试链接
	globalSSDBClient = gossdb.Client()
	if !globalSSDBClient.Ping() {
		return errors.New("invalid ssdb ping false ")
	}
	return nil
}
func GetClient() *pool.Client {
	return globalSSDBClient
}

func Close() {
	gossdb.Shutdown()
}
