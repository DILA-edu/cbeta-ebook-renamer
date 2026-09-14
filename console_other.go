//go:build !windows

package main

// macOS 與 Linux 的終端機本來就是 UTF-8，不需要額外處理。
func initConsole() {}
