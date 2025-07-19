package main

import (
	"strconv"
)

var i2s = strconv.Itoa

func iIf(b bool, t int, f int) int {
	if b {
		return t
	}
	return f
}
