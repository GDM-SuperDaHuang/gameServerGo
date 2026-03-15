package salt

import (
	"gameServer/pkg/bytes"
	"math/rand"
	"sync"
	"time"
)

var (
	lowerLetters = []byte("abcdefghijklmnopqrstuvwxyz")
	randPool     = sync.Pool{
		New: func() any {
			return rand.New(rand.NewSource(time.Now().UnixNano()))
		},
	}
)

// Lower 获取指定长度的字符串，仅包含小写字母
func Lower(length int) string {
	return rs(lowerLetters, length)
}

func rs(letters []byte, length int) string {
	b := bytes.Get().Buffer(length)
	defer bytes.Get().Release(b)

	r := getRand()
	defer putRand(r)

	for range length {
		b.WriteByte(letters[r.Intn(len(letters))])
	}

	return b.String()
}

func getRand() *rand.Rand {
	return randPool.Get().(*rand.Rand)
}

func putRand(r *rand.Rand) {
	randPool.Put(r)
}
