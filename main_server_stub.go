//go:build !server
// +build !server

package main

import (
	"fmt"
	"os"
)

// server is stubbed out in client-only builds.
func server(args []string) {
	fmt.Println("server mode is not included in this build (built without 'server' tag)")
	os.Exit(2)
}
