package snowflake

import (
	"sync"
	"time"
)

const (
	workerBits = 10
	seqBits    = 12

	maxWorkerId = -1 ^ (-1 << workerBits)
	maxSeq      = -1 ^ (-1 << seqBits)

	workerShift = seqBits
	timeShift   = seqBits + workerBits
)

type Node struct {
	mu        sync.Mutex
	workerId  uint64
	sequence  uint64
	lastStamp uint64
}

// 使用 uint64 作为 workerId
func NewNode(workerId uint64) *Node {
	if workerId > maxWorkerId {
		panic("workerId out of range")
	}

	return &Node{
		workerId: workerId,
	}
}

// 返回 uint64 RoomType
func (n *Node) Generate() uint64 {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := uint64(time.Now().UnixMilli())

	if now == n.lastStamp {
		n.sequence = (n.sequence + 1) & maxSeq
		if n.sequence == 0 {
			// 等到下一个毫秒
			for now <= n.lastStamp {
				now = uint64(time.Now().UnixMilli())
			}
		}
	} else {
		n.sequence = 0
	}

	n.lastStamp = now

	id := (now << timeShift) |
		(n.workerId << workerShift) |
		n.sequence

	return id
}
