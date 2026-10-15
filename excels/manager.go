package excels

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	cfg "gameServer/excels/gen"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// Manager luban 配表管理器：启动加载 + 手动 Reload + 文件监听热更。
//
// 并发安全：内部用 atomic.Pointer 持有 *cfg.Tables，
// 读无锁，热更先解析成功再原子替换，失败则保留老数据。
type Manager struct {
	dataDir  string
	debounce time.Duration
	logger   *zap.Logger

	tables atomic.Pointer[cfg.Tables]

	reloadMu sync.Mutex // 串行化 Reload，避免并发重载

	cbMu      sync.RWMutex
	callbacks []func(old, newTables *cfg.Tables)

	watcher   *fsnotify.Watcher
	closeOnce sync.Once
	closed    chan struct{}

	debounceMu    sync.Mutex
	debounceTimer *time.Timer
}

// Option Manager 可选项
type Option func(*Manager)

// WithLogger 热更成功/失败时打日志，nil 则静默
func WithLogger(l *zap.Logger) Option {
	return func(m *Manager) { m.logger = l }
}

// WithDebounce 监听到文件变更后延迟重载，合并编辑器连续写入，默认 200ms
func WithDebounce(d time.Duration) Option {
	return func(m *Manager) {
		if d > 0 {
			m.debounce = d
		}
	}
}

// New 加载 dataDir 下的 *.json 并构建 Manager，不启动监听。
// dataDir 如 excels/data，内含 item_tb_item.json 等 luban 产物。
func New(dataDir string, opts ...Option) (*Manager, error) {
	m := &Manager{
		dataDir:  dataDir,
		debounce: 200 * time.Millisecond,
		closed:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	tables, err := cfg.NewTables(m.loader())
	if err != nil {
		return nil, fmt.Errorf("load tables from %s: %w", dataDir, err)
	}
	m.tables.Store(tables)
	return m, nil
}

func (m *Manager) loader() cfg.JsonLoader {
	return func(name string) ([]map[string]interface{}, error) {
		buf, err := os.ReadFile(filepath.Join(m.dataDir, name+".json"))
		if err != nil {
			return nil, err
		}
		var data []map[string]interface{}
		if err := json.Unmarshal(buf, &data); err != nil {
			return nil, err
		}
		return data, nil
	}
}

// Get 无锁获取当前全量配表，热更后读到的是新指针
func (m *Manager) Get() *cfg.Tables {
	return m.tables.Load()
}

// Reload 手动重载：解析成功才原子替换，失败返回 err 且老数据不受影响
func (m *Manager) Reload() error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()

	old := m.tables.Load()
	newTables, err := cfg.NewTables(m.loader())
	if err != nil {
		m.log(false, "excels reload failed", err)
		return err
	}
	m.tables.Store(newTables)
	m.log(true, "excels reload ok", nil)
	m.emit(old, newTables)
	return nil
}

// OnReload 注册热更回调（Reload/Watch 触发成功后调用），回调内只读 newTables，不要阻塞太久
func (m *Manager) OnReload(cb func(old, newTables *cfg.Tables)) {
	m.cbMu.Lock()
	defer m.cbMu.Unlock()
	m.callbacks = append(m.callbacks, cb)
}

func (m *Manager) emit(old, newTables *cfg.Tables) {
	m.cbMu.RLock()
	defer m.cbMu.RUnlock()
	for _, cb := range m.callbacks {
		cb(old, newTables)
	}
}

// Watch 启动文件监听：dataDir 下 *.json 变更去抖后自动 Reload，非阻塞。
// 调用 Close 停止。重复调用返回 error。
func (m *Manager) Watch() error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	if m.watcher != nil {
		return fmt.Errorf("already watching %s", m.dataDir)
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(m.dataDir); err != nil {
		_ = w.Close()
		return err
	}
	m.watcher = w
	go m.loop(w)
	return nil
}

// Close 停止监听，幂等
func (m *Manager) Close() error {
	var err error
	m.closeOnce.Do(func() {
		close(m.closed)
		m.debounceMu.Lock()
		if m.debounceTimer != nil {
			m.debounceTimer.Stop()
		}
		m.debounceMu.Unlock()
		if m.watcher != nil {
			err = m.watcher.Close()
		}
	})
	return err
}

func (m *Manager) loop(w *fsnotify.Watcher) {
	for {
		select {
		case <-m.closed:
			return
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			m.log(false, "excels watch error", err)
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			// 只关心 json 的写入/创建/重命名（编辑器多为 rename 覆盖），Chmod 忽略
			if !isJSONEvent(ev) {
				continue
			}
			m.scheduleReload()
		}
	}
}

func isJSONEvent(ev fsnotify.Event) bool {
	if !strings.HasSuffix(strings.ToLower(ev.Name), ".json") {
		return false
	}
	return ev.Has(fsnotify.Write) || ev.Has(fsnotify.Create) || ev.Has(fsnotify.Rename)
}

func (m *Manager) scheduleReload() {
	m.debounceMu.Lock()
	defer m.debounceMu.Unlock()
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
	}
	m.debounceTimer = time.AfterFunc(m.debounce, func() {
		// 失败也只记日志，保留老数据；成功后 emit 回调
		_ = m.Reload()
	})
}

func (m *Manager) log(ok bool, msg string, err error) {
	if m.logger == nil {
		return
	}
	if ok {
		m.logger.Info(msg, zap.String("dir", m.dataDir))
		return
	}
	m.logger.Error(msg, zap.String("dir", m.dataDir), zap.Error(err))
}

// ---- 便捷访问（读最新快照） ----

// Item 查道具表，miss 返回 nil
func (m *Manager) Item(id int32) *cfg.Item {
	t := m.tables.Load()
	if t == nil || t.Tb_Item == nil {
		return nil
	}
	return t.Tb_Item.Get(id)
}

// ---- 包级默认实例：单服进程用一份即可 ----

var (
	defaultManager *Manager
	defaultMu      sync.RWMutex
)

// Init 初始化全局实例（启动时调一次）。之后用 Default()/Get() 读表。
func Init(dataDir string, opts ...Option) error {
	m, err := New(dataDir, opts...)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultManager != nil {
		_ = defaultManager.Close()
	}
	defaultManager = m
	return nil
}

// Default 返回全局实例，未 Init 则为 nil
func Default() *Manager {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultManager
}

// Get 全局配表快照，未 Init 返回 nil，方便业务层直接 Get().Tb_Item.Get(id)
func Get() *cfg.Tables {
	if m := Default(); m != nil {
		return m.Get()
	}
	return nil
}
