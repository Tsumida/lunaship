// Some useful logical operation for data processing
package utils

import "golang.org/x/exp/constraints"

func Diff[T any, K comparable](list1 []T, list2 []T, Key func(item T) K) (noInList1, noInList2 []T) {

	m1 := make(map[K]T, len(list1)+1)
	m2 := make(map[K]T, len(list2)+1)

	for _, e := range list1 {
		m1[Key(e)] = e
	}

	for _, e := range list2 {
		m2[Key(e)] = e
	}

	for k, v := range m1 {
		if _, ok := m2[k]; !ok {
			noInList2 = append(noInList2, v)
		}
	}

	for k, v := range m2 {
		if _, ok := m1[k]; !ok {
			noInList1 = append(noInList1, v)
		}
	}

	return
}

func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func All(bs ...func() bool) (int, bool) {
	for i, v := range bs {
		if !v() {
			return i, false
		}
	}
	return -1, true
}
