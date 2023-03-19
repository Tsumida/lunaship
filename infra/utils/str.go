package utils

import (
	"golang.org/x/exp/constraints"
)

type Default interface {
	constraints.Ordered
}

func StrOrDefault(str string, other string) string {
	if str != "" {
		return str
	} else {
		return other
	}
}
