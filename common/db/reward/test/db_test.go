package test

import (
	"context"
	"fmt"
	"gameServer/common/db/heros"
	"gameServer/common/db/items"
	"gameServer/common/db/reward"
	"gameServer/pkg/cache/ssdb"
	"gameServer/pkg/redis"
	"sync"
	"testing"
	"time"

	"github.com/seefan/gossdb/v2/conf"
	"golang.org/x/exp/rand"
)

func TestRun2(t *testing.T) {

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ========== 初始化阶段 ==========
	if err := initialize(); err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer cleanup()

	// ========== 获取数据 ==========
	allItems, err := items.GetAllItems(123456)
	if err != nil {
		t.Fatalf("获取物品失败: %v", err)
	}
	t.Logf("初始物品: %+v", allItems)

	// ========== 并发执行 ==========
	const (
		goroutineCount = 300  // 总协程数
		batchSize      = 1000 // 每批启动数量（防止瞬间启动过多）
	)

	var wg sync.WaitGroup
	wg.Add(goroutineCount)
	rewardInfo := reward.GetAllRewardInfo(123456)
	t.Logf("rrr: %+v", rewardInfo)

	// 使用有缓冲通道控制并发速率
	semaphore := make(chan struct{}, batchSize)
	start := time.Now().UnixMilli()
	for i := 0; i < goroutineCount; i++ {
		//itoa := strconv.Itoa(i)
		semaphore <- struct{}{} // 获取令牌
		go func(index int) {
			defer wg.Done()
			defer func() { <-semaphore }() // 释放令牌
			saveWithRetry(123456, i)
			rewardInfo = reward.GetAllRewardInfo(123456)

			//if err := saveWithRetry(123456, -1); err != nil {
			//	t.Errorf("协程 %d 执行失败: %v", index, err)
			//}
			//if err := saveWithRetry(123456, 1); err != nil {
			//	t.Errorf("协程 %d 执行失败: %v", index, err)
			//}
		}(i)
	}

	// 等待所有协程完成
	wg.Wait()
	close(semaphore)

	// ========== 验证结果 ==========
	rewardInfo = reward.GetAllRewardInfo(123456)

	//finalItems, err := items.GetAllItems(123456)
	//if err != nil {
	//	t.Fatalf("获取最终物品失败: %v", err)
	//}
	end := time.Now().UnixMilli()
	t.Logf("rrrr: %+v", rewardInfo)

	//t.Logf("最终物品: %+v", finalItems)
	t.Logf("耗时: %+d", end-start)

}

func initialize() error {
	rand.Seed(uint64(time.Now().UnixNano()))

	// 读取Excel配置
	//r := reader.NewExcelReader("./testExcels")
	//allData, err := r.ReadAllExcels()
	//if err != nil {
	//	return fmt.Errorf("读取Excel失败: %w", err)
	//}
	//
	//if err := r.ReadSheetToStruct(allData, config.GetAllExcelConfig()); err != nil {
	//	return fmt.Errorf("解析Excel失败: %w", err)
	//}
	//
	//// 验证配置
	//for i, info := range config.GetAllExcelConfig() {
	//	if info == nil {
	//		return fmt.Errorf("配置项 %d 为空", i)
	//	}
	//}

	// 初始化SSDB
	cfg := &conf.Config{
		Host:        "127.0.0.1",
		Port:        18888,
		MinPoolSize: 100,
		MaxPoolSize: 1000,
		Encoding:    true,
		AutoClose:   true,
		Password:    "dsafgasfgnliqwrasdgaseghbcuabzavfaf",
	}
	if err := ssdb.Init(cfg); err != nil {
		return fmt.Errorf("初始化SSDB失败: %w", err)
	}

	// 初始化Redis
	redis.NewRedisClient("127.0.0.1:16379")

	// 启动监听
	items.Listening()
	heros.Listening()

	return nil
}

func cleanup() {
	ssdb.Close()
	// 其他清理操作...
}

func saveWithRetry(userId uint64, id int) {
	reward.SaveRewardInfo(userId, id)

}
