//go:build windows

package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

const udpPort = 45454

var (
	udpConn *net.UDPConn
	udpMu   sync.Mutex
)

func iniciarUDPDiscovery() {
	udpMu.Lock()

	if udpConn != nil {
		udpMu.Unlock()
		fmt.Println("[UDP] Já está ativo")
		return
	}

	conn, err := net.ListenUDP(
		"udp4",
		&net.UDPAddr{
			IP:   net.IPv4zero,
			Port: udpPort,
		},
	)

	if err != nil {
		udpMu.Unlock()
		fmt.Println("[UDP] Erro:", err)
		return
	}

	udpConn = conn
	udpMu.Unlock()

	fmt.Println(
		"[UDP] Discovery ativo na porta",
		udpPort,
	)

	buffer := make([]byte, 1024)

	for {
		n, remoto, err :=
			conn.ReadFromUDP(buffer)

		if err != nil {
			udpMu.Lock()

			fechado :=
				udpConn != conn

			udpMu.Unlock()

			if fechado {
				fmt.Println(
					"[UDP] Encerrado",
				)
				return
			}

			fmt.Println(
				"[UDP] Erro leitura:",
				err,
			)

			continue
		}

		mensagem :=
			strings.TrimSpace(
				string(buffer[:n]),
			)

		fmt.Println(
			"[UDP] Recebido:",
			mensagem,
			"de",
			remoto.String(),
		)

		if mensagem != "DISCOVER_MEUAPP" {
			continue
		}

		_, err = conn.WriteToUDP(
			[]byte("MEUAPP|8080"),
			remoto,
		)

		if err != nil {
			fmt.Println(
				"[UDP] Erro resposta:",
				err,
			)

			continue
		}

		fmt.Println(
			"[UDP] Resposta enviada para",
			remoto.String(),
		)
	}
}

func pararUDPDiscovery() {
	udpMu.Lock()

	conn := udpConn
	udpConn = nil

	udpMu.Unlock()

	if conn == nil {
		return
	}

	fmt.Println(
		"[UDP] Fechando porta",
		udpPort,
	)

	_ = conn.Close()
}
