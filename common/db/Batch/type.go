package Batch

type DirtyType string

const (
	DirtyHero DirtyType = "hero"
	DirtyItem DirtyType = "item"
)

// Redis Key 定义
func DirtyZSetKey(t DirtyType) string {
	return "dirty:" + string(t)
}
func DirtyVersionKey(t DirtyType) string {
	return "dirty:" + string(t) + ":version"
}
