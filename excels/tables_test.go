package excels_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	cfg "gameServer/excels/gen"
)

// 定位到 excels/data 目录，不依赖 go test 的执行 cwd
func dataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(filename), "data")
}

func jsonLoader(t *testing.T) cfg.JsonLoader {
	t.Helper()
	dir := dataDir(t)
	return func(name string) ([]map[string]interface{}, error) {
		buf, err := os.ReadFile(filepath.Join(dir, name+".json"))
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

func TestNewTables(t *testing.T) {
	tables, err := cfg.NewTables(jsonLoader(t))
	if err != nil {
		t.Fatalf("NewTables failed: %v", err)
	}
	if tables.Tb_Item == nil {
		t.Fatal("Tb_Item is nil")
	}

	// data/item_tb_item.json 共 10 条：10000~10009
	list := tables.Tb_Item.GetDataList()
	if len(list) != 10 {
		t.Fatalf("want 10 items, got %d", len(list))
	}
	if len(tables.Tb_Item.GetDataMap()) != 10 {
		t.Fatalf("want map size 10, got %d", len(tables.Tb_Item.GetDataMap()))
	}

	// 普通字段 + bean list/bean 列
	item := tables.Tb_Item.Get(10000)
	if item == nil {
		t.Fatal("Get(10000) is nil")
	}
	if item.Name != "发型" || item.Price != 100 || item.UpgradeToItemId != 10001 {
		t.Fatalf("unexpected item 10000: %+v", item)
	}
	if item.Quality != cfg.ItemEQuality_WHITE {
		t.Fatalf("want quality WHITE(%d), got %d", cfg.ItemEQuality_WHITE, item.Quality)
	}
	if item.ExpireTime != nil {
		t.Fatalf("item 10000 ExpireTime should be nil, got %d", *item.ExpireTime)
	}
	if len(item.ExchangeList) != 2 || item.ExchangeList[0].Id != 10001 || item.ExchangeList[0].Num != 2 {
		t.Fatalf("unexpected ExchangeList: %+v", item.ExchangeList)
	}
	if item.ExchangeColumn == nil || item.ExchangeColumn.Id != 10001 || item.ExchangeColumn.Num != 2 {
		t.Fatalf("unexpected ExchangeColumn: %+v", item.ExchangeColumn)
	}

	// 可空 expire_time 有值的行
	expireItem := tables.Tb_Item.Get(10007)
	if expireItem == nil {
		t.Fatal("Get(10007) is nil")
	}
	if expireItem.ExpireTime == nil || *expireItem.ExpireTime != 1633881599 {
		t.Fatalf("unexpected ExpireTime for 10007: %+v", expireItem.ExpireTime)
	}

	// 不存在的 key 返回 nil
	if tables.Tb_Item.Get(99999) != nil {
		t.Fatal("Get(99999) should be nil")
	}
}

func TestNewTables_LoaderError(t *testing.T) {
	_, err := cfg.NewTables(func(string) ([]map[string]interface{}, error) {
		return nil, os.ErrNotExist
	})
	if err == nil {
		t.Fatal("want error when loader fails, got nil")
	}
}
