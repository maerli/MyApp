

//go:build linux && !android

package main
package main

import (
	"fmt"
	"net"
	"strings"
)

const udpPort = 45454

func iniciarUDPDiscovery() {

	conn, err := net.ListenUDP(
		"udp4",
		&net.UDPAddr{
			IP:   net.IPv4zero,
			Port: udpPort,
		},
	)

	if err != nil {
		fmt.Println(
			"Erro UDP:",
			err,
		)
		return
	}

	defer conn.Close()

	fmt.Println(
		"UDP Discovery ativo:",
		udpPort,
	)

	buffer := make([]byte, 1024)

	for {

		n, remoto, err :=
			conn.ReadFromUDP(buffer)

		if err != nil {
			continue
		}

		mensagem :=
			strings.TrimSpace(
				string(buffer[:n]),
			)

		if mensagem != "DISCOVER_MEUAPP" {
			continue
		}

		_, err =
			conn.WriteToUDP(
				[]byte("MEUAPP|8080"),
				remoto,
			)

		if err != nil {
			fmt.Println(
				"Erro ao responder:",
				err,
			)
		}
	}
}
