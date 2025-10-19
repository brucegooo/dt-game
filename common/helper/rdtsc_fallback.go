//go:build !windows
// +build !windows

package helper

import "time"

// Rdtsc fallback for ARM (M1/M2)
func Rdtsc() uint64 {
	return uint64(time.Now().UnixNano())
}

func Cputicks() uint64 {
	return Rdtsc()
}
