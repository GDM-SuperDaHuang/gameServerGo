package Batch

import "fmt"

type Factory func(uid uint64) Storage

type Registry struct {
	factories map[DirtyType]Factory
}

// 空注册器
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[DirtyType]Factory),
	}

}

// 注册某种类型的数据
func (r *Registry) Register(t DirtyType, f Factory) {
	r.factories[t] = f

}

// 创建某玩家的批量数据
func (r *Registry) Create(t DirtyType, uid uint64) (Storage, error) {
	f, ok := r.factories[t]
	if !ok {
		return nil, fmt.Errorf("storage not register:%s", t)
	}
	return f(uid), nil
}

// 总类型列表
func (r *Registry) Types() []DirtyType {
	result := make([]DirtyType, 0)
	for k := range r.factories {
		result = append(
			result,
			k,
		)
	}
	return result
}
