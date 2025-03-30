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

func MaptoConcurentMap[T comparable, V any](src map[T]V) *sync.Map {
	var result = &sync.Map{}
	for k, v := range src {
		result.Store(k, v)
	}
	return result
}
