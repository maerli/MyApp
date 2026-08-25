package main

import (
	"fmt"
	"net"
	"strings"
)

func iniciarDiscovery() {
	addr := net.UDPAddr{
		Port: 9876,
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		fmt.Println("Erro discovery:", err)
		return
	}

	fmt.Println("Discovery UDP ativo na porta 9876")

	buffer := make([]byte, 1024)

	for {
		n, cliente, err := conn.ReadFromUDP(buffer)

		if err != nil {
			continue
		}

		mensagem := strings.TrimSpace(
			string(buffer[:n]),
		)

		fmt.Println(
			"Discovery:",
			mensagem,
			"de",
			cliente.IP,
		)

		if mensagem == "DESCOBRIR_MEUAPP" {

			resposta := []byte(
				"MEUAPP:8080",
			)

			_, err := conn.WriteToUDP(
				resposta,
				cliente,
			)

			if err != nil {
				fmt.Println(
					"Erro resposta:",
					err,
				)
			}
		}
	}
}
