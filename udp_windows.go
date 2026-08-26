//go:build windows

package main

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
)

const udpPort = 45454

func iniciarUDPDiscovery() {

	/*
		Inicializa Winsock manualmente.
		0x0202 = Winsock 2.2
	*/
	var wsaData windows.WSAData

	err := windows.WSAStartup(
		0x0202,
		&wsaData,
	)

	if err != nil {
		fmt.Println(
			"Erro WSAStartup:",
			err,
		)
		return
	}

	defer windows.WSACleanup()

	/*
		Cria o socket DIRETAMENTE.

		IMPORTANTE:
		não usamos net.ListenUDP(),
		portanto o Go NÃO executará:

		WSAIoctl(SIO_UDP_CONNRESET)
	*/
	socket, err := windows.Socket(
		windows.AF_INET,
		windows.SOCK_DGRAM,
		windows.IPPROTO_UDP,
	)

	if err != nil {
		fmt.Println(
			"Erro criando socket UDP:",
			err,
		)
		return
	}

	defer windows.Closesocket(socket)

	/*
		Escuta em:

		0.0.0.0:45454
	*/
	addr := &windows.SockaddrInet4{
		Port: udpPort,

		Addr: [4]byte{
			0,
			0,
			0,
			0,
		},
	}

	err = windows.Bind(
		socket,
		addr,
	)

	if err != nil {
		fmt.Println(
			"Erro bind UDP:",
			err,
		)
		return
	}

	fmt.Println(
		"UDP Discovery ativo em 0.0.0.0:45454",
	)

	fmt.Println(
		"Aguardando DISCOVER_MEUAPP...",
	)

	buffer :=
		make([]byte, 1024)

	for {

		n, remoto, err :=
			windows.Recvfrom(
				socket,
				buffer,
				0,
			)

		if err != nil {
			fmt.Println(
				"Erro recebendo UDP:",
				err,
			)

			continue
		}

		if n <= 0 {
			continue
		}

		mensagem :=
			strings.TrimSpace(
				string(
					buffer[:n],
				),
			)

		fmt.Println(
			"UDP recebido:",
			mensagem,
		)

		if mensagem !=
			"DISCOVER_MEUAPP" {

			continue
		}

		/*
			O APK obterá o IP do próprio
			endereço de origem da resposta.

			Então só precisamos informar
			a porta HTTP.
		*/
		resposta :=
			[]byte(
				"MEUAPP|8080",
			)

		err =
			windows.Sendto(
				socket,
				resposta,
				0,
				remoto,
			)

		if err != nil {
			fmt.Println(
				"Erro respondendo UDP:",
				err,
			)

			continue
		}

		fmt.Println(
			"Discovery respondido:",
			"MEUAPP|8080",
		)

		if ip,
			ok :=
			remoto.(*windows.SockaddrInet4);
			ok {

			fmt.Printf(
				"Cliente: %d.%d.%d.%d:%d\n",
				ip.Addr[0],
				ip.Addr[1],
				ip.Addr[2],
				ip.Addr[3],
				ip.Port,
			)
		}
	}
}
