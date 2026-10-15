package main

import (
	"context"
	"encoding/json"
	"fmt"
	"gameServer/pkg/config"
	"gameServer/pkg/logger/log2"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ServiceInfo 服务节点信息
type ServiceInfo struct {
	Id        int               `json:"id"`         // 服务名 使用唯一id
	GroupId   int               `json:"groupId"`    //组id
	Version   int               `json:"version"`    // 版本
	Address   string            `json:"address"`    // 地址，如 tcp@192.168.1.10:8972
	Network   string            `json:"network"`    // 网络类型 tcp/udp
	IP        string            `json:"ip"`         // IP地址
	Port      string            `json:"port"`       // 端口
	Metadata  map[string]string `json:"metadata"`   // 扩展元数据
	UpdatedAt time.Time         `json:"updated_at"` // 更新时间
}

// ServiceGroup 按服务类型分组
type ServiceGroup struct {
	ServicePath string        `json:"service_path"` // etcd 中的路径，如 Gate/node/Forward
	Instances   []ServiceInfo `json:"instances"`    // 该服务的所有实例
	Count       int           `json:"count"`        // 实例数量
}

// DiscoveryService 服务发现服务（无RPC功能）
type DiscoveryService struct {
	etcdClient *clientv3.Client
	basePath   string        // 根路径，如 "node"
	ttl        time.Duration // 缓存TTL

	mu        sync.RWMutex
	cache     map[string]ServiceGroup // 缓存的服务信息
	cacheTime time.Time               // 缓存更新时间
}

func (h *HTTPServer) Start() error {
	mux := http.NewServeMux()

	// GET /services/{name} - 获取指定服务
	mux.HandleFunc("/service/", h.handleGetService)

	// GET /health - 健康检查
	mux.HandleFunc("/health", h.handleHealth)

	log2.Get().Info("HTTP服务启动，监听: %s", zap.String("addr", h.addr))
	return http.ListenAndServe(h.addr, mux)
}

// NewDiscoveryService 创建服务发现实例
func NewDiscoveryService(etcdEndpoints []string, basePath string) (*DiscoveryService, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdEndpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("连接etcd失败: %w", err)
	}

	ds := &DiscoveryService{
		etcdClient: cli,
		basePath:   strings.TrimSuffix(basePath, "/"),
		ttl:        10 * time.Second,
		cache:      make(map[string]ServiceGroup),
	}

	// 启动后台刷新
	go ds.backgroundRefresh()

	return ds, nil
}

// ==================== 核心查询方法 ====================

// GetAllServices 获取所有服务信息
func (d *DiscoveryService) GetAllServices(ctx context.Context) ([]ServiceGroup, error) {
	// 先查缓存
	d.mu.RLock()
	if time.Since(d.cacheTime) < d.ttl && len(d.cache) > 0 {
		result := make([]ServiceGroup, 0, len(d.cache))
		for _, v := range d.cache {
			result = append(result, v)
		}
		d.mu.RUnlock()
		return result, nil
	}
	d.mu.RUnlock()

	// 缓存失效，从etcd查询
	return d.fetchFromEtcd(ctx)
}

// ==================== 内部方法 ====================

// fetchFromEtcd 从etcd拉取所有服务
func (d *DiscoveryService) fetchFromEtcd(ctx context.Context) ([]ServiceGroup, error) {
	resp, err := d.etcdClient.Get(ctx, d.basePath, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	// 按服务路径分组
	groups := make(map[string]*ServiceGroup)

	for _, kv := range resp.Kvs {
		key := string(kv.Key)     //集群信息
		value := string(kv.Value) //元数据

		// 解析路径: mash/Forward/service -> servicePath=mash
		parts := strings.Split(key, "/")
		if len(parts) < 2 {
			continue
		}
		//servicePath := parts[1]
		if parts[1] != "Gate" {
			continue
		}
		gameName := parts[0]
		if _, ok := groups[gameName]; !ok {
			groups[gameName] = &ServiceGroup{
				ServicePath: gameName,
				Instances:   make([]ServiceInfo, 0),
			}
		}

		info := d.parseServiceInfo(key, value)
		groups[gameName].Instances = append(groups[gameName].Instances, info)
	}

	// 更新缓存
	d.mu.Lock()
	d.cache = make(map[string]ServiceGroup)
	result := make([]ServiceGroup, 0, len(groups))

	for path, group := range groups {
		group.Count = len(group.Instances)
		d.cache[path] = *group
		result = append(result, *group)
	}
	d.cacheTime = time.Now()
	d.mu.Unlock()

	return result, nil
}

// parseServiceInfo 解析etcd中的服务信息
func (d *DiscoveryService) parseServiceInfo(key, value string) ServiceInfo {
	var (
		id      = 0
		groupId = 0
		version = 0
		addr    = ""
		port    = ""
	)

	gouldData := strings.Split(value, "&")
	for _, data := range gouldData {
		if data == "" {
			continue
		}
		fields := strings.SplitN(data, "=", 2)
		if len(fields) != 2 {
			continue
		}
		switch fields[0] {
		case "id":
			idV, err := strconv.ParseInt(fields[1], 10, 32)
			if err != nil {
				continue
			}
			id = int(idV)
		case "groupId":
			groupIdV, err := strconv.ParseInt(fields[1], 10, 32)
			if err != nil {
				continue
			}
			groupId = int(groupIdV)
		case "version":
			versionV, err := strconv.ParseInt(fields[1], 10, 32)
			if err != nil {
				continue
			}
			version = int(versionV)
		case "port":
			port = fields[1]
		default:
			continue
		}
	}

	network, ip := "", ""
	if atIdx := strings.Index(key, "@"); atIdx != -1 {
		network = key[:atIdx]
		addr = key[atIdx+1:]
		if colonIdx := strings.LastIndex(addr, ":"); colonIdx != -1 {
			ip = addr[:colonIdx]
			//port = addr[colonIdx+1:]
		}
	}

	return ServiceInfo{
		Id:        id,
		GroupId:   groupId,
		Version:   version,
		Address:   addr,
		Network:   network,
		IP:        ip,
		Port:      port,
		Metadata:  make(map[string]string),
		UpdatedAt: time.Now(),
	}
}

// backgroundRefresh 后台定时刷新缓存
func (d *DiscoveryService) backgroundRefresh() {
	ticker := time.NewTicker(d.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := d.fetchFromEtcd(ctx)
		cancel()

		if err != nil {
			log2.Get().Warn("刷新服务缓存失败", zap.Any("err", err))
		}
	}
}

// Close 关闭服务
func (d *DiscoveryService) Close() error {
	return d.etcdClient.Close()
}

// ==================== HTTP API 暴露 ====================

type HTTPServer struct {
	discovery *DiscoveryService
	addr      string
}

func NewHTTPServer(discovery *DiscoveryService, addr string) *HTTPServer {
	return &HTTPServer{
		discovery: discovery,
		addr:      addr,
	}
}

func (h *HTTPServer) handleGetService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services, err := h.discovery.GetAllServices(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 提取服务名 /service/Forward
	gameName := strings.TrimPrefix(r.URL.Path, "/service/")
	if gameName == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    404,
			"addr":    "",
			"version": 0,
		})
		return
	}

	for _, info := range services {
		if info.ServicePath == gameName {
			MaxVersion := 0
			ip := ""
			port := ""
			code := 0
			// 先找出最大版本号，再从同版本的实例中随机选一个做负载均衡
			candidates := make([]ServiceInfo, 0, len(info.Instances))
			first := true
			for _, Instance := range info.Instances {
				if first || Instance.Version > MaxVersion {
					MaxVersion = Instance.Version
					candidates = candidates[:0]
					candidates = append(candidates, Instance)
					first = false
				} else if Instance.Version == MaxVersion {
					candidates = append(candidates, Instance)
				}
			}
			if len(candidates) > 0 {
				chosen := candidates[rand.IntN(len(candidates))]
				ip = chosen.IP
				port = chosen.Port
			}
			if ip == "" || port == "" {
				code = 500
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    code,
				"addr":    ip + ":" + port,
				"version": MaxVersion,
			})
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    404,
		"addr":    "",
		"version": 0,
	})
}

func (h *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Unix(),
	})
}

// ==================== 使用示例 ====================

func main() {
	// 创建服务发现（连接etcd）
	//fmt.Sprintf("%s", s.config.BasePath)
	configPath := "./config/" // ./config/
	logPath := "./logs/"      // ./logs/
	nodeName := "serverlist-1"
	log2.Init(log2.Config{Level: zapcore.DebugLevel, LogDir: logPath, IsDocker: false})

	if err := config.Init(nodeName, "serverlist", configPath, "serverlist", "toml"); err != nil {
		panic(err)
	}
	c := config.Get()

	etcdEndpoints := c.EtcdAddress()
	discovery, err := NewDiscoveryService(etcdEndpoints, c.EtcdPrefix())
	if err != nil {
		panic(err)
	}
	defer discovery.Close()

	// jpServers.9skygaming.com
	// 启动HTTP API服务（无RPC）
	httpServer := NewHTTPServer(discovery, c.Address())
	if err = httpServer.Start(); err != nil {
		panic(err)
	}
}
