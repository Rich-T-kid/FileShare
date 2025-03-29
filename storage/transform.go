package storage

import "sync"

// useful transformations and data structures

func ConcurentMaptoMap[T comparable, V any](src *sync.Map) map[T]V {
	var result = make(map[T]V)
	src.Range(func(k, v any) bool {
		key, ok := k.(T)
		val, ok1 := v.(V)
		if ok && ok1 {
			result[key] = val
		}
		return true
	})
	return result
}
