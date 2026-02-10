package config

import "strings"

// Config 服务配置表
//
// 配置表分为三部分
// - common: 所有服务都使用到的通用配置，在 [common] 中的配置
// - service: 一个服务内通用的部分，例如在 [gate] 中的配置
// - node: 每一个服务单独使用的部分，例如在 [gate-1], [gate-2] 中的配置
type Config struct {
	common      *viper.Viper
	service     *viper.Viper
	node        *viper.Viper
	serviceName string

	// isTest 当前是否是测试用例
	isTest bool
}

// globalConfig ..
var globalConfig *Config

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}

// IsTest 是否处于测试用例模式
func (c *Config) IsTest() bool {
	return c.isTest
}

// ID 本专服 id
func (c *Config) ID() int {
	return c.common.GetInt("id")
}

// Name 本专服名称
func (c *Config) Name() string {
	return c.common.GetString("name")
}

// Secret 本专服秘钥
func (c *Config) Secret() string {
	return c.common.GetString("secret")
}

// IsDevelop 是否为开发模式
func (c *Config) IsDevelop() bool {
	if c != nil && c.common != nil {
		return c.common.GetBool("isdevelop")
	}

	return false
}

// CachePrefix 缓存公共前缀
func (c *Config) CachePrefix() string {
	return c.common.GetString("cacheprefix")
}

// CacheAddress 缓存地址
func (c *Config) CacheAddress() string {
	return c.common.GetString("cacheaddress")
}

// CacheDB 缓存数据库，redis 0-15
func (c *Config) CacheDB() int {
	return c.common.GetInt("cachedb")
}

// CachePassword 缓存密码
func (c *Config) CachePassword() string {
	return c.common.GetString("cachepassword")
}

// CacheMinIdle 缓存最小空闲连接数
func (c *Config) CacheMinIdle() int {
	return c.common.GetInt("cacheminidle")
}

// CacheMaxActive 最大激活连接数
func (c *Config) CacheMaxActive() int {
	return c.common.GetInt("cachemaxactive")
}

// JaegerAddress jaeger agent 地址，udp
func (c *Config) JaegerAddress() string {
	return c.common.GetString("jaegeraddress")
}

// EtcdAddress etcd 地址
func (c *Config) EtcdAddress() []string {
	l := c.common.GetString("etcdaddress")
	ss := strings.Split(l, ";")
	return ss
}

// EtcdPrefix etcd 前缀，防止多项目共享 etcd 时混淆
func (c *Config) EtcdPrefix() string {
	v := c.common.GetString("etcdprefix")
	if len(v) == 0 {
		return c.Name()
	}
	return v
}

// RPCHeart rpc 心跳间隔
func (c *Config) RPCHeart() int {
	return c.common.GetInt("rpcheart")
}

// ServiceName 服务名称，如 gate, game
func (c *Config) ServiceName() string {
	return c.serviceName
}

// IsPublic 当前不在任何节点内
func (c *Config) IsPublic() bool {
	return c.node == nil || c.service == nil
}

// NodeID 节点 id
func (c *Config) NodeID() uint32 {
	return c.node.GetUint32("id")
}

// NodeName 节点名称，如 gate-1, gate-2, game-1
func (c *Config) NodeName() string {
	return c.node.GetString("name")
}

// NodeVersion 节点版本号
func (c *Config) NodeVersion() uint32 {
	return c.node.GetUint32("version")
}

// PProfAddress pprof 地址
func (c *Config) PProfAddress() string {
	return c.node.GetString("pprofaddr")
}

// GameConfigFilePath 游戏配置文件路径
func (c *Config) GameConfigFilePath() string {
	return c.common.GetString("gameconfigpath")
}
