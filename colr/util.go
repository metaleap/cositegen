package main

import (
	"fmt"
	"image/color"
	"strconv"
)

var i2s = strconv.Itoa

func c2s(col color.Color) string {
	switch c := col.(type) {
	case color.RGBA:
		return fmt.Sprintf("#%X%X%X", c.R, c.G, c.B)
	}
	r, g, b, _ := col.RGBA()
	return fmt.Sprintf("#%X%X%X", r, g, b)
}

func f2s(f float64) string {
	return strconv.FormatFloat(f, 'f', 2, 64)
}

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
