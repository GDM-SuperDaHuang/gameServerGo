package excels_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gameServer/excels"
	cfg "gameServer/excels/gen"
)

// 拷贝真实 data 到临时目录做隔离测试
func copyDataDir(t *testing.T) string {
	t.Helper()
	src := dataDir(t)
	dst := t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		buf, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), buf, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func rewriteItemName(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, "item_tb_item.json")
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]interface{}
	if err := json.Unmarshal(buf, &rows); err != nil {
		t.Fatal(err)
	}
	rows[0]["name"] = name
	out, _ := json.Marshal(rows)
	if err := os.WriteFile(path, out, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestManager_GetAndReload(t *testing.T) {
	dir := copyDataDir(t)
	m, err := excels.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	if m.Get().Tb_Item.Get(10000).Name != "发型" {
		t.Fatal("initial load wrong")
	}
	if m.Item(10000) == nil || m.Item(99999) != nil {
		t.Fatal("Item() helper wrong")
	}

	// 改文件 + 手动 Reload 生效
	rewriteItemName(t, dir, "热更发型")
	if err := m.Reload(); err != nil {
		t.Fatal(err)
	}
	if got := m.Item(10000).Name; got != "热更发型" {
		t.Fatalf("want 热更发型, got %q", got)
	}

	// 写坏文件 Reload 失败，老数据保留
	badPath := filepath.Join(dir, "item_tb_item.json")
	old, _ := os.ReadFile(badPath)
	if err := os.WriteFile(badPath, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := m.Reload(); err == nil {
		t.Fatal("want error on bad json")
	}
	if got := m.Item(10000).Name; got != "热更发型" {
		t.Fatalf("bad reload should keep old data, got %q", got)
	}
	// 恢复后可重新载入
	if err := os.WriteFile(badPath, old, 0644); err != nil {
		t.Fatal(err)
	}
	if err := m.Reload(); err != nil {
		t.Fatal(err)
	}
}

func TestManager_Watch(t *testing.T) {
	dir := copyDataDir(t)
	m, err := excels.New(dir, excels.WithDebounce(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	reloaded := make(chan *cfg.Tables, 4)
	m.OnReload(func(_, newTables *cfg.Tables) {
		reloaded <- newTables
	})

	if err := m.Watch(); err != nil {
		t.Fatal(err)
	}
	if err := m.Watch(); err == nil {
		t.Fatal("double Watch should fail")
	}

	// 触发文件变更，等待自动热更（最多 5s）
	rewriteItemName(t, dir, "监听热更")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if got := m.Item(10000).Name; got == "监听热更" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("watch reload timeout, still %q", m.Item(10000).Name)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
