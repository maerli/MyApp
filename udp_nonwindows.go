//go:build !windows && !android

package main

import "fmt"

// Estas funções existem apenas para permitir que o Wails
// gere os bindings quando está sendo executado fora do Windows.
//
// No build final windows/amd64, este arquivo é automaticamente
// excluído e udp_windows.go é utilizado.

func iniciarUDPDiscovery() {
	fmt.Println("[UDP] stub não-Windows")
}

func pararUDPDiscovery() {
	fmt.Println("[UDP] stub não-Windows")
}
