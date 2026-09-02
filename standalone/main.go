package main

import (
	"embed"
	"fmt"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	fmt.Println("================================")
	fmt.Println("COMANDA FÁCIL - ANDROID")
	fmt.Println("================================")

	fmt.Println("[APP] Iniciando UDP...")
	go iniciarUDPDiscovery()

	fmt.Println("[APP] Iniciando HTTP...")

	if err := iniciarServidor(); err != nil {
		fmt.Println(
			"[HTTP] Servidor encerrado:",
			err,
		)
	}
}
