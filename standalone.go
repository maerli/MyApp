package main

import (
	"embed"
	"fmt"
	
	

)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	go iniciarServidor()

	go iniciarUDPDiscovery()

	fmt.Println(
		"Servidor HTTP: 8080",
	)

	fmt.Println(
		"Discovery UDP: 45454",
	)

	select {}
}
