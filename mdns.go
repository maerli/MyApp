package main

import (
	"fmt"
	"net"
	"os"

	zeroconf "github.com/enbility/zeroconf/v2"
)

var mdnsServer *zeroconf.Server

func iniciarMDNS() {

	iface, ip, err := obterInterfaceLAN()

	if err != nil {
		fmt.Println("Erro interface mDNS:", err)
		return
	}

	host, err := os.Hostname()

	if err != nil {
		fmt.Println("Erro hostname:", err)
		return
	}

	hostMDNS := host + ".local."

	fmt.Println("==============================")
	fmt.Println("INICIANDO mDNS")
	fmt.Println("Interface:", iface.Name)
	fmt.Println("IP:", ip.String())
	fmt.Println("Host:", hostMDNS)
	fmt.Println("Serviço: MeuApp._meuapp._tcp.local.")
	fmt.Println("Porta: 8080")
	fmt.Println("==============================")

	mdnsServer, err =
		zeroconf.RegisterProxy(
			"MeuApp",

			"_meuapp._tcp",

			"local.",

			8080,

			hostMDNS,

			[]string{
				ip.String(),
			},

			[]string{
				"app=comandas",
				"version=1",
			},

			[]net.Interface{
				*iface,
			},
		)

	if err != nil {

		fmt.Println(
			"ERRO mDNS:",
			err,
		)

		return
	}

	fmt.Println()
	fmt.Println("mDNS ATIVO!")
	fmt.Println(
		"MeuApp._meuapp._tcp.local.",
	)

	fmt.Printf(
		"IP anunciado: %s:8080\n",
		ip.String(),
	)
}

func obterInterfaceLAN() (
	*net.Interface,
	net.IP,
	error,
) {

	interfaces, err :=
		net.Interfaces()

	if err != nil {
		return nil, nil, err
	}

	for i := range interfaces {

		iface :=
			&interfaces[i]

		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		if iface.Flags&net.FlagMulticast == 0 {
			continue
		}

		addrs, err :=
			iface.Addrs()

		if err != nil {
			continue
		}

		for _, addr :=
			range addrs {

			var ip net.IP

			switch v :=
				addr.(type) {

			case *net.IPNet:
				ip = v.IP

			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil {
				continue
			}

			ip4 :=
				ip.To4()

			if ip4 == nil {
				continue
			}

			fmt.Printf(
				"Interface %s -> %s\n",
				iface.Name,
				ip4.String(),
			)

			/*
			   Seu computador atualmente
			   está na rede 192.168.x.x
			*/
			if ip4[0] == 192 &&
				ip4[1] == 168 {

				fmt.Println()
				fmt.Println(
					">>> Interface mDNS escolhida:",
					iface.Name,
				)

				return iface, ip4, nil
			}
		}
	}

	return nil,
		nil,
		fmt.Errorf(
			"nenhuma interface 192.168.x.x encontrada",
		)
}

func pararMDNS() {

	if mdnsServer == nil {
		return
	}

	mdnsServer.Shutdown()

	mdnsServer = nil

	fmt.Println(
		"mDNS encerrado",
	)
}
