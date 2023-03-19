package utils

import "time"

func NowInSec() uint64 {
	return uint64(time.Now().UTC().Unix())
}

func NowInMs() uint64 {
	return uint64(time.Now().UTC().UnixMilli())
}
