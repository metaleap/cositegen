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

func Max(i int, j int) int {
	return If(j > i, j, i)
}
func Min(i int, j int) int {
	return If(j < i, j, i)
}
