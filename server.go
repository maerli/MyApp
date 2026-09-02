package main

import (
	"database/sql"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"

	_ "modernc.org/sqlite"
)

const (
	pixKeyPadrao    = "+5588981628528"
	pixNomePadrao   = "SEU NOME"
	pixCidadePadrao = "SUA CIDADE"
)

var (
	db        *sql.DB
	jwtSecret = []byte(segredoJWT())
	fusoLocal = time.FixedZone("America/Fortaleza", -3*60*60)
	tempoReal = novoHubWebSocket()
)

type Produto struct {
	ID        int64   `json:"id"`
	Nome      string  `json:"nome"`
	Categoria string  `json:"categoria"`
	Preco     float64 `json:"preco"`
	Ativo     bool    `json:"ativo"`
}

type Usuario struct {
	ID     int64  `json:"id"`
	Nome   string `json:"nome"`
	Login  string `json:"login"`
	Perfil string `json:"perfil"`
	Ativo  bool   `json:"ativo"`
}

type Comanda struct {
	ID             int64   `json:"id"`
	Mesa           string  `json:"mesa"`
	Status         string  `json:"status"`
	UsuarioID      int64   `json:"usuario_id"`
	Usuario        string  `json:"usuario"`
	FormaPagamento string  `json:"forma_pagamento"`
	Total          float64 `json:"total"`
	CriadaEm       string  `json:"criada_em"`
}

type Item struct {
	ID         int64   `json:"id"`
	ProdutoID  int64   `json:"produto_id"`
	Nome       string  `json:"nome"`
	Quantidade int     `json:"quantidade"`
	Preco      float64 `json:"preco"`
	Subtotal   float64 `json:"subtotal"`
}

type RelatorioUsuario struct {
	UsuarioID          int64   `json:"usuario_id"`
	Usuario            string  `json:"usuario"`
	QuantidadeComandas int     `json:"quantidade_comandas"`
	Dinheiro           float64 `json:"dinheiro"`
	Cartao             float64 `json:"cartao"`
	Pix                float64 `json:"pix"`
	Total              float64 `json:"total"`
}

type RelatorioDia struct {
	Data               string             `json:"data"`
	QuantidadeComandas int                `json:"quantidade_comandas"`
	Dinheiro           float64            `json:"dinheiro"`
	Cartao             float64            `json:"cartao"`
	Pix                float64            `json:"pix"`
	Total              float64            `json:"total"`
	AtualizadoEm       string             `json:"atualizado_em"`
	Usuarios           []RelatorioUsuario `json:"usuarios"`
}

type eventoTempoReal struct {
	Tipo          string   `json:"tipo"`
	Recursos      []string `json:"recursos,omitempty"`
	ComandaID     int64    `json:"comanda_id,omitempty"`
	OrigemCliente string   `json:"-"`
}

type clienteWebSocket struct {
	conexao *websocket.Conn
	envio   chan eventoTempoReal
	id      string
}

type hubWebSocket struct {
	clientes map[*clienteWebSocket]struct{}
	entrar   chan *clienteWebSocket
	sair     chan *clienteWebSocket
	eventos  chan eventoTempoReal
}

func novoHubWebSocket() *hubWebSocket {
	hub := &hubWebSocket{
		clientes: make(map[*clienteWebSocket]struct{}),
		entrar:   make(chan *clienteWebSocket),
		sair:     make(chan *clienteWebSocket),
		eventos:  make(chan eventoTempoReal, 64),
	}
	go hub.executar()
	return hub
}

func (hub *hubWebSocket) executar() {
	for {
		select {
		case cliente := <-hub.entrar:
			hub.clientes[cliente] = struct{}{}
		case cliente := <-hub.sair:
			hub.remover(cliente)
		case evento := <-hub.eventos:
			for cliente := range hub.clientes {
				if evento.OrigemCliente != "" && evento.OrigemCliente == cliente.id {
					continue
				}
				select {
				case cliente.envio <- evento:
				default:
					hub.remover(cliente)
				}
			}
		}
	}
}

func (hub *hubWebSocket) remover(cliente *clienteWebSocket) {
	if _, existe := hub.clientes[cliente]; !existe {
		return
	}
	delete(hub.clientes, cliente)
	close(cliente.envio)
	_ = cliente.conexao.Close()
}

func (cliente *clienteWebSocket) ler() {
	defer func() {
		tempoReal.sair <- cliente
		_ = cliente.conexao.Close()
	}()

	cliente.conexao.SetReadLimit(1024)
	_ = cliente.conexao.SetReadDeadline(time.Now().Add(60 * time.Second))
	cliente.conexao.SetPongHandler(func(string) error {
		return cliente.conexao.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	for {
		if _, _, err := cliente.conexao.ReadMessage(); err != nil {
			return
		}
	}
}

func (cliente *clienteWebSocket) escrever() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		_ = cliente.conexao.Close()
	}()

	for {
		select {
		case evento, aberto := <-cliente.envio:
			_ = cliente.conexao.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if !aberto {
				_ = cliente.conexao.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := cliente.conexao.WriteJSON(evento); err != nil {
				return
			}
		case <-ticker.C:
			_ = cliente.conexao.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := cliente.conexao.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func notificarTempoReal(c *gin.Context, comandaID int64, recursos ...string) {
	tempoReal.eventos <- eventoTempoReal{
		Tipo:          "atualizar",
		Recursos:      recursos,
		ComandaID:     comandaID,
		OrigemCliente: c.GetHeader("X-Client-ID"),
	}
}

func origemPermitida(origin string) bool {
	return origin == "" ||
		strings.HasPrefix(origin, "wails://") ||
		strings.HasPrefix(origin, "http://") ||
		strings.HasPrefix(origin, "https://")
}

var atualizadorWebSocket = websocket.Upgrader{
	Subprotocols: []string{"comanda-facil"},
	CheckOrigin: func(r *http.Request) bool {
		return origemPermitida(r.Header.Get("Origin"))
	},
}

func segredoJWT() string {
	if segredo := strings.TrimSpace(os.Getenv("COMANDAS_JWT_SECRET")); segredo != "" {
		return segredo
	}
	return "comanda-facil-local-altere-esta-chave-em-producao"
}

func configurarFrontend(router *gin.Engine) {
	frontendFS, err := fs.Sub(
		assets,
		"frontend/dist",
	)
	if err != nil {
		panic(err)
	}

	indexHTML, err := fs.ReadFile(
		frontendFS,
		"index.html",
	)
	if err != nil {
		panic(err)
	}

	router.GET(
		"/assets/*filepath",
		func(c *gin.Context) {
			caminho :=
				strings.TrimPrefix(
					c.Param("filepath"),
					"/",
				)

			conteudo, err :=
				fs.ReadFile(
					frontendFS,
					"assets/"+caminho,
				)

			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}

			switch {
			case strings.HasSuffix(caminho, ".js"):
				c.Header(
					"Content-Type",
					"application/javascript; charset=utf-8",
				)

			case strings.HasSuffix(caminho, ".css"):
				c.Header(
					"Content-Type",
					"text/css; charset=utf-8",
				)

			case strings.HasSuffix(caminho, ".svg"):
				c.Header(
					"Content-Type",
					"image/svg+xml",
				)
			}

			c.Data(
				http.StatusOK,
				c.Writer.Header().Get("Content-Type"),
				conteudo,
			)
		},
	)

	servirIndex := func(c *gin.Context) {
		c.Data(
			http.StatusOK,
			"text/html; charset=utf-8",
			indexHTML,
		)
	}

	router.GET("/", servirIndex)

	router.NoRoute(
		func(c *gin.Context) {
			if strings.HasPrefix(
				c.Request.URL.Path,
				"/api/",
			) {
				c.JSON(
					http.StatusNotFound,
					gin.H{
						"erro": "Rota não encontrada",
					},
				)
				return
			}

			servirIndex(c)
		},
	)
}

func iniciarServidor() error {
	fmt.Println("[1] Abrindo SQLite")

	var err error

	db, err = sql.Open(
		"sqlite",
		"file:comandas.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
	)
	if err != nil {
		return fmt.Errorf("abrir banco: %w", err)
	}

	db.SetMaxOpenConns(1)

	fmt.Println("[2] Ping SQLite")
	if err = db.Ping(); err != nil {
		return fmt.Errorf("conectar ao banco: %w", err)
	}

	fmt.Println("[3] Criando tabelas")
	if err = criarTabelas(); err != nil {
		return fmt.Errorf("criar tabelas: %w", err)
	}

	fmt.Println("[4] Criando administrador")
	if err = criarAdminInicial(); err != nil {
		return fmt.Errorf("criar administrador inicial: %w", err)
	}

	fmt.Println("[5] Banco SQLite pronto")

	// DESATIVADO TEMPORARIAMENTE PARA DIAGNÓSTICO NO ANDROID.
	// Depois que o servidor estiver confirmado funcionando, pode reativar.
	// if err = sincronizarRelatoriosExistentes(); err != nil {
	//     fmt.Println("Aviso ao sincronizar relatórios:", err)
	// }

	fmt.Println("[6] Criando Gin")
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	fmt.Println("[7] Configurando CORS")
	router.Use(
		cors.New(
			cors.Config{
				AllowOriginFunc: func(origin string) bool {
					return origemPermitida(origin)
				},
				AllowMethods: []string{
					"GET",
					"POST",
					"PUT",
					"DELETE",
					"OPTIONS",
				},
				AllowHeaders: []string{
					"Origin",
					"Content-Type",
					"Accept",
					"Authorization",
					"X-Client-ID",
				},
				AllowCredentials: true,
				MaxAge:           12 * time.Hour,
			},
		),
	)

	fmt.Println("[8] Criando rota de teste")
	router.GET(
		"/api/status",
		func(c *gin.Context) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"status": "online",
					"app":    "Comanda Fácil",
				},
			)
		},
	)

	fmt.Println("[9] Configurando API")
	api := router.Group("/api")

	api.POST("/login", login)
	api.GET("/ws", conectarWebSocket)

	auth := api.Group("")
	auth.Use(authMiddleware())

	auth.GET("/produtos", listarProdutos)
	auth.POST("/produtos", adminMiddleware(), criarProduto)
	auth.PUT("/produtos/:id", adminMiddleware(), editarProduto)

	auth.GET("/configuracoes", adminMiddleware(), buscarConfiguracoes)
	auth.PUT("/configuracoes", adminMiddleware(), salvarConfiguracoes)

	auth.GET("/usuarios", adminMiddleware(), listarUsuarios)
	auth.POST("/usuarios", adminMiddleware(), criarUsuario)
	auth.PUT("/usuarios/:id", adminMiddleware(), editarUsuario)

	auth.GET("/comandas", listarComandas)
	auth.POST("/comandas", criarComanda)
	auth.GET("/comandas/:id", buscarComanda)
	auth.DELETE("/comandas/:id", adminMiddleware(), excluirComanda)
	auth.GET("/comandas/:id/itens", listarItens)
	auth.POST("/comandas/:id/itens", adicionarItem)
	auth.PUT("/itens/:id/quantidade", atualizarQuantidadeItem)
	auth.DELETE("/itens/:id", excluirItem)
	auth.POST("/comandas/:id/pagamento", pagarComanda)
	auth.POST("/comandas/:id/confirmar-pix", confirmarPix)

	auth.GET("/caixas", adminMiddleware(), listarCaixas)
	auth.POST(
		"/usuarios/:id/fechar-caixa",
		adminMiddleware(),
		fecharCaixa,
	)
	auth.GET(
		"/usuarios/:id/fechamentos",
		adminMiddleware(),
		listarFechamentos,
	)

	auth.GET(
		"/relatorios/dia",
		adminMiddleware(),
		buscarRelatorioDia,
	)
	auth.GET(
		"/relatorios/datas",
		adminMiddleware(),
		listarDatasRelatorios,
	)
	auth.GET(
		"/relatorios/por-usuario",
		adminMiddleware(),
		relatorioPorUsuario,
	)
	auth.GET(
		"/relatorios/final-dia",
		adminMiddleware(),
		relatorioFinalDia,
	)

	fmt.Println("[10] Configurando frontend")
	configurarFrontend(router)

	fmt.Println("[11] TUDO PRONTO")
	fmt.Println("[12] ABRINDO PORTA 8080")
	fmt.Println("http://127.0.0.1:8080")
	fmt.Println("http://127.0.0.1:8080/api/status")

	if err = router.Run("0.0.0.0:8080"); err != nil {
		return fmt.Errorf("erro ao abrir servidor HTTP: %w", err)
	}

	return nil
}

func criarTabelas() error {
	_, err := db.Exec(`
	CREATE TABLE IF NOT EXISTS usuarios (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		login TEXT NOT NULL UNIQUE,
		senha_hash TEXT NOT NULL,
		perfil TEXT NOT NULL DEFAULT 'usuario',
		ativo INTEGER NOT NULL DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS produtos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nome TEXT NOT NULL,
		categoria TEXT NOT NULL DEFAULT 'Geral',
		preco REAL NOT NULL DEFAULT 0,
		ativo INTEGER NOT NULL DEFAULT 1
	);

	CREATE TABLE IF NOT EXISTS configuracoes (
		chave TEXT PRIMARY KEY,
		valor TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS comandas (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mesa TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'aberta',
		usuario_id INTEGER NOT NULL,
		forma_pagamento TEXT,
		valor_pago REAL NOT NULL DEFAULT 0,
		criada_em DATETIME DEFAULT CURRENT_TIMESTAMP,
		fechada_em DATETIME,
		fechamento_id INTEGER
	);

	CREATE TABLE IF NOT EXISTS itens (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		comanda_id INTEGER NOT NULL,
		produto_id INTEGER NOT NULL,
		quantidade INTEGER NOT NULL DEFAULT 1,
		preco REAL NOT NULL
	);

	CREATE TABLE IF NOT EXISTS fechamentos_caixa (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		usuario_id INTEGER NOT NULL,
		admin_id INTEGER NOT NULL,
		total_dinheiro REAL NOT NULL DEFAULT 0,
		total_cartao REAL NOT NULL DEFAULT 0,
		total_pix REAL NOT NULL DEFAULT 0,
		total_geral REAL NOT NULL DEFAULT 0,
		criado_em DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS relatorios_diarios (
		data TEXT PRIMARY KEY,
		quantidade_comandas INTEGER NOT NULL DEFAULT 0,
		total_dinheiro REAL NOT NULL DEFAULT 0,
		total_cartao REAL NOT NULL DEFAULT 0,
		total_pix REAL NOT NULL DEFAULT 0,
		total_geral REAL NOT NULL DEFAULT 0,
		atualizado_em DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS relatorios_diarios_usuarios (
		data TEXT NOT NULL,
		usuario_id INTEGER NOT NULL,
		usuario TEXT NOT NULL,
		quantidade_comandas INTEGER NOT NULL DEFAULT 0,
		total_dinheiro REAL NOT NULL DEFAULT 0,
		total_cartao REAL NOT NULL DEFAULT 0,
		total_pix REAL NOT NULL DEFAULT 0,
		total_geral REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (data, usuario_id)
	);

	CREATE INDEX IF NOT EXISTS idx_comandas_usuario_status
		ON comandas(usuario_id, status);
	CREATE INDEX IF NOT EXISTS idx_itens_comanda
		ON itens(comanda_id);
	CREATE INDEX IF NOT EXISTS idx_relatorios_usuarios_data
		ON relatorios_diarios_usuarios(data);
	`)
	if err != nil {
		return err
	}

	// Migração para bancos já existentes. Se a coluna já existe, o erro é ignorado.
	_, _ = db.Exec("ALTER TABLE produtos ADD COLUMN categoria TEXT NOT NULL DEFAULT 'Geral'")

	_, err = db.Exec(`
		INSERT OR IGNORE INTO configuracoes (chave, valor) VALUES
			('pix_chave', ?),
			('pix_nome', ?),
			('pix_cidade', ?)
	`, pixKeyPadrao, pixNomePadrao, pixCidadePadrao)

	return err
}

func criarAdminInicial() error {
	var quantidade int
	if err := db.QueryRow("SELECT COUNT(*) FROM usuarios WHERE perfil='admin'").Scan(&quantidade); err != nil {
		return err
	}
	if quantidade > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		"INSERT INTO usuarios (nome, login, senha_hash, perfil, ativo) VALUES (?, ?, ?, 'admin', 1)",
		"Administrador",
		"admin",
		string(hash),
	)
	if err == nil {
		fmt.Println("Administrador inicial: admin / admin123")
	}
	return err
}

func responderErro(c *gin.Context, status int, mensagem string) {
	c.JSON(status, gin.H{"erro": mensagem})
}

func parametroID(c *gin.Context, nome string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(nome), 10, 64)
	if err != nil || id < 1 {
		responderErro(c, http.StatusBadRequest, "Identificador inválido")
		return 0, false
	}
	return id, true
}

func login(c *gin.Context) {
	var entrada struct {
		Login string `json:"login"`
		Senha string `json:"senha"`
	}
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	entrada.Login = strings.TrimSpace(entrada.Login)
	var usuario Usuario
	var hash string
	var ativo int
	err := db.QueryRow(`
		SELECT id, nome, login, senha_hash, perfil, ativo
		FROM usuarios
		WHERE login=?
	`, entrada.Login).Scan(
		&usuario.ID,
		&usuario.Nome,
		&usuario.Login,
		&hash,
		&usuario.Perfil,
		&ativo,
	)
	if err != nil || ativo == 0 ||
		bcrypt.CompareHashAndPassword([]byte(hash), []byte(entrada.Senha)) != nil {
		responderErro(c, http.StatusUnauthorized, "Login ou senha inválidos")
		return
	}
	usuario.Ativo = true

	claims := jwt.MapClaims{
		"id":     usuario.ID,
		"perfil": usuario.Perfil,
		"exp":    time.Now().Add(12 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, "Erro ao criar sessão")
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString, "usuario": usuario})
}

func autenticarToken(tokenString string) (int64, string, error) {
	token, err := jwt.Parse(
		strings.TrimSpace(tokenString),
		func(token *jwt.Token) (interface{}, error) {
			if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, fmt.Errorf("método de assinatura inválido")
			}
			return jwtSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !token.Valid {
		return 0, "", fmt.Errorf("sessão inválida")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	idNumero, okID := claims["id"].(float64)
	if !ok || !okID {
		return 0, "", fmt.Errorf("sessão inválida")
	}

	id := int64(idNumero)
	var perfil string
	var ativo int
	if err := db.QueryRow(
		"SELECT perfil, ativo FROM usuarios WHERE id=?",
		id,
	).Scan(&perfil, &ativo); err != nil || ativo == 0 {
		return 0, "", fmt.Errorf("usuário inativo")
	}
	return id, perfil, nil
}

func conectarWebSocket(c *gin.Context) {
	var tokenString string
	var clienteID string
	for _, protocolo := range websocket.Subprotocols(c.Request) {
		if strings.HasPrefix(protocolo, "jwt.") {
			tokenString = strings.TrimPrefix(protocolo, "jwt.")
		}
		if strings.HasPrefix(protocolo, "cliente.") {
			clienteID = strings.TrimPrefix(protocolo, "cliente.")
		}
	}
	if tokenString == "" {
		responderErro(c, http.StatusUnauthorized, "Não autenticado")
		return
	}
	if _, _, err := autenticarToken(tokenString); err != nil {
		responderErro(c, http.StatusUnauthorized, err.Error())
		return
	}

	conexao, err := atualizadorWebSocket.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	cliente := &clienteWebSocket{
		conexao: conexao,
		envio:   make(chan eventoTempoReal, 16),
		id:      clienteID,
	}
	tempoReal.entrar <- cliente
	go cliente.escrever()
	cliente.ler()
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"erro": "Não autenticado"})
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		id, perfil, err := autenticarToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"erro": err.Error()})
			return
		}

		c.Set("usuario_id", id)
		c.Set("perfil", perfil)
		c.Next()
	}
}

func adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("perfil") != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"erro": "Somente administrador"})
			return
		}
		c.Next()
	}
}

func listarUsuarios(c *gin.Context) {
	rows, err := db.Query("SELECT id, nome, login, perfil, ativo FROM usuarios ORDER BY nome")
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	usuarios := []Usuario{}
	for rows.Next() {
		var usuario Usuario
		var ativo int
		if err := rows.Scan(&usuario.ID, &usuario.Nome, &usuario.Login, &usuario.Perfil, &ativo); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		usuario.Ativo = ativo == 1
		usuarios = append(usuarios, usuario)
	}
	c.JSON(http.StatusOK, usuarios)
}

type usuarioEntrada struct {
	Nome   string `json:"nome"`
	Login  string `json:"login"`
	Senha  string `json:"senha"`
	Perfil string `json:"perfil"`
	Ativo  *bool  `json:"ativo"`
}

func validarUsuarioEntrada(entrada *usuarioEntrada, senhaObrigatoria bool) string {
	entrada.Nome = strings.TrimSpace(entrada.Nome)
	entrada.Login = strings.TrimSpace(entrada.Login)
	entrada.Perfil = strings.TrimSpace(entrada.Perfil)
	if entrada.Nome == "" || entrada.Login == "" {
		return "Nome e login são obrigatórios"
	}
	if senhaObrigatoria && len(entrada.Senha) < 4 {
		return "A senha deve ter pelo menos 4 caracteres"
	}
	if entrada.Senha != "" && len(entrada.Senha) < 4 {
		return "A senha deve ter pelo menos 4 caracteres"
	}
	if entrada.Perfil != "admin" && entrada.Perfil != "usuario" {
		return "Perfil inválido"
	}
	return ""
}

func criarUsuario(c *gin.Context) {
	var entrada usuarioEntrada
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if mensagem := validarUsuarioEntrada(&entrada, true); mensagem != "" {
		responderErro(c, http.StatusBadRequest, mensagem)
		return
	}

	ativo := true
	if entrada.Ativo != nil {
		ativo = *entrada.Ativo
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(entrada.Senha), bcrypt.DefaultCost)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, "Erro ao proteger a senha")
		return
	}

	resultado, err := db.Exec(`
		INSERT INTO usuarios (nome, login, senha_hash, perfil, ativo)
		VALUES (?, ?, ?, ?, ?)
	`, entrada.Nome, entrada.Login, string(hash), entrada.Perfil, ativo)
	if err != nil {
		responderErro(c, http.StatusBadRequest, "Este login já está cadastrado")
		return
	}

	id, _ := resultado.LastInsertId()
	notificarTempoReal(c, 0, "usuarios", "caixas")
	c.JSON(http.StatusCreated, Usuario{
		ID: id, Nome: entrada.Nome, Login: entrada.Login,
		Perfil: entrada.Perfil, Ativo: ativo,
	})
}

func editarUsuario(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok {
		return
	}

	var entrada usuarioEntrada
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if mensagem := validarUsuarioEntrada(&entrada, false); mensagem != "" {
		responderErro(c, http.StatusBadRequest, mensagem)
		return
	}

	var perfilAtual string
	var ativoAtual int
	if err := db.QueryRow("SELECT perfil, ativo FROM usuarios WHERE id=?", id).Scan(&perfilAtual, &ativoAtual); err != nil {
		responderErro(c, http.StatusNotFound, "Usuário não encontrado")
		return
	}

	ativo := ativoAtual == 1
	if entrada.Ativo != nil {
		ativo = *entrada.Ativo
	}
	if id == c.GetInt64("usuario_id") && (entrada.Perfil != "admin" || !ativo) {
		responderErro(c, http.StatusBadRequest, "Seu próprio acesso deve continuar como administrador ativo")
		return
	}
	if perfilAtual == "admin" && ativoAtual == 1 && (entrada.Perfil != "admin" || !ativo) {
		var outrosAdmins int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM usuarios WHERE perfil='admin' AND ativo=1 AND id<>?",
			id,
		).Scan(&outrosAdmins); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		if outrosAdmins == 0 {
			responderErro(c, http.StatusBadRequest, "É necessário manter pelo menos um administrador ativo")
			return
		}
	}

	var err error
	if entrada.Senha == "" {
		_, err = db.Exec(
			"UPDATE usuarios SET nome=?, login=?, perfil=?, ativo=? WHERE id=?",
			entrada.Nome, entrada.Login, entrada.Perfil, ativo, id,
		)
	} else {
		var hash []byte
		hash, err = bcrypt.GenerateFromPassword([]byte(entrada.Senha), bcrypt.DefaultCost)
		if err == nil {
			_, err = db.Exec(
				"UPDATE usuarios SET nome=?, login=?, senha_hash=?, perfil=?, ativo=? WHERE id=?",
				entrada.Nome, entrada.Login, string(hash), entrada.Perfil, ativo, id,
			)
		}
	}
	if err != nil {
		responderErro(c, http.StatusBadRequest, "Este login já está cadastrado")
		return
	}

	notificarTempoReal(c, 0, "usuarios", "caixas")
	c.JSON(http.StatusOK, Usuario{
		ID: id, Nome: entrada.Nome, Login: entrada.Login,
		Perfil: entrada.Perfil, Ativo: ativo,
	})
}

func listarProdutos(c *gin.Context) {
	query := "SELECT id, nome, COALESCE(categoria, 'Geral'), preco, ativo FROM produtos"
	if c.GetString("perfil") != "admin" {
		query += " WHERE ativo=1"
	}
	query += " ORDER BY categoria, nome"

	rows, err := db.Query(query)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	produtos := []Produto{}
	for rows.Next() {
		var produto Produto
		var ativo int
		if err := rows.Scan(&produto.ID, &produto.Nome, &produto.Categoria, &produto.Preco, &ativo); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		produto.Ativo = ativo == 1
		produtos = append(produtos, produto)
	}
	c.JSON(http.StatusOK, produtos)
}

type produtoEntrada struct {
	Nome      string  `json:"nome"`
	Categoria string  `json:"categoria"`
	Preco     float64 `json:"preco"`
	Ativo     *bool   `json:"ativo"`
}

func validarProduto(entrada *produtoEntrada) string {
	entrada.Nome = strings.TrimSpace(entrada.Nome)
	entrada.Categoria = strings.TrimSpace(entrada.Categoria)
	if entrada.Nome == "" {
		return "Informe o nome do produto"
	}
	if entrada.Categoria == "" {
		entrada.Categoria = "Geral"
	}
	if entrada.Preco < 0 {
		return "O preço não pode ser negativo"
	}
	return ""
}

func criarProduto(c *gin.Context) {
	var entrada produtoEntrada
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if mensagem := validarProduto(&entrada); mensagem != "" {
		responderErro(c, http.StatusBadRequest, mensagem)
		return
	}

	ativo := true
	if entrada.Ativo != nil {
		ativo = *entrada.Ativo
	}
	resultado, err := db.Exec(
		"INSERT INTO produtos (nome, categoria, preco, ativo) VALUES (?, ?, ?, ?)",
		entrada.Nome, entrada.Categoria, entrada.Preco, ativo,
	)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := resultado.LastInsertId()
	notificarTempoReal(c, 0, "produtos")
	c.JSON(http.StatusCreated, Produto{ID: id, Nome: entrada.Nome, Categoria: entrada.Categoria, Preco: entrada.Preco, Ativo: ativo})
}

func editarProduto(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok {
		return
	}
	var entrada produtoEntrada
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if mensagem := validarProduto(&entrada); mensagem != "" {
		responderErro(c, http.StatusBadRequest, mensagem)
		return
	}

	ativo := true
	if entrada.Ativo != nil {
		ativo = *entrada.Ativo
	}
	resultado, err := db.Exec(
		"UPDATE produtos SET nome=?, categoria=?, preco=?, ativo=? WHERE id=?",
		entrada.Nome, entrada.Categoria, entrada.Preco, ativo, id,
	)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	linhas, _ := resultado.RowsAffected()
	if linhas == 0 {
		responderErro(c, http.StatusNotFound, "Produto não encontrado")
		return
	}
	notificarTempoReal(c, 0, "produtos")
	c.JSON(http.StatusOK, Produto{ID: id, Nome: entrada.Nome, Categoria: entrada.Categoria, Preco: entrada.Preco, Ativo: ativo})
}

func lerConfiguracao(chave, padrao string) string {
	var valor string
	if err := db.QueryRow("SELECT valor FROM configuracoes WHERE chave=?", chave).Scan(&valor); err != nil {
		return padrao
	}
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return padrao
	}
	return valor
}

func configuracaoPix() (string, string, string) {
	return lerConfiguracao("pix_chave", pixKeyPadrao),
		lerConfiguracao("pix_nome", pixNomePadrao),
		lerConfiguracao("pix_cidade", pixCidadePadrao)
}

func buscarConfiguracoes(c *gin.Context) {
	chave, nome, cidade := configuracaoPix()
	c.JSON(http.StatusOK, gin.H{
		"pix_chave":  chave,
		"pix_nome":   nome,
		"pix_cidade": cidade,
	})
}

func salvarConfiguracoes(c *gin.Context) {
	var entrada struct {
		PixChave  string `json:"pix_chave"`
		PixNome   string `json:"pix_nome"`
		PixCidade string `json:"pix_cidade"`
	}
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}

	entrada.PixChave = strings.TrimSpace(entrada.PixChave)
	entrada.PixNome = strings.TrimSpace(entrada.PixNome)
	entrada.PixCidade = strings.TrimSpace(entrada.PixCidade)
	if entrada.PixChave == "" {
		responderErro(c, http.StatusBadRequest, "Informe a chave Pix")
		return
	}
	if entrada.PixNome == "" {
		responderErro(c, http.StatusBadRequest, "Informe o nome do recebedor")
		return
	}
	if entrada.PixCidade == "" {
		responderErro(c, http.StatusBadRequest, "Informe a cidade")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	configs := map[string]string{
		"pix_chave":  entrada.PixChave,
		"pix_nome":   entrada.PixNome,
		"pix_cidade": entrada.PixCidade,
	}
	for chave, valor := range configs {
		_, err = tx.Exec(`
			INSERT INTO configuracoes (chave, valor) VALUES (?, ?)
			ON CONFLICT(chave) DO UPDATE SET valor=excluded.valor
		`, chave, valor)
		if err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err = tx.Commit(); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}

	notificarTempoReal(c, 0, "configuracoes")
	c.JSON(http.StatusOK, gin.H{
		"pix_chave":  entrada.PixChave,
		"pix_nome":   entrada.PixNome,
		"pix_cidade": entrada.PixCidade,
	})
}

func autorizarComanda(c *gin.Context, id int64, exigirAberta bool) bool {
	var donoID int64
	var status string
	err := db.QueryRow("SELECT usuario_id, status FROM comandas WHERE id=?", id).Scan(&donoID, &status)
	if err != nil {
		responderErro(c, http.StatusNotFound, "Comanda não encontrada")
		return false
	}
	if c.GetString("perfil") != "admin" && donoID != c.GetInt64("usuario_id") {
		responderErro(c, http.StatusForbidden, "Você não pode acessar esta comanda")
		return false
	}
	if exigirAberta && status != "aberta" {
		responderErro(c, http.StatusConflict, "Esta comanda já foi fechada")
		return false
	}
	return true
}

func autorizarItem(c *gin.Context, id int64) (int64, bool) {
	var comandaID int64
	var donoID int64
	var status string
	err := db.QueryRow(`
		SELECT i.comanda_id, c.usuario_id, c.status
		FROM itens i
		JOIN comandas c ON c.id=i.comanda_id
		WHERE i.id=?
	`, id).Scan(&comandaID, &donoID, &status)
	if err != nil {
		responderErro(c, http.StatusNotFound, "Item não encontrado")
		return 0, false
	}
	if c.GetString("perfil") != "admin" && donoID != c.GetInt64("usuario_id") {
		responderErro(c, http.StatusForbidden, "Você não pode alterar este item")
		return 0, false
	}
	if status != "aberta" {
		responderErro(c, http.StatusConflict, "Esta comanda já foi fechada")
		return 0, false
	}
	return comandaID, true
}

func criarComanda(c *gin.Context) {
	var entrada struct {
		Mesa string `json:"mesa"`
	}
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	entrada.Mesa = strings.TrimSpace(entrada.Mesa)
	if entrada.Mesa == "" {
		responderErro(c, http.StatusBadRequest, "Informe a mesa ou identificação")
		return
	}
	if len([]rune(entrada.Mesa)) > 60 {
		responderErro(c, http.StatusBadRequest, "A identificação é muito longa")
		return
	}

	resultado, err := db.Exec(
		"INSERT INTO comandas (mesa, usuario_id) VALUES (?, ?)",
		entrada.Mesa,
		c.GetInt64("usuario_id"),
	)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	id, _ := resultado.LastInsertId()
	notificarTempoReal(c, id, "comandas")
	c.JSON(http.StatusCreated, gin.H{"id": id, "mesa": entrada.Mesa, "status": "aberta"})
}

func listarComandas(c *gin.Context) {
	query := `
		SELECT
			c.id,
			c.mesa,
			c.status,
			c.usuario_id,
			u.nome,
			COALESCE(c.forma_pagamento, ''),
			COALESCE(c.criada_em, ''),
			CASE
				WHEN c.status='fechada' THEN c.valor_pago
				ELSE COALESCE(SUM(i.quantidade*i.preco), 0)
			END
		FROM comandas c
		JOIN usuarios u ON u.id=c.usuario_id
		LEFT JOIN itens i ON i.comanda_id=c.id
	`
	args := []interface{}{}
	if c.GetString("perfil") != "admin" {
		query += " WHERE c.usuario_id=?"
		args = append(args, c.GetInt64("usuario_id"))
	}
	query += " GROUP BY c.id ORDER BY c.id DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	comandas := []Comanda{}
	for rows.Next() {
		var comanda Comanda
		if err := rows.Scan(
			&comanda.ID,
			&comanda.Mesa,
			&comanda.Status,
			&comanda.UsuarioID,
			&comanda.Usuario,
			&comanda.FormaPagamento,
			&comanda.CriadaEm,
			&comanda.Total,
		); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		comandas = append(comandas, comanda)
	}
	c.JSON(http.StatusOK, comandas)
}

func buscarComanda(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok || !autorizarComanda(c, id, false) {
		return
	}

	var comanda Comanda
	err := db.QueryRow(`
		SELECT
			c.id,
			c.mesa,
			c.status,
			c.usuario_id,
			u.nome,
			COALESCE(c.forma_pagamento, ''),
			COALESCE(c.criada_em, ''),
			CASE
				WHEN c.status='fechada' THEN c.valor_pago
				ELSE COALESCE(SUM(i.quantidade*i.preco), 0)
			END
		FROM comandas c
		JOIN usuarios u ON u.id=c.usuario_id
		LEFT JOIN itens i ON i.comanda_id=c.id
		WHERE c.id=?
		GROUP BY c.id
	`, id).Scan(
		&comanda.ID,
		&comanda.Mesa,
		&comanda.Status,
		&comanda.UsuarioID,
		&comanda.Usuario,
		&comanda.FormaPagamento,
		&comanda.CriadaEm,
		&comanda.Total,
	)
	if err != nil {
		responderErro(c, http.StatusNotFound, "Comanda não encontrada")
		return
	}
	c.JSON(http.StatusOK, comanda)
}

func excluirComanda(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok {
		return
	}

	tx, err := db.Begin()
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var dataFechamento string
	var fechamentoID sql.NullInt64
	err = tx.QueryRow(`
		SELECT
			COALESCE(strftime('%Y-%m-%d', fechada_em, '-3 hours'), ''),
			fechamento_id
		FROM comandas
		WHERE id=?
	`, id).Scan(&dataFechamento, &fechamentoID)
	if err != nil {
		responderErro(c, http.StatusNotFound, "Comanda não encontrada")
		return
	}

	if _, err = tx.Exec("DELETE FROM itens WHERE comanda_id=?", id); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	if _, err = tx.Exec("DELETE FROM comandas WHERE id=?", id); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}

	if fechamentoID.Valid {
		var dinheiro, cartao, pix, total float64
		err = tx.QueryRow(`
			SELECT
				COALESCE(SUM(CASE WHEN forma_pagamento='dinheiro' THEN valor_pago ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN forma_pagamento='cartao' THEN valor_pago ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN forma_pagamento='pix' THEN valor_pago ELSE 0 END), 0),
				COALESCE(SUM(valor_pago), 0)
			FROM comandas
			WHERE fechamento_id=?
		`, fechamentoID.Int64).Scan(&dinheiro, &cartao, &pix, &total)
		if err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		if total == 0 {
			_, err = tx.Exec("DELETE FROM fechamentos_caixa WHERE id=?", fechamentoID.Int64)
		} else {
			_, err = tx.Exec(`
				UPDATE fechamentos_caixa
				SET total_dinheiro=?, total_cartao=?, total_pix=?, total_geral=?
				WHERE id=?
			`, dinheiro, cartao, pix, total, fechamentoID.Int64)
		}
		if err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if err = tx.Commit(); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	if dataFechamento != "" {
		if err := salvarRelatorioDia(dataFechamento); err != nil {
			fmt.Println("Aviso ao atualizar relatório após exclusão:", err)
		}
	}
	notificarTempoReal(c, id, "comandas", "itens", "caixas", "relatorios")
	c.JSON(http.StatusOK, gin.H{"mensagem": "Comanda excluída"})
}

func adicionarItem(c *gin.Context) {
	comandaID, ok := parametroID(c, "id")
	if !ok || !autorizarComanda(c, comandaID, true) {
		return
	}
	var entrada struct {
		ProdutoID  int64 `json:"produto_id"`
		Quantidade int   `json:"quantidade"`
	}
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Dados inválidos")
		return
	}
	if entrada.Quantidade < 1 {
		entrada.Quantidade = 1
	}

	var preco float64
	if err := db.QueryRow(
		"SELECT preco FROM produtos WHERE id=? AND ativo=1",
		entrada.ProdutoID,
	).Scan(&preco); err != nil {
		responderErro(c, http.StatusNotFound, "Produto não encontrado ou inativo")
		return
	}

	var itemID int64
	var quantidadeAtual int
	err := db.QueryRow(
		"SELECT id, quantidade FROM itens WHERE comanda_id=? AND produto_id=? LIMIT 1",
		comandaID,
		entrada.ProdutoID,
	).Scan(&itemID, &quantidadeAtual)
	if err == nil {
		_, err = db.Exec(
			"UPDATE itens SET quantidade=? WHERE id=?",
			quantidadeAtual+entrada.Quantidade,
			itemID,
		)
	} else if err == sql.ErrNoRows {
		_, err = db.Exec(`
			INSERT INTO itens (comanda_id, produto_id, quantidade, preco)
			VALUES (?, ?, ?, ?)
		`, comandaID, entrada.ProdutoID, entrada.Quantidade, preco)
	}
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	notificarTempoReal(c, comandaID, "comandas", "itens")
	c.JSON(http.StatusOK, gin.H{"mensagem": "Produto adicionado"})
}

func listarItens(c *gin.Context) {
	comandaID, ok := parametroID(c, "id")
	if !ok || !autorizarComanda(c, comandaID, false) {
		return
	}
	rows, err := db.Query(`
		SELECT
			i.id,
			i.produto_id,
			p.nome,
			i.quantidade,
			i.preco,
			i.quantidade*i.preco
		FROM itens i
		JOIN produtos p ON p.id=i.produto_id
		WHERE i.comanda_id=?
		ORDER BY i.id DESC
	`, comandaID)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	itens := []Item{}
	for rows.Next() {
		var item Item
		if err := rows.Scan(
			&item.ID,
			&item.ProdutoID,
			&item.Nome,
			&item.Quantidade,
			&item.Preco,
			&item.Subtotal,
		); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		itens = append(itens, item)
	}
	c.JSON(http.StatusOK, itens)
}

func atualizarQuantidadeItem(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok {
		return
	}
	comandaID, autorizado := autorizarItem(c, id)
	if !autorizado {
		return
	}
	var entrada struct {
		Quantidade int `json:"quantidade"`
	}
	if err := c.ShouldBindJSON(&entrada); err != nil || entrada.Quantidade < 1 {
		responderErro(c, http.StatusBadRequest, "Quantidade inválida")
		return
	}
	if _, err := db.Exec("UPDATE itens SET quantidade=? WHERE id=?", entrada.Quantidade, id); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	notificarTempoReal(c, comandaID, "comandas", "itens")
	c.JSON(http.StatusOK, gin.H{"quantidade": entrada.Quantidade})
}

func excluirItem(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok {
		return
	}
	comandaID, autorizado := autorizarItem(c, id)
	if !autorizado {
		return
	}
	if _, err := db.Exec("DELETE FROM itens WHERE id=?", id); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	notificarTempoReal(c, comandaID, "comandas", "itens")
	c.JSON(http.StatusOK, gin.H{"mensagem": "Item removido"})
}

func totalComanda(id int64) (float64, error) {
	var total float64
	err := db.QueryRow(
		"SELECT COALESCE(SUM(quantidade*preco), 0) FROM itens WHERE comanda_id=?",
		id,
	).Scan(&total)
	return total, err
}

func pagarComanda(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok || !autorizarComanda(c, id, true) {
		return
	}
	var entrada struct {
		Forma string `json:"forma"`
	}
	if err := c.ShouldBindJSON(&entrada); err != nil {
		responderErro(c, http.StatusBadRequest, "Pagamento inválido")
		return
	}

	total, err := totalComanda(id)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	if total <= 0 {
		responderErro(c, http.StatusBadRequest, "A comanda está vazia")
		return
	}

	if entrada.Forma == "pix" {
		chavePix, nomePix, cidadePix := configuracaoPix()
		c.JSON(http.StatusOK, gin.H{
			"pix":        gerarPix(chavePix, nomePix, cidadePix, total, "CMD"+strconv.FormatInt(id, 10)),
			"chave":      chavePix,
			"total":      total,
			"aguardando": true,
		})
		return
	}
	if entrada.Forma != "dinheiro" && entrada.Forma != "cartao" {
		responderErro(c, http.StatusBadRequest, "Forma de pagamento inválida")
		return
	}

	resultado, err := db.Exec(`
		UPDATE comandas
		SET status='fechada', forma_pagamento=?, valor_pago=?, fechada_em=CURRENT_TIMESTAMP
		WHERE id=? AND status='aberta'
	`, entrada.Forma, total, id)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	linhas, _ := resultado.RowsAffected()
	if linhas == 0 {
		responderErro(c, http.StatusConflict, "Esta comanda já foi fechada")
		return
	}
	if err := salvarRelatorioDia(dataLocalAtual()); err != nil {
		fmt.Println("Aviso ao atualizar relatório:", err)
	}
	notificarTempoReal(c, id, "comandas", "caixas", "relatorios")
	c.JSON(http.StatusOK, gin.H{"mensagem": "Pagamento realizado", "forma": entrada.Forma, "total": total})
}

func confirmarPix(c *gin.Context) {
	id, ok := parametroID(c, "id")
	if !ok || !autorizarComanda(c, id, true) {
		return
	}
	total, err := totalComanda(id)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	if total <= 0 {
		responderErro(c, http.StatusBadRequest, "A comanda está vazia")
		return
	}

	resultado, err := db.Exec(`
		UPDATE comandas
		SET status='fechada', forma_pagamento='pix', valor_pago=?, fechada_em=CURRENT_TIMESTAMP
		WHERE id=? AND status='aberta'
	`, total, id)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	linhas, _ := resultado.RowsAffected()
	if linhas == 0 {
		responderErro(c, http.StatusConflict, "Esta comanda já foi fechada")
		return
	}
	if err := salvarRelatorioDia(dataLocalAtual()); err != nil {
		fmt.Println("Aviso ao atualizar relatório:", err)
	}
	notificarTempoReal(c, id, "comandas", "caixas", "relatorios")
	c.JSON(http.StatusOK, gin.H{"mensagem": "Pix confirmado", "total": total})
}

func listarCaixas(c *gin.Context) {
	rows, err := db.Query(`
		SELECT
			u.id,
			u.nome,
			u.ativo,
			COALESCE(SUM(CASE WHEN c.forma_pagamento='dinheiro' THEN c.valor_pago ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.forma_pagamento='cartao' THEN c.valor_pago ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.forma_pagamento='pix' THEN c.valor_pago ELSE 0 END), 0),
			COALESCE(SUM(c.valor_pago), 0)
		FROM usuarios u
		LEFT JOIN comandas c
			ON c.usuario_id=u.id
			AND c.status='fechada'
			AND c.fechamento_id IS NULL
		WHERE u.perfil='usuario'
		GROUP BY u.id, u.nome, u.ativo
		ORDER BY u.nome
	`)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	caixas := []gin.H{}
	for rows.Next() {
		var id int64
		var nome string
		var ativo int
		var dinheiro, cartao, pix, total float64
		if err := rows.Scan(&id, &nome, &ativo, &dinheiro, &cartao, &pix, &total); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		caixas = append(caixas, gin.H{
			"usuario_id": id,
			"usuario":    nome,
			"ativo":      ativo == 1,
			"dinheiro":   dinheiro,
			"cartao":     cartao,
			"pix":        pix,
			"total":      total,
		})
	}
	c.JSON(http.StatusOK, caixas)
}

func fecharCaixa(c *gin.Context) {
	usuarioID, ok := parametroID(c, "id")
	if !ok {
		return
	}
	var existe int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM usuarios WHERE id=? AND perfil='usuario'",
		usuarioID,
	).Scan(&existe); err != nil || existe == 0 {
		responderErro(c, http.StatusNotFound, "Usuário não encontrado")
		return
	}

	tx, err := db.Begin()
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer tx.Rollback()

	var dinheiro, cartao, pix float64
	err = tx.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN forma_pagamento='dinheiro' THEN valor_pago ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN forma_pagamento='cartao' THEN valor_pago ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN forma_pagamento='pix' THEN valor_pago ELSE 0 END), 0)
		FROM comandas
		WHERE usuario_id=? AND status='fechada' AND fechamento_id IS NULL
	`, usuarioID).Scan(&dinheiro, &cartao, &pix)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	total := dinheiro + cartao + pix
	if total <= 0 {
		responderErro(c, http.StatusBadRequest, "Não existem valores para fechar")
		return
	}

	resultado, err := tx.Exec(`
		INSERT INTO fechamentos_caixa (
			usuario_id, admin_id, total_dinheiro, total_cartao, total_pix, total_geral
		) VALUES (?, ?, ?, ?, ?, ?)
	`, usuarioID, c.GetInt64("usuario_id"), dinheiro, cartao, pix, total)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	fechamentoID, _ := resultado.LastInsertId()
	if _, err = tx.Exec(`
		UPDATE comandas
		SET fechamento_id=?
		WHERE usuario_id=? AND status='fechada' AND fechamento_id IS NULL
	`, fechamentoID, usuarioID); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err = tx.Commit(); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	notificarTempoReal(c, 0, "caixas")
	c.JSON(http.StatusOK, gin.H{
		"mensagem": "Caixa fechado",
		"dinheiro": dinheiro,
		"cartao":   cartao,
		"pix":      pix,
		"total":    total,
	})
}

func listarFechamentos(c *gin.Context) {
	usuarioID, ok := parametroID(c, "id")
	if !ok {
		return
	}
	rows, err := db.Query(`
		SELECT id, total_dinheiro, total_cartao, total_pix, total_geral, criado_em
		FROM fechamentos_caixa
		WHERE usuario_id=?
		ORDER BY id DESC
	`, usuarioID)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	fechamentos := []gin.H{}
	for rows.Next() {
		var id int64
		var dinheiro, cartao, pix, total float64
		var data string
		if err := rows.Scan(&id, &dinheiro, &cartao, &pix, &total, &data); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		fechamentos = append(fechamentos, gin.H{
			"id": id, "dinheiro": dinheiro, "cartao": cartao,
			"pix": pix, "total": total, "data": data,
		})
	}
	c.JSON(http.StatusOK, fechamentos)
}

func dataLocalAtual() string {
	return time.Now().In(fusoLocal).Format("2006-01-02")
}

func intervaloDataRelatorio(data string) (string, string, string, error) {
	data = strings.TrimSpace(data)
	if data == "" {
		data = dataLocalAtual()
	}
	dia, err := time.ParseInLocation("2006-01-02", data, fusoLocal)
	if err != nil {
		return "", "", "", fmt.Errorf("data inválida. Use AAAA-MM-DD")
	}
	inicio := dia.UTC().Format("2006-01-02 15:04:05")
	fim := dia.AddDate(0, 0, 1).UTC().Format("2006-01-02 15:04:05")
	return data, inicio, fim, nil
}

func salvarRelatorioDia(data string) error {
	data, inicio, fim, err := intervaloDataRelatorio(data)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT
			u.id,
			u.nome,
			COUNT(c.id),
			COALESCE(SUM(CASE WHEN c.forma_pagamento='dinheiro' THEN c.valor_pago ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.forma_pagamento='cartao' THEN c.valor_pago ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN c.forma_pagamento='pix' THEN c.valor_pago ELSE 0 END), 0),
			COALESCE(SUM(c.valor_pago), 0)
		FROM comandas c
		JOIN usuarios u ON u.id=c.usuario_id
		WHERE c.status='fechada' AND c.fechada_em>=? AND c.fechada_em<?
		GROUP BY u.id, u.nome
		ORDER BY u.nome
	`, inicio, fim)
	if err != nil {
		return err
	}

	usuarios := []RelatorioUsuario{}
	for rows.Next() {
		var item RelatorioUsuario
		if err := rows.Scan(
			&item.UsuarioID,
			&item.Usuario,
			&item.QuantidadeComandas,
			&item.Dinheiro,
			&item.Cartao,
			&item.Pix,
			&item.Total,
		); err != nil {
			rows.Close()
			return err
		}
		usuarios = append(usuarios, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	var quantidade int
	var dinheiro, cartao, pix, total float64
	for _, item := range usuarios {
		quantidade += item.QuantidadeComandas
		dinheiro += item.Dinheiro
		cartao += item.Cartao
		pix += item.Pix
		total += item.Total
	}

	_, err = tx.Exec(`
		INSERT INTO relatorios_diarios (
			data, quantidade_comandas, total_dinheiro, total_cartao,
			total_pix, total_geral, atualizado_em
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(data) DO UPDATE SET
			quantidade_comandas=excluded.quantidade_comandas,
			total_dinheiro=excluded.total_dinheiro,
			total_cartao=excluded.total_cartao,
			total_pix=excluded.total_pix,
			total_geral=excluded.total_geral,
			atualizado_em=CURRENT_TIMESTAMP
	`, data, quantidade, dinheiro, cartao, pix, total)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM relatorios_diarios_usuarios WHERE data=?", data); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT INTO relatorios_diarios_usuarios (
			data, usuario_id, usuario, quantidade_comandas,
			total_dinheiro, total_cartao, total_pix, total_geral
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	for _, item := range usuarios {
		if _, err = stmt.Exec(
			data,
			item.UsuarioID,
			item.Usuario,
			item.QuantidadeComandas,
			item.Dinheiro,
			item.Cartao,
			item.Pix,
			item.Total,
		); err != nil {
			stmt.Close()
			return err
		}
	}
	if err = stmt.Close(); err != nil {
		return err
	}
	return tx.Commit()
}

func sincronizarRelatoriosExistentes() error {
	rows, err := db.Query(`
		SELECT DISTINCT strftime('%Y-%m-%d', fechada_em, '-3 hours')
		FROM comandas
		WHERE status='fechada' AND fechada_em IS NOT NULL
		ORDER BY 1
	`)
	if err != nil {
		return err
	}
	datas := []string{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			rows.Close()
			return err
		}
		datas = append(datas, data)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, data := range datas {
		var quantidade int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM relatorios_diarios WHERE data=?",
			data,
		).Scan(&quantidade); err != nil {
			return err
		}
		if quantidade == 0 {
			if err := salvarRelatorioDia(data); err != nil {
				return err
			}
		}
	}
	return nil
}

func lerRelatorioDia(data string) (RelatorioDia, error) {
	relatorio := RelatorioDia{Data: data, Usuarios: []RelatorioUsuario{}}
	err := db.QueryRow(`
		SELECT
			quantidade_comandas,
			total_dinheiro,
			total_cartao,
			total_pix,
			total_geral,
			atualizado_em
		FROM relatorios_diarios
		WHERE data=?
	`, data).Scan(
		&relatorio.QuantidadeComandas,
		&relatorio.Dinheiro,
		&relatorio.Cartao,
		&relatorio.Pix,
		&relatorio.Total,
		&relatorio.AtualizadoEm,
	)
	if err != nil {
		return relatorio, err
	}

	rows, err := db.Query(`
		SELECT
			usuario_id,
			usuario,
			quantidade_comandas,
			total_dinheiro,
			total_cartao,
			total_pix,
			total_geral
		FROM relatorios_diarios_usuarios
		WHERE data=?
		ORDER BY usuario
	`, data)
	if err != nil {
		return relatorio, err
	}
	defer rows.Close()

	for rows.Next() {
		var item RelatorioUsuario
		if err := rows.Scan(
			&item.UsuarioID,
			&item.Usuario,
			&item.QuantidadeComandas,
			&item.Dinheiro,
			&item.Cartao,
			&item.Pix,
			&item.Total,
		); err != nil {
			return relatorio, err
		}
		relatorio.Usuarios = append(relatorio.Usuarios, item)
	}
	return relatorio, rows.Err()
}

func buscarRelatorioDia(c *gin.Context) {
	data, _, _, err := intervaloDataRelatorio(c.Query("data"))
	if err != nil {
		responderErro(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := salvarRelatorioDia(data); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	relatorio, err := lerRelatorioDia(data)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, relatorio)
}

func listarDatasRelatorios(c *gin.Context) {
	rows, err := db.Query(`
		SELECT data, quantidade_comandas, total_geral, atualizado_em
		FROM relatorios_diarios
		ORDER BY data DESC
	`)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	datas := []gin.H{}
	for rows.Next() {
		var data, atualizadoEm string
		var quantidade int
		var total float64
		if err := rows.Scan(&data, &quantidade, &total, &atualizadoEm); err != nil {
			responderErro(c, http.StatusInternalServerError, err.Error())
			return
		}
		datas = append(datas, gin.H{
			"data": data, "quantidade_comandas": quantidade,
			"total": total, "atualizado_em": atualizadoEm,
		})
	}
	c.JSON(http.StatusOK, datas)
}

func relatorioPorUsuario(c *gin.Context) {
	data, _, _, err := intervaloDataRelatorio(c.Query("data"))
	if err != nil {
		responderErro(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := salvarRelatorioDia(data); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	relatorio, err := lerRelatorioDia(data)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "usuarios": relatorio.Usuarios})
}

func relatorioFinalDia(c *gin.Context) {
	data, _, _, err := intervaloDataRelatorio(c.Query("data"))
	if err != nil {
		responderErro(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := salvarRelatorioDia(data); err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	relatorio, err := lerRelatorioDia(data)
	if err != nil {
		responderErro(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": data, "quantidade_comandas": relatorio.QuantidadeComandas,
		"dinheiro": relatorio.Dinheiro, "cartao": relatorio.Cartao,
		"pix": relatorio.Pix, "total": relatorio.Total,
	})
}

func emv(id, valor string) string {
	return fmt.Sprintf("%s%02d%s", id, len(valor), valor)
}

func gerarPix(chave, nome, cidade string, valor float64, txid string) string {
	nome = strings.ToUpper(nome)
	cidade = strings.ToUpper(cidade)
	if len(nome) > 25 {
		nome = nome[:25]
	}
	if len(cidade) > 15 {
		cidade = cidade[:15]
	}
	if len(txid) > 25 {
		txid = txid[:25]
	}

	merchant := emv("00", "br.gov.bcb.pix") + emv("01", chave)
	payload := emv("00", "01") +
		emv("26", merchant) +
		emv("52", "0000") +
		emv("53", "986") +
		emv("54", fmt.Sprintf("%.2f", valor)) +
		emv("58", "BR") +
		emv("59", nome) +
		emv("60", cidade) +
		emv("62", emv("05", txid)) +
		"6304"
	return fmt.Sprintf("%s%04X", payload, crc16(payload))
}

func crc16(data string) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range []byte(data) {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
