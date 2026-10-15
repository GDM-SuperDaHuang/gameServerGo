package excelConfig

import (
	"gameServer/pkg/loginSdk"
	"sync"
)

// 注册
var (
	// 读写锁 - 保护所有配置数据的并发访问
	configMu sync.RWMutex
)

// 初始化配置表配置
func init() {
	err := UpdateConfig()
	if err != nil {
		panic(err)
	}
	//第三方初始化
	loginSdk.InitWxService()
	err = loginSdk.InitDouyinClient()
	if err != nil {
		panic(err)
	}

}

// todo 配置表加载 UpdateTaskList 原子性更新任务列表（写操作）
func UpdateConfig() error {
	configMu.Lock()         // 获取写锁（阻塞所有读写）
	defer configMu.Unlock() // 函数返回时释放写锁

	return nil
}
