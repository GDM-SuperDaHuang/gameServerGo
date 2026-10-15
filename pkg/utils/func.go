package utils

import (
	"strconv"
)

func ToInt64(t string) uint64 {
	parseInt, err := strconv.ParseUint(t, 10, 64)
	if err != nil {
		return 0
	}
	return parseInt
}

func ToString(t uint64) string {
	return strconv.FormatUint(t, 10)
}
