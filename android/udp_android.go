//go:build android

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

		fmt.Println(
			"[UDP] Discovery já está ativo",
		)

		return
	}

	/*
		Escuta em todas as interfaces.

		Isso é importante para receber
		broadcast no Android.
	*/
	conn, err := net.ListenUDP(
		"udp4",
		&net.UDPAddr{
			IP:   net.IPv4zero,
			Port: udpPort,
		},
	)

	if err != nil {
		udpMu.Unlock()

		fmt.Println(
			"[UDP] Erro ao abrir:",
			err,
		)

		return
	}

	udpConn = conn

	udpMu.Unlock()

	fmt.Println(
		"[UDP] Discovery ativo em",
		conn.LocalAddr(),
	)

	buffer := make([]byte, 2048)

	for {

		fmt.Println(
			"[UDP] Aguardando discovery...",
		)

		n, remoto, err :=
			conn.ReadFromUDP(buffer)

		if err != nil {

			udpMu.Lock()

			fechado :=
				udpConn == nil ||
					udpConn != conn

			udpMu.Unlock()

			if fechado {

				fmt.Println(
					"[UDP] Discovery encerrado",
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
			"==============================",
		)

		fmt.Println(
			"[UDP] PACOTE RECEBIDO",
		)

		fmt.Println(
			"[UDP] Mensagem:",
			mensagem,
		)

		fmt.Println(
			"[UDP] Cliente:",
			remoto.IP.String(),
		)

		fmt.Println(
			"[UDP] Porta cliente:",
			remoto.Port,
		)

		fmt.Println(
			"==============================",
		)

		if mensagem != "DISCOVER_MEUAPP" {

			fmt.Println(
				"[UDP] Mensagem ignorada",
			)

			continue
		}

		responderUDP(
			conn,
			remoto,
		)
	}
}

func responderUDP(
	conn *net.UDPConn,
	cliente *net.UDPAddr,
) {

	resposta :=
		[]byte("MEUAPP|8080")

	fmt.Println(
		"[UDP] Preparando resposta...",
	)

	fmt.Println(
		"[UDP] Destino:",
		cliente.String(),
	)

	/*
		A resposta é enviada pelo MESMO socket
		que recebeu DISCOVER_MEUAPP.

		Não consulta interfaces.
		Não usa net.Interfaces().
		Não usa netlink.
	*/
	n, err :=
		conn.WriteToUDP(
			resposta,
			cliente,
		)

	if err != nil {

		fmt.Println(
			"[UDP] ERRO AO RESPONDER:",
			err,
		)

		return
	}

	fmt.Println(
		"[UDP] RESPOSTA ENVIADA",
	)

	fmt.Println(
		"[UDP] Mensagem:",
		string(resposta),
	)

	fmt.Println(
		"[UDP] Destino:",
		cliente.String(),
	)

	fmt.Println(
		"[UDP] Bytes:",
		n,
	)
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
		"[UDP] Encerrando discovery...",
	)

	if err := conn.Close(); err != nil {

		fmt.Println(
			"[UDP] Erro ao fechar:",
			err,
		)

		return
	}

	fmt.Println(
		"[UDP] Discovery encerrado",
	)
}
