//go:build !server
// +build !server

package main

func topHelp() string {
	return clientHelp
}
