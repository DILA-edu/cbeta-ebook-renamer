//go:build windows

package main

import "syscall"

// initConsole 把主控台的編碼切成 UTF-8。
// 不做這件事的話，cmd.exe 預設的 code page(950/437) 會把中文訊息印成亂碼。
func initConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	const cpUTF8 = 65001
	_, _, _ = kernel32.NewProc("SetConsoleOutputCP").Call(uintptr(cpUTF8))
	_, _, _ = kernel32.NewProc("SetConsoleCP").Call(uintptr(cpUTF8))
}
