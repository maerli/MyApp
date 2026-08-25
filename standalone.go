package main

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Servidor HTTP
	go iniciarServidor()

	// mDNS
	iniciarMDNS()

	fmt.Println("================================")
	fmt.Println("Servidor HTTP + mDNS ativos")
	fmt.Println("HTTP: porta 8080")
	fmt.Println("mDNS: _meuapp._tcp.local.")
	fmt.Println("CTRL+C para encerrar")
	fmt.Println("================================")

	ch := make(chan os.Signal, 1)

	signal.Notify(
		ch,
		os.Interrupt,
		syscall.SIGTERM,
	)

	<-ch

	fmt.Println("Encerrando...")

	pararMDNS()
}
