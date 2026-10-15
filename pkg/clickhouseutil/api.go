package clickhouseutil

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Select 执行查询并将结果扫描到 dest (必须是一个指向切片的指针，例如 &[]User{})
func (c *Client) Select(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	// 使用 conn.Select 可以利用驱动的高效序列化
	return c.conn.Select(ctx, dest, query, args...)
}

// 用于执行 INSERT, UPDATE, DELETE, ALTER 等不返回结果集的 SQL 语句。
func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) error {
	return c.conn.Exec(ctx, query, args...)
}

// QueryRow 执行查询并返回单行结果
func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return c.conn.QueryRow(ctx, query, args...)
}

// BatchInsert 提供一个批量插入的构建器，支持列式追加数据。
// 返回的 Batch 对象需要调用 Send() 来最终提交。
func (c *Client) PrepareBatch(ctx context.Context, query string) (driver.Batch, error) {
	return c.conn.PrepareBatch(ctx, query)
}

// 一个便捷的批量插入方法示例
func (c *Client) BatchInsert(ctx context.Context, query string, items [][]interface{}) error {
	batch, err := c.PrepareBatch(ctx, query)
	if err != nil {
		return err
	}
	// 注意：生产环境中请确保在出错时处理 batch 的清理，避免连接泄漏[reference:6]
	defer func() {
		if err != nil {
			_ = batch.Abort()
		}
	}()

	for _, item := range items {
		// Append 方法接收可变参数，与 VALUES 中的列一一对应
		if err := batch.Append(item...); err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}
	return batch.Send()
}

// Stats() 方法获取连接池状态，这对于排查问题和容量规划非常有用
func (c *Client) Stats() driver.Stats {
	return c.conn.Stats()
}
