//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("pdf-watcher is Windows-only; use GOOS=windows to build")
}
