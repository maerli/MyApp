package main

import (
	"context"
	"fmt"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup é chamado quando o Wails inicia.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	fmt.Println("[APP] Iniciando aplicação...")

	// UDP precisa iniciar independentemente do servidor HTTP.
	go iniciarUDPDiscovery()

	// Se iniciarServidor bloqueia no router.Run(),
	// ele também precisa ficar em goroutine.
	go func() {
		fmt.Println("[HTTP] Iniciando servidor...")

		if err := iniciarServidor(); err != nil {
			fmt.Println(
				"[HTTP] Servidor encerrado:",
				err,
			)
		}
	}()
}

// shutdown é chamado ao fechar o Wails.
func (a *App) shutdown(ctx context.Context) {
	fmt.Println("[APP] Encerrando aplicação...")

	pararUDPDiscovery()

	fmt.Println("[APP] UDP encerrado")
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf(
		"Hello %s, ♡ hIt's show time!",
		name,
	)
}
