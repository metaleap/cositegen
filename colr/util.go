package main

import (
	"strconv"
)

var i2s = strconv.Itoa

func If[T any](b bool, t T, f T) T {
	if b {
		return t
	}
	return f
}

func assert(b bool) {
	if !b {
		panic(b)
	}
}
