package bytes

import (
	"bytes"
	"fmt"
	"gameServer/pkg/logger"
	"gameServer/pkg/utils"
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
)

// from https://github.com/lxzan/gws.git

// BufferPool buff池
type BufferPool struct {
	begin  int
	end    int
	shards map[int]*sync.Pool
}

// NewBufferPool 创建一个内存池
// creates a memory pool
//
// left 和 right 表示内存池的区间范围，它们将被转换为 2 的 n 次幂
// left and right indicate the interval range of the memory pool, they will be transformed into pow(2, n)
//
// 小于 left 的情况下，Get 方法将返回至少 left 字节的缓冲区；大于 right 的情况下，Put 方法不会回收缓冲区
// Below left, the Get method will return at least left bytes; above right, the Put method will not reclaim the buffer
func NewBufferPool(left, right int) *BufferPool {
	begin, end := binaryCeil(left), binaryCeil(right)
	p := &BufferPool{
		begin:  begin,
		end:    end,
		shards: map[int]*sync.Pool{},
	}
	for i := begin; i <= end; i *= 2 {
		capacity := i
		p.shards[i] = &sync.Pool{
			New: func() any { return bytes.NewBuffer(make([]byte, 0, capacity)) },
		}
	}
	return p
}

// Release 将缓冲区放回到内存池
// returns the buffer to the memory pool
func (p *BufferPool) Release(b *bytes.Buffer) {
	if b != nil {
		if pool, ok := p.shards[b.Cap()]; ok {
			pool.Put(b)
		}
	}
}

// Buffer 从内存池中获取一个至少 n 字节的缓冲区
// fetches a buffer from the memory pool, of at least n bytes
func (p *BufferPool) Buffer(n int) *bytes.Buffer {
	size := utils.Max(binaryCeil(n), p.begin)
	if pool, ok := p.shards[size]; ok {
		b := pool.Get().(*bytes.Buffer)
		if b.Cap() < size {
			b.Grow(size)
		}
		b.Reset()
		return b
	}
	return bytes.NewBuffer(make([]byte, 0, n))
}

// binaryCeil 将给定的 uint32 值向上取整到最近的 2 的幂
// rounds up the given uint32 value to the nearest power of 2
func binaryCeil(v int) int {
	return utils.F2(v)
}

// Reset ..
type Reset interface {
	Reset()
}

// implementsReset 检查类型是否实现了 Reset 接口
func implementsReset(t reflect.Type) bool {
	// 获取 Reset 接口类型
	resetType := reflect.TypeOf((*Reset)(nil)).Elem()

	// 检查类型是否实现了 Reset 接口
	return t.Implements(resetType) || reflect.PointerTo(t).Implements(resetType)
}

// NewPool 创建一个新的泛型内存池
// creates a new generic pool
func NewPool[T Reset](f func() T) *Pool[T] {
	// 获取类型名称
	t := reflect.TypeOf((*T)(nil)).Elem()
	var name string
	if t.Kind() == reflect.Ptr {
		name = fmt.Sprintf("%s.%s", t.Elem().PkgPath(), t.Elem().Name())
	} else {
		name = fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
	}

	// 创建对象池
	pool := &Pool[T]{
		name:     name,
		p:        sync.Pool{New: func() any { return f() }},
		getCount: 0,
		putCount: 0,
		ticker:   time.NewTicker(5 * time.Minute), // 默认每分钟输出一次统计信息
		done:     make(chan bool),
	}

	// 启动定时统计日志
	go func() {
		for {
			select {
			case <-pool.ticker.C:
				pool.LogStats()
			case <-pool.done:
				pool.ticker.Stop()
				return
			}
		}
	}()

	return pool
}

// Pool 泛型内存池
// generic pool
type Pool[T Reset] struct {
	name string
	p    sync.Pool

	getLocations sync.Map // map[string]*int64
	putLocations sync.Map // map[string]*int64

	getCount  uint64
	putCount  uint64
	countLock sync.RWMutex

	ticker *time.Ticker
	done   chan bool
}

// Put 将一个值放入池中
// puts a value into the pool
func (c *Pool[T]) Put(v T) {
	//if config.Get().IsDevelop() {
	//	_, file, line, _ := runtime.Caller(1)
	//	key := fmt.Sprintf("%s:%d", file, line)
	//	count, _ := c.putLocations.LoadOrStore(key, new(int64))
	//	atomic.AddInt64(count.(*int64), 1)
	//}

	c.countLock.Lock()
	c.putCount++
	c.countLock.Unlock()

	c.p.Put(v)
}

func (c *Pool[T]) Get() T {
	//if config.Get().IsDevelop() {
	//	_, file, line, _ := runtime.Caller(1)
	//	key := fmt.Sprintf("%s:%d", file, line)
	//	count, _ := c.getLocations.LoadOrStore(key, new(int64))
	//	atomic.AddInt64(count.(*int64), 1)
	//}

	c.countLock.Lock()
	c.getCount++
	c.countLock.Unlock()

	v := c.p.Get().(T)
	v.Reset()
	return v
}

// GetStats 获取对象池使用统计信息
// returns the usage statistics of the pool
func (c *Pool[T]) GetStats() (getCount, putCount uint64) {
	c.countLock.RLock()
	defer c.countLock.RUnlock()

	return c.getCount, c.putCount
}

// ResetStats 重置对象池使用统计信息
// resets the usage statistics of the pool
func (c *Pool[T]) ResetStats() {
	c.countLock.Lock()
	defer c.countLock.Unlock()

	c.getCount = 0
	c.putCount = 0
}

// TypePools 指定类型内存池
type TypePools struct {
	lock  sync.RWMutex
	pools map[reflect.Type]*sync.Pool
	New   func(t reflect.Type) any

	// 监控计数器
	getCounts map[reflect.Type]uint64 // 每种类型获取对象的次数
	putCounts map[reflect.Type]uint64 // 每种类型释放对象的次数
	countLock sync.RWMutex            // 保护计数器的锁
}

// NewTypePools 创建一个新的 TypePools 实例
func NewTypePools(poolSize int) *TypePools {
	return &TypePools{
		pools:     make(map[reflect.Type]*sync.Pool, poolSize),
		getCounts: make(map[reflect.Type]uint64, poolSize),
		putCounts: make(map[reflect.Type]uint64, poolSize),
	}
}

// Add 新增对象类型
//
// - t 中必须实现 Reset 接口
func (tp *TypePools) Add(t reflect.Type) {
	// 检查类型是否实现了 Reset 接口
	if !implementsReset(t) {
		var name string
		if t.Kind() == reflect.Ptr {
			name = t.Elem().Name()
		} else {
			name = t.Name()
		}
		panic(fmt.Sprintf("type: %s does not implement Reset interface", name))
	}

	p := &sync.Pool{
		New: func() any {
			var v reflect.Value
			if t.Kind() == reflect.Ptr {
				v = reflect.New(t.Elem())
			} else {
				v = reflect.New(t)
			}
			return v.Interface()
		},
	}

	tp.lock.Lock()
	defer tp.lock.Unlock()

	tp.pools[t] = p
}

func (tp *TypePools) Get(t reflect.Type) any {
	tp.lock.RLock()
	pool, found := tp.pools[t]
	tp.lock.RUnlock()

	if !found {
		panic(fmt.Sprintf("type %s has not been registered to the pool via Add method", t.String()))
	}

	v := pool.Get()
	v.(Reset).Reset()

	// 更新获取计数
	tp.countLock.Lock()
	tp.getCounts[t]++
	tp.countLock.Unlock()

	return v
}

// Put ..
func (tp *TypePools) Put(t reflect.Type, x any) {
	tp.lock.RLock()
	pool := tp.pools[t]
	tp.lock.RUnlock()

	pool.Put(x)

	// 更新释放计数
	tp.countLock.Lock()
	tp.putCounts[t]++
	tp.countLock.Unlock()
}

// GetTypeStats 获取指定类型的对象池使用统计信息
// returns the usage statistics of the specified type pool
func (tp *TypePools) GetTypeStats(t reflect.Type) (getCount, putCount uint64) {
	tp.countLock.RLock()
	defer tp.countLock.RUnlock()

	return tp.getCounts[t], tp.putCounts[t]
}

// GetAllStats 获取所有类型的对象池使用统计信息
// returns the usage statistics of all type pools
func (tp *TypePools) GetAllStats() map[reflect.Type]struct{ GetCount, PutCount uint64 } {
	tp.countLock.RLock()
	defer tp.countLock.RUnlock()

	result := make(map[reflect.Type]struct{ GetCount, PutCount uint64 }, len(tp.pools))
	for t := range tp.pools {
		result[t] = struct{ GetCount, PutCount uint64 }{
			GetCount: tp.getCounts[t],
			PutCount: tp.putCounts[t],
		}
	}

	return result
}

// ResetStats 重置所有类型的对象池使用统计信息
// resets the usage statistics of all type pools
func (tp *TypePools) ResetStats() {
	tp.countLock.Lock()
	defer tp.countLock.Unlock()

	for t := range tp.pools {
		tp.getCounts[t] = 0
		tp.putCounts[t] = 0
	}
}

// ResetTypeStats 重置指定类型的对象池使用统计信息
// resets the usage statistics of the specified type pool
func (tp *TypePools) ResetTypeStats(t reflect.Type) {
	tp.countLock.Lock()
	defer tp.countLock.Unlock()

	tp.getCounts[t] = 0
	tp.putCounts[t] = 0
}

// LogStats 输出对象池统计信息到日志
// logs the usage statistics of the pool
func (p *BufferPool) LogStats() {
	log := logger.Get()
	if log == nil {
		return
	}

	if p.begin == p.end {
		log.Info("BufferPool stats",
			zap.String("POOL_STATUS", "OK"),
			zap.Int("pool_size", len(p.shards)),
			zap.Int("begin_size", p.begin),
			zap.Int("end_size", p.end))
	} else {
		log.Warn("BufferPool stats",
			zap.String("POOL_STATUS", "OK"),
			zap.Int("pool_size", len(p.shards)),
			zap.Int("begin_size", p.begin),
			zap.Int("end_size", p.end))
	}
}

// LogStats 输出对象池统计信息到日志
// logs the usage statistics of the pool
func (c *Pool[T]) LogStats() {
	log := logger.Get()
	if log == nil {
		return
	}

	getCount, putCount := c.GetStats()
	logStatus("Pool", c.name, getCount, putCount)

	if getCount != putCount {
		//if config.Get().IsDevelop() { // 开发模式
		if true { // 开发模式
			c.getLocations.Range(func(key, getVal any) bool {
				log.Warn("未释放对象追踪",
					zap.String("name", c.name),
					zap.Int64("获取次数", *getVal.(*int64)),
					zap.String("调用位置", key.(string)),
					zap.String("POOL_STATUS", "OK"),
				)
				return true
			})
			c.putLocations.Range(func(key, putVal any) bool {
				log.Warn("未释放对象追踪",
					zap.String("name", c.name),
					zap.Int64("释放次数", *putVal.(*int64)),
					zap.String("调用位置", key.(string)),
					zap.String("POOL_STATUS", "OK"),
				)
				return true
			})
		}
	}
}

// Close 关闭对象池，停止定时统计
// closes the pool and stops the stats timer
func (c *Pool[T]) Close() {
	if c.done != nil {
		c.done <- true
		close(c.done)
		c.done = nil
	}
}

// LogStats 输出所有类型对象池统计信息到日志
// logs the usage statistics of all type pools
func (tp *TypePools) LogStats() {
	log := logger.Get()
	if log == nil {
		return
	}

	stats := tp.GetAllStats()
	for t, stat := range stats {
		typeName := t.String()

		logStatus("TypePools", typeName, stat.GetCount, stat.PutCount)
	}
}

// StartStatsLogger 启动定时统计日志记录器
// starts a timer to periodically log pool statistics
func StartStatsLogger(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute // 默认5分钟记录一次
	}

	log := logger.Get()
	if log == nil {
		return
	}

	log.Info("Starting pool stats logger", zap.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			// 记录全局对象池统计信息
			if binaryPool != nil {
				binaryPool.LogStats()
			}

			if reflectTypePool != nil {
				reflectTypePool.LogStats()
			}
		}
	}()
}

func logStatus(poolType, typeName string, getCount, putCount uint64) {
	if getCount == putCount {
		logger.Get().Info(poolType,
			zap.String("POOL_STATUS", "OK"),
			zap.String("type", typeName),
			zap.Uint64("get_count", getCount),
			zap.Uint64("put_count", putCount),
			zap.Int64("diff", int64(getCount-putCount)))
	} else if getCount > putCount {
		logger.Get().Warn(poolType,
			zap.String("POOL_STATUS", "OK"),
			zap.String("type", typeName),
			zap.Uint64("get_count", getCount),
			zap.Uint64("put_count", putCount),
			zap.Int64("diff", int64(getCount-putCount)))
	} else {
		logger.Get().Error(poolType,
			zap.String("POOL_STATUS", "OK"),
			zap.String("type", typeName),
			zap.Uint64("get_count", getCount),
			zap.Uint64("put_count", putCount),
			zap.Int64("diff", int64(getCount-putCount)))
	}
}
