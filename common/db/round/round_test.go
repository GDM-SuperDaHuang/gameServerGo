// Package round 单测：覆盖“局内缓冲、终局一次性收获”的纯逻辑分支。
//
// 本文件只测 Redis 未初始化时的内存降级路径（本地单测默认不起 Redis）；
// 若环境中已初始化真实 Redis 则自动 Skip，语义一致性由集成环境覆盖。
//
// 运行：go test ./common/db/round/ -v -count=1。
package round

import (
	"testing"

	"gameServer/pkg/redis"

	"github.com/stretchr/testify/require"
)

func requireMemFallback(t *testing.T) {
	t.Helper()
	if redis.GetRedisClient() != nil {
		t.Skip("redis initialized, mem-fallback cases covered by integration env")
	}
}

// TestRoundBufferAndHarvest 主链路：多次累积可叠加，终局一次取清，重复收获幂等。
func TestRoundBufferAndHarvest(t *testing.T) {
	requireMemFallback(t)
	uid := int64(990101)

	rid := IncrRoundId(uid)
	require.Greater(t, rid, int64(0))
	require.Equal(t, rid, GetCurrentRoundId(uid))
	require.True(t, BeginRound(uid, rid))

	require.True(t, AddPendingReward(uid, rid, map[int]int64{6: 10, 7: 2}))
	require.True(t, AddPendingReward(uid, rid, map[int]int64{6: 5}))
	require.Equal(t, map[int]int64{6: 15, 7: 2}, GetPending(uid, rid))

	rewards, harvested := TakePendingRewards(uid, rid)
	require.True(t, harvested)
	require.Equal(t, map[int]int64{6: 15, 7: 2}, rewards)

	// 重复收获幂等：不再产出
	rewards, harvested = TakePendingRewards(uid, rid)
	require.False(t, harvested)
	require.Empty(t, rewards)

	// 已结算的局拒绝迟到写入
	require.False(t, AddPendingReward(uid, rid, map[int]int64{6: 1}))
}

// TestRoundStaleRejected 旧局上报拒绝，新局可用。
func TestRoundStaleRejected(t *testing.T) {
	requireMemFallback(t)
	uid := int64(990102)

	oldRid := IncrRoundId(uid)
	newRid := IncrRoundId(uid)
	require.Greater(t, newRid, oldRid)

	require.False(t, AddPendingReward(uid, oldRid, map[int]int64{6: 1}))
	require.True(t, AddPendingReward(uid, newRid, map[int]int64{6: 1}))
	require.Equal(t, map[int]int64{6: 1}, GetPending(uid, newRid))
}

// TestRoundInvalidArgs 非法参数永不 panic，全部拒绝。
func TestRoundInvalidArgs(t *testing.T) {
	requireMemFallback(t)
	uid := int64(990103)
	rid := IncrRoundId(uid)

	require.False(t, AddPendingReward(0, rid, map[int]int64{6: 1}))
	require.False(t, AddPendingReward(uid, 0, map[int]int64{6: 1}))
	require.False(t, AddPendingReward(uid, -1, map[int]int64{6: 1}))
	require.False(t, BeginRound(uid, 0))
	require.Nil(t, GetPending(uid, 0))

	// 未来局不允许结算（且不写 settled，避免误伤未开始的局）
	_, harvested := TakePendingRewards(uid, rid+100)
	require.False(t, harvested)
	require.True(t, AddPendingReward(uid, rid, map[int]int64{6: 1}))
}

// TestRoundDiscard 废弃对局：pending 清空且后续读写均关闭。
func TestRoundDiscard(t *testing.T) {
	requireMemFallback(t)
	uid := int64(990104)
	rid := IncrRoundId(uid)

	require.True(t, AddPendingReward(uid, rid, map[int]int64{6: 3}))
	require.True(t, DiscardRound(uid, rid))

	_, harvested := TakePendingRewards(uid, rid)
	require.False(t, harvested)
	require.False(t, AddPendingReward(uid, rid, map[int]int64{6: 1}))
	require.False(t, DiscardRound(0, rid))
}
