import { useEffect, useMemo, useRef, useState } from "react";
import { QRCodeSVG } from "qrcode.react";

const host = window.location.protocol === "wails:"
  ? "127.0.0.1"
  : window.location.hostname || "127.0.0.1";

const API = (import.meta.env.VITE_API_URL || `http://${host}:8080/api`).replace(/\/$/, "");

const WS = (() => {
  const endereco = new URL(`${API}/ws`, window.location.href);
  endereco.protocol = endereco.protocol === "https:" ? "wss:" : "ws:";
  return endereco.toString();
})();

const usuarioNovo = () => ({
  id: null,
  nome: "",
  login: "",
  senha: "",
  perfil: "usuario",
  ativo: true
});

const produtoNovo = () => ({
  id: null,
  nome: "",
  categoria: "Geral",
  preco: "",
  ativo: true
});

function hoje() {
  const data = new Date();
  const ano = data.getFullYear();
  const mes = String(data.getMonth() + 1).padStart(2, "0");
  const dia = String(data.getDate()).padStart(2, "0");
  return `${ano}-${mes}-${dia}`;
}

function moeda(valor) {
  return Number(valor || 0).toLocaleString("pt-BR", {
    style: "currency",
    currency: "BRL"
  });
}

function formatarData(data) {
  if (!data) return "";
  const [ano, mes, dia] = data.slice(0, 10).split("-");
  return `${dia}/${mes}/${ano}`;
}

function lerUsuario() {
  try {
    return JSON.parse(localStorage.getItem("usuario") || "null");
  } catch {
    return null;
  }
}

function Marca({ detalhe }) {
  return (
    <div className="marca">
      <span className="marca-simbolo">C</span>
      <div>
        <strong>Comanda Fácil</strong>
        {detalhe && <small>{detalhe}</small>}
      </div>
    </div>
  );
}

function Modal({ titulo, subtitulo, onClose, children, grande = false }) {
  useEffect(() => {
    const overflow = document.body.style.overflow;
    const fecharComEsc = event => event.key === "Escape" && onClose();
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", fecharComEsc);

    return () => {
      document.body.style.overflow = overflow;
      window.removeEventListener("keydown", fecharComEsc);
    };
  }, [onClose]);

  return (
    <div
      className="modal-fundo"
      onMouseDown={event => event.target === event.currentTarget && onClose()}
    >
      <section
        className={`modal ${grande ? "modal-grande" : ""}`}
        role="dialog"
        aria-modal="true"
        aria-label={titulo}
      >
        <header className="modal-topo">
          <div>
            <h2>{titulo}</h2>
            {subtitulo && <p>{subtitulo}</p>}
          </div>
          <button className="botao-icone" type="button" onClick={onClose} aria-label="Fechar">
            ×
          </button>
        </header>
        <div className="modal-conteudo">{children}</div>
      </section>
    </div>
  );
}

function Cabecalho({ sobre, titulo, texto, acao }) {
  return (
    <header className="pagina-topo">
      <div>
        <span className="sobre">{sobre}</span>
        <h1>{titulo}</h1>
        {texto && <p>{texto}</p>}
      </div>
      {acao}
    </header>
  );
}

function Vazio({ titulo, texto, acao }) {
  return (
    <div className="vazio">
      <span>≡</span>
      <h3>{titulo}</h3>
      <p>{texto}</p>
      {acao}
    </div>
  );
}

function Selo({ children, tipo = "neutro" }) {
  return <span className={`selo selo-${tipo}`}>{children}</span>;
}

const paginasAdmin = [
  ["comandas", "Comandas"],
  ["usuarios", "Usuários"],
  ["produtos", "Produtos"],
  ["configuracoes", "Configurações"],
  ["caixas", "Caixa"],
  ["relatorios", "Relatórios"]
];

export default function App() {
  const [token, setToken] = useState(localStorage.getItem("token") || "");
  const [usuario, setUsuario] = useState(lerUsuario);
  const [clienteID] = useState(() =>
    window.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`
  );

  const [login, setLogin] = useState("");
  const [senha, setSenha] = useState("");
  const [pagina, setPagina] = useState("comandas");

  const [comandas, setComandas] = useState([]);
  const [produtos, setProdutos] = useState([]);
  const [usuarios, setUsuarios] = useState([]);
  const [caixas, setCaixas] = useState([]);
  const [comanda, setComanda] = useState(null);
  const [itens, setItens] = useState([]);

  const [mesa, setMesa] = useState("");
  const [buscaProduto, setBuscaProduto] = useState("");
  const [categoriaProduto, setCategoriaProduto] = useState("todos");
  const [filtroUsuario, setFiltroUsuario] = useState("todos");
  const [filtroTipo, setFiltroTipo] = useState("todos");
  const [pix, setPix] = useState(null);

  const [configuracoes, setConfiguracoes] = useState({
    pix_chave: "",
    pix_nome: "",
    pix_cidade: ""
  });

  const [novaComandaAberta, setNovaComandaAberta] = useState(false);
  const [editorUsuario, setEditorUsuario] = useState(null);
  const [editorProduto, setEditorProduto] = useState(null);
  const [caixaParaFechar, setCaixaParaFechar] = useState(null);
  const [comandaParaExcluir, setComandaParaExcluir] = useState(null);

  const [dataRelatorio, setDataRelatorio] = useState(hoje);
  const [relatorio, setRelatorio] = useState(null);
  const [datasRelatorios, setDatasRelatorios] = useState([]);
  const [ultimaAtualizacao, setUltimaAtualizacao] = useState(null);
  const [aviso, setAviso] = useState(null);
  const [ocupado, setOcupado] = useState(false);
  const [tempoRealAtivo, setTempoRealAtivo] = useState(false);

  const estadoTempoReal = useRef({
    pagina: "comandas",
    comandaID: null,
    relatorioData: null,
    perfil: usuario?.perfil
  });

  function mostrarAviso(texto, tipo = "sucesso") {
    setAviso({ texto, tipo, id: Date.now() });
  }

  function sair() {
    localStorage.removeItem("token");
    localStorage.removeItem("usuario");
    setToken("");
    setUsuario(null);
    setPagina("comandas");
    setComanda(null);
    setItens([]);
    setPix(null);
    setTempoRealAtivo(false);
  }

  async function requisicao(caminho, opcoes = {}) {
    let resposta;

    try {
      resposta = await fetch(`${API}${caminho}`, {
        method: opcoes.method || "GET",
        headers: {
          "Content-Type": "application/json",
          "X-Client-ID": clienteID,
          ...(token && caminho !== "/login"
            ? { Authorization: `Bearer ${token}` }
            : {})
        },
        ...(opcoes.body !== undefined
          ? { body: JSON.stringify(opcoes.body) }
          : {})
      });
    } catch {
      throw new Error("Servidor não encontrado. Confira se os aparelhos estão na mesma rede.");
    }

    let dados = {};
    try {
      dados = await resposta.json();
    } catch {
      dados = {};
    }

    if (resposta.status === 401 && token && caminho !== "/login") {
      sair();
      throw new Error("Sua sessão terminou. Entre novamente.");
    }

    if (!resposta.ok) {
      throw new Error(dados.erro || "Não foi possível concluir a operação.");
    }

    return dados;
  }

  async function entrar(event) {
    event.preventDefault();
    setOcupado(true);

    try {
      const dados = await requisicao("/login", {
        method: "POST",
        body: { login: login.trim(), senha }
      });

      localStorage.setItem("token", dados.token);
      localStorage.setItem("usuario", JSON.stringify(dados.usuario));
      setToken(dados.token);
      setUsuario(dados.usuario);
      setLogin("");
      setSenha("");
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function carregarProdutos(silencioso = false) {
    try {
      setProdutos(await requisicao("/produtos"));
    } catch (erro) {
      if (!silencioso) mostrarAviso(erro.message, "erro");
    }
  }

  async function carregarComandas(silencioso = false) {
    try {
      setComandas(await requisicao("/comandas"));
      setUltimaAtualizacao(new Date());
    } catch (erro) {
      if (!silencioso) mostrarAviso(erro.message, "erro");
    }
  }

  async function carregarUsuarios(silencioso = false) {
    if (usuario?.perfil !== "admin") return;

    try {
      setUsuarios(await requisicao("/usuarios"));
    } catch (erro) {
      if (!silencioso) mostrarAviso(erro.message, "erro");
    }
  }

  async function carregarCaixas(silencioso = false) {
    if (usuario?.perfil !== "admin") return;

    try {
      setCaixas(await requisicao("/caixas"));
    } catch (erro) {
      if (!silencioso) mostrarAviso(erro.message, "erro");
    }
  }

  async function carregarConfiguracoes(silencioso = false) {
    if (usuario?.perfil !== "admin") return;

    try {
      setConfiguracoes(await requisicao("/configuracoes"));
    } catch (erro) {
      if (!silencioso) mostrarAviso(erro.message, "erro");
    }
  }

  async function salvarConfiguracoes(event) {
    event.preventDefault();
    setOcupado(true);

    try {
      const dados = await requisicao("/configuracoes", {
        method: "PUT",
        body: configuracoes
      });

      setConfiguracoes(dados);
      mostrarAviso("Configurações salvas.");
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function carregarDatasRelatorios(silencioso = false) {
    if (usuario?.perfil !== "admin") return;

    try {
      setDatasRelatorios(await requisicao("/relatorios/datas"));
    } catch (erro) {
      if (!silencioso) mostrarAviso(erro.message, "erro");
    }
  }

  async function carregarRelatorio(data = dataRelatorio, silencioso = false) {
    if (usuario?.perfil !== "admin") return;
    if (!silencioso) setOcupado(true);

    try {
      const dados = await requisicao(`/relatorios/dia?data=${encodeURIComponent(data)}`);
      setDataRelatorio(dados.data);
      setRelatorio(dados);
      await carregarDatasRelatorios(true);
    } catch (erro) {
      if (!silencioso) mostrarAviso(erro.message, "erro");
    } finally {
      if (!silencioso) setOcupado(false);
    }
  }

  async function abrirComanda(id, silencioso = false) {
    try {
      const [dadosComanda, dadosItens] = await Promise.all([
        requisicao(`/comandas/${id}`),
        requisicao(`/comandas/${id}/itens`)
      ]);

      setComanda(dadosComanda);
      setItens(dadosItens);
      if (!silencioso) setPix(null);
    } catch (erro) {
      if (silencioso) {
        setComanda(null);
        setItens([]);
      } else {
        mostrarAviso(erro.message, "erro");
      }
    }
  }

  useEffect(() => {
    if (!aviso) return undefined;
    const tempo = setTimeout(() => setAviso(null), 3800);
    return () => clearTimeout(tempo);
  }, [aviso]);

  useEffect(() => {
    if (!token || !usuario) return;

    carregarProdutos();
    carregarComandas();

    if (usuario.perfil === "admin") {
      carregarUsuarios();
      carregarCaixas();
      carregarConfiguracoes();
    }
  }, [token, usuario?.perfil]);

  useEffect(() => {
    estadoTempoReal.current = {
      pagina,
      comandaID: comanda?.id || null,
      relatorioData: relatorio?.data || null,
      perfil: usuario?.perfil
    };
  }, [pagina, comanda?.id, relatorio?.data, usuario?.perfil]);

  useEffect(() => {
    if (!token || !usuario) return undefined;

    let socket;
    let reconexao;
    let tentativas = 0;
    let encerrado = false;

    function sincronizar(evento) {
      if (evento?.tipo !== "atualizar") return;

      const recursos = new Set(evento.recursos || []);
      const estado = estadoTempoReal.current;
      const tarefas = [];

      if (recursos.has("comandas")) tarefas.push(carregarComandas(true));
      if (recursos.has("produtos")) tarefas.push(carregarProdutos(true));

      const mesmaComanda =
        estado.comandaID && Number(evento.comanda_id) === Number(estado.comandaID);

      if (mesmaComanda && (recursos.has("comandas") || recursos.has("itens"))) {
        tarefas.push(abrirComanda(estado.comandaID, true));
      }

      if (estado.perfil === "admin") {
        if (recursos.has("usuarios")) tarefas.push(carregarUsuarios(true));
        if (recursos.has("caixas")) tarefas.push(carregarCaixas(true));
        if (recursos.has("configuracoes")) tarefas.push(carregarConfiguracoes(true));

        if (recursos.has("relatorios")) {
          tarefas.push(
            estado.relatorioData
              ? carregarRelatorio(estado.relatorioData, true)
              : carregarDatasRelatorios(true)
          );
        }
      }

      void Promise.allSettled(tarefas);
    }

    function agendarReconexao() {
      if (encerrado) return;
      const atraso = Math.min(1000 * 2 ** tentativas, 10000);
      tentativas += 1;
      reconexao = setTimeout(conectar, atraso);
    }

    function conectar() {
      if (encerrado) return;

      try {
        socket = new WebSocket(WS, [
          "comanda-facil",
          `jwt.${token}`,
          `cliente.${clienteID}`
        ]);
      } catch {
        agendarReconexao();
        return;
      }

      socket.onopen = () => {
        tentativas = 0;
        setTempoRealAtivo(true);
      };

      socket.onmessage = mensagem => {
        try {
          sincronizar(JSON.parse(mensagem.data));
        } catch {
          // Ignora mensagens inválidas.
        }
      };

      socket.onerror = () => socket.close();
      socket.onclose = () => {
        setTempoRealAtivo(false);
        agendarReconexao();
      };
    }

    conectar();

    return () => {
      encerrado = true;
      clearTimeout(reconexao);
      socket?.close();
      setTempoRealAtivo(false);
    };
  }, [token, usuario?.perfil, clienteID]);

  useEffect(() => {
    if (!token || !usuario || tempoRealAtivo) return undefined;

    const intervalo = setInterval(() => {
      carregarProdutos(true);
      carregarComandas(true);

      if (usuario.perfil === "admin") {
        if (pagina === "usuarios") carregarUsuarios(true);
        if (pagina === "caixas") carregarCaixas(true);
        if (pagina === "configuracoes") carregarConfiguracoes(true);
        if (pagina === "relatorios") {
          relatorio?.data
            ? carregarRelatorio(relatorio.data, true)
            : carregarDatasRelatorios(true);
        }
      }

      if (comanda?.id) abrirComanda(comanda.id, true);
    }, 15000);

    return () => clearInterval(intervalo);
  }, [token, usuario?.perfil, tempoRealAtivo, pagina, comanda?.id, relatorio?.data]);

  async function criarComanda(event) {
    event.preventDefault();

    if (!mesa.trim()) {
      mostrarAviso("Informe a mesa ou identificação.", "erro");
      return;
    }

    setOcupado(true);

    try {
      const dados = await requisicao("/comandas", {
        method: "POST",
        body: { mesa: mesa.trim() }
      });

      setMesa("");
      setNovaComandaAberta(false);
      await carregarComandas(true);
      await abrirComanda(dados.id);
      mostrarAviso("Comanda aberta. O administrador já pode vê-la.");
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function adicionarProduto(produto) {
    if (!comanda) return;

    try {
      await requisicao(`/comandas/${comanda.id}/itens`, {
        method: "POST",
        body: { produto_id: produto.id, quantidade: 1 }
      });

      await Promise.all([
        abrirComanda(comanda.id, true),
        carregarComandas(true)
      ]);
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    }
  }

  async function alterarQuantidade(item, quantidade) {
    if (quantidade < 1) return;

    try {
      await requisicao(`/itens/${item.id}/quantidade`, {
        method: "PUT",
        body: { quantidade }
      });

      await Promise.all([
        abrirComanda(comanda.id, true),
        carregarComandas(true)
      ]);
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    }
  }

  async function removerItem(id) {
    try {
      await requisicao(`/itens/${id}`, { method: "DELETE" });

      await Promise.all([
        abrirComanda(comanda.id, true),
        carregarComandas(true)
      ]);
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    }
  }

  async function pagar(forma) {
    setOcupado(true);

    try {
      const dados = await requisicao(`/comandas/${comanda.id}/pagamento`, {
        method: "POST",
        body: { forma }
      });

      if (forma === "pix") {
        setPix(dados);
      } else {
        setComanda(null);
        setItens([]);
        await carregarComandas(true);
        mostrarAviso(`Pagamento de ${moeda(dados.total)} concluído.`);
      }
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function confirmarPix() {
    setOcupado(true);

    try {
      await requisicao(`/comandas/${comanda.id}/confirmar-pix`, {
        method: "POST"
      });

      setPix(null);
      setComanda(null);
      setItens([]);
      await carregarComandas(true);
      mostrarAviso("Pagamento Pix confirmado.");
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function copiarPix() {
    try {
      await navigator.clipboard.writeText(pix.pix);
      mostrarAviso("Código Pix copiado.");
    } catch {
      const campo = document.createElement("textarea");
      campo.value = pix.pix;
      campo.style.position = "fixed";
      campo.style.opacity = "0";
      document.body.appendChild(campo);
      campo.select();
      document.execCommand("copy");
      campo.remove();
      mostrarAviso("Código Pix copiado.");
    }
  }

  async function salvarUsuario(event) {
    event.preventDefault();
    const editando = Boolean(editorUsuario.id);
    setOcupado(true);

    try {
      const dados = await requisicao(
        editando ? `/usuarios/${editorUsuario.id}` : "/usuarios",
        {
          method: editando ? "PUT" : "POST",
          body: editorUsuario
        }
      );

      if (dados.id === usuario.id) {
        const usuarioAtualizado = {
          ...usuario,
          nome: dados.nome,
          login: dados.login
        };

        setUsuario(usuarioAtualizado);
        localStorage.setItem("usuario", JSON.stringify(usuarioAtualizado));
      }

      setEditorUsuario(null);
      await Promise.all([carregarUsuarios(true), carregarCaixas(true)]);
      mostrarAviso(editando ? "Usuário atualizado." : "Usuário criado.");
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function salvarProduto(event) {
    event.preventDefault();
    const editando = Boolean(editorProduto.id);
    setOcupado(true);

    try {
      await requisicao(
        editando ? `/produtos/${editorProduto.id}` : "/produtos",
        {
          method: editando ? "PUT" : "POST",
          body: {
            ...editorProduto,
            categoria: (editorProduto.categoria || "Geral").trim() || "Geral",
            preco: Number(editorProduto.preco)
          }
        }
      );

      setEditorProduto(null);
      await carregarProdutos(true);
      mostrarAviso(editando ? "Produto atualizado." : "Produto criado.");
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function confirmarFechamento() {
    setOcupado(true);

    try {
      const dados = await requisicao(
        `/usuarios/${caixaParaFechar.usuario_id}/fechar-caixa`,
        { method: "POST" }
      );

      setCaixaParaFechar(null);
      await carregarCaixas(true);
      mostrarAviso(`Caixa fechado em ${moeda(dados.total)}.`);
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  async function excluirComanda() {
    setOcupado(true);

    try {
      await requisicao(`/comandas/${comandaParaExcluir.id}`, {
        method: "DELETE"
      });

      setComandaParaExcluir(null);
      setComanda(null);
      setItens([]);

      await Promise.all([
        carregarComandas(true),
        carregarCaixas(true)
      ]);

      if (relatorio) await carregarRelatorio(relatorio.data, true);
      mostrarAviso("Comanda excluída.");
    } catch (erro) {
      mostrarAviso(erro.message, "erro");
    } finally {
      setOcupado(false);
    }
  }

  function abrirEditorUsuario(item) {
    setEditorUsuario(item ? { ...item, senha: "" } : usuarioNovo());
  }

  function abrirEditorProduto(item) {
    setEditorProduto(
      item
        ? {
            ...item,
            categoria: item.categoria || "Geral",
            preco: String(item.preco)
          }
        : produtoNovo()
    );
  }

  function navegar(destino) {
    setPagina(destino);
    setComanda(null);
    setItens([]);

    if (destino === "configuracoes") carregarConfiguracoes();
    if (destino === "caixas") carregarCaixas();

    if (destino === "relatorios") {
      carregarDatasRelatorios();
      carregarRelatorio(dataRelatorio);
    }
  }

  function tipoComanda(item) {
    if (item.status === "aberta") return "Em aberto";

    return {
      dinheiro: "Dinheiro",
      cartao: "Cartão",
      pix: "Pix"
    }[item.forma_pagamento] || "Fechada";
  }

  const comandasAbertas = useMemo(
    () => comandas.filter(item => item.status === "aberta"),
    [comandas]
  );

  const comandasFiltradas = useMemo(
    () =>
      comandas.filter(item => {
        const usuarioCorreto =
          filtroUsuario === "todos" || String(item.usuario_id) === filtroUsuario;

        const tipo = item.status === "aberta" ? "aberta" : item.forma_pagamento;

        const tipoCorreto =
          filtroTipo === "todos" ||
          (filtroTipo === "fechada" && item.status === "fechada") ||
          filtroTipo === tipo;

        return usuarioCorreto && tipoCorreto;
      }),
    [comandas, filtroUsuario, filtroTipo]
  );

  const categoriasProdutos = useMemo(
    () =>
      [...new Set(
        produtos
          .filter(item => item.ativo)
          .map(item => item.categoria || "Geral")
      )].sort((a, b) => a.localeCompare(b, "pt-BR")),
    [produtos]
  );

  const produtosFiltrados = useMemo(() => {
    const termo = buscaProduto.trim().toLocaleLowerCase("pt-BR");

    return produtos.filter(item => {
      const categoria = item.categoria || "Geral";

      return (
        item.ativo &&
        (categoriaProduto === "todos" || categoria === categoriaProduto) &&
        (!termo || item.nome.toLocaleLowerCase("pt-BR").includes(termo))
      );
    });
  }, [produtos, buscaProduto, categoriaProduto]);

  if (!token || !usuario) {
    return (
      <main className="login-pagina">
        <section className="login-apresentacao">
          <Marca detalhe="Pedidos, caixa e relatórios em um só lugar" />
          <h1>Atendimento simples.<br />Controle completo.</h1>
          <p>Use no computador ou no celular conectado à mesma rede.</p>
        </section>

        <form className="login-card" onSubmit={entrar}>
          <span className="sobre">Acesso ao sistema</span>
          <h2>Entrar</h2>

          <label>
            Usuário
            <input
              autoFocus
              required
              autoComplete="username"
              value={login}
              onChange={event => setLogin(event.target.value)}
              placeholder="Digite seu usuário"
            />
          </label>

          <label>
            Senha
            <input
              required
              type="password"
              autoComplete="current-password"
              value={senha}
              onChange={event => setSenha(event.target.value)}
              placeholder="Digite sua senha"
            />
          </label>

          <button className="botao botao-primario botao-largo" disabled={ocupado}>
            {ocupado ? "Entrando..." : "Entrar"}
          </button>
        </form>

        {aviso && <div className={`aviso aviso-${aviso.tipo}`}>{aviso.texto}</div>}
      </main>
    );
  }

  if (usuario.perfil !== "admin") {
    const totalAberto = comandasAbertas.reduce(
      (soma, item) => soma + Number(item.total || 0),
      0
    );

    return (
      <div className="atendimento-app">
        <header className="topbar">
          <Marca detalhe={usuario.nome} />
          <button className="botao botao-claro" onClick={sair}>Sair</button>
        </header>

        {!comanda ? (
          <main className="conteudo atendimento-conteudo">
            <Cabecalho
              sobre="Atendimento"
              titulo="Comandas abertas"
              texto="Abra uma comanda e toque nela para lançar os produtos."
              acao={
                <button
                  className="botao botao-primario"
                  onClick={() => setNovaComandaAberta(true)}
                >
                  + Nova comanda
                </button>
              }
            />

            <section className="resumo-atendimento">
              <div><span>Em atendimento</span><strong>{comandasAbertas.length}</strong></div>
              <div><span>Total em aberto</span><strong>{moeda(totalAberto)}</strong></div>
            </section>

            {comandasAbertas.length === 0 ? (
              <Vazio
                titulo="Nenhuma comanda aberta"
                texto="Abra a primeira comanda para iniciar um atendimento."
                acao={
                  <button
                    className="botao botao-primario"
                    onClick={() => setNovaComandaAberta(true)}
                  >
                    Abrir comanda
                  </button>
                }
              />
            ) : (
              <section className="grade-comandas">
                {comandasAbertas.map(item => (
                  <button
                    className="comanda-card"
                    key={item.id}
                    onClick={() => abrirComanda(item.id)}
                  >
                    <div>
                      <Selo tipo="aberta">Em aberto</Selo>
                      <span>#{item.id}</span>
                    </div>
                    <strong>Mesa {item.mesa}</strong>
                    <b>{moeda(item.total)}</b>
                    <small>Abrir pedido →</small>
                  </button>
                ))}
              </section>
            )}
          </main>
        ) : (
          <main className="conteudo pedido-pagina">
            <header className="pedido-topo">
              <button
                className="botao botao-claro"
                onClick={() => {
                  setComanda(null);
                  setItens([]);
                  setPix(null);
                }}
              >
                ← Voltar
              </button>

              <div>
                <span className="sobre">Comanda #{comanda.id}</span>
                <h1>Mesa {comanda.mesa}</h1>
              </div>

              <Selo tipo="aberta">Em aberto</Selo>
            </header>

            <div className="pedido-layout">
              <section className="painel catalogo">
                <div className="secao-topo">
                  <div><h2>Produtos</h2><p>Toque para adicionar.</p></div>
                  <input
                    type="search"
                    value={buscaProduto}
                    onChange={event => setBuscaProduto(event.target.value)}
                    placeholder="Buscar produto"
                    aria-label="Buscar produto"
                  />
                </div>

                <div className="filtros-categoria">
                  <button
                    type="button"
                    className={`botao ${categoriaProduto === "todos" ? "botao-primario" : "botao-claro"}`}
                    onClick={() => setCategoriaProduto("todos")}
                  >
                    Todos
                  </button>

                  {categoriasProdutos.map(categoria => (
                    <button
                      type="button"
                      key={categoria}
                      className={`botao ${categoriaProduto === categoria ? "botao-primario" : "botao-claro"}`}
                      onClick={() => setCategoriaProduto(categoria)}
                    >
                      {categoria}
                    </button>
                  ))}
                </div>

                {produtosFiltrados.length === 0 ? (
                  <Vazio
                    titulo="Nenhum produto"
                    texto="Não há produto ativo nessa busca ou categoria."
                  />
                ) : (
                  <div className="grade-produtos">
                    {produtosFiltrados.map(produto => (
                      <button
                        className="produto-card"
                        key={produto.id}
                        onClick={() => adicionarProduto(produto)}
                      >
                        <span>{produto.nome.slice(0, 1).toUpperCase()}</span>
                        <strong>{produto.nome}</strong>
                        <small>{produto.categoria || "Geral"}</small>
                        <b>{moeda(produto.preco)}</b>
                        <small>+ Adicionar</small>
                      </button>
                    ))}
                  </div>
                )}
              </section>

              <aside className="painel pedido-painel">
                <div className="secao-topo">
                  <div><h2>Pedido</h2><p>{itens.length} produto(s)</p></div>
                </div>

                <div className="lista-itens">
                  {itens.length === 0 ? (
                    <div className="pedido-vazio">Adicione produtos ao pedido.</div>
                  ) : (
                    itens.map(item => (
                      <article className="item-pedido" key={item.id}>
                        <div className="item-info">
                          <strong>{item.nome}</strong>
                          <span>{moeda(item.preco)} cada</span>
                        </div>

                        <div className="quantidade">
                          <button onClick={() => alterarQuantidade(item, item.quantidade - 1)}>−</button>
                          <strong>{item.quantidade}</strong>
                          <button onClick={() => alterarQuantidade(item, item.quantidade + 1)}>+</button>
                        </div>

                        <b>{moeda(item.subtotal)}</b>

                        <button
                          className="remover-item"
                          onClick={() => removerItem(item.id)}
                          aria-label={`Remover ${item.nome}`}
                        >
                          ×
                        </button>
                      </article>
                    ))
                  )}
                </div>

                <footer className="pedido-rodape">
                  <div><span>Total</span><strong>{moeda(comanda.total)}</strong></div>

                  <div className="formas-pagamento">
                    <button disabled={ocupado || !itens.length} onClick={() => pagar("dinheiro")}>Dinheiro</button>
                    <button disabled={ocupado || !itens.length} onClick={() => pagar("cartao")}>Cartão</button>
                    <button disabled={ocupado || !itens.length} onClick={() => pagar("pix")}>Pix</button>
                  </div>
                </footer>
              </aside>
            </div>
          </main>
        )}

        {novaComandaAberta && (
          <Modal
            titulo="Nova comanda"
            subtitulo="Use a mesa, o nome do cliente ou outra identificação."
            onClose={() => setNovaComandaAberta(false)}
          >
            <form className="formulario" onSubmit={criarComanda}>
              <label>
                Mesa ou identificação
                <input
                  autoFocus
                  value={mesa}
                  onChange={event => setMesa(event.target.value)}
                  placeholder="Ex.: 12, Balcão ou João"
                />
              </label>

              <footer className="modal-acoes">
                <button
                  type="button"
                  className="botao botao-claro"
                  onClick={() => setNovaComandaAberta(false)}
                >
                  Cancelar
                </button>
                <button className="botao botao-primario" disabled={ocupado}>Abrir comanda</button>
              </footer>
            </form>
          </Modal>
        )}

        {pix && (
          <Modal
            titulo="Receber por Pix"
            subtitulo={`Mesa ${comanda?.mesa}`}
            onClose={() => setPix(null)}
          >
            <div className="pix-modal">
              <strong>{moeda(pix.total)}</strong>
              <div className="pix-qr">
                <QRCodeSVG value={pix.pix} size={210} includeMargin />
              </div>
              <p>Chave: {pix.chave}</p>
              <button className="botao botao-claro botao-largo" onClick={copiarPix}>Copiar código Pix</button>
              <button className="botao botao-sucesso botao-largo" disabled={ocupado} onClick={confirmarPix}>Confirmar recebimento</button>
            </div>
          </Modal>
        )}

        {aviso && <div className={`aviso aviso-${aviso.tipo}`}>{aviso.texto}</div>}
      </div>
    );
  }

  const valorAberto = comandasAbertas.reduce(
    (soma, item) => soma + Number(item.total || 0),
    0
  );

  const totalExibido = comandas.reduce(
    (soma, item) => soma + Number(item.total || 0),
    0
  );

  return (
    <div className="admin-app">
      <aside className="sidebar">
        <Marca detalhe="Administração" />

        <nav>
          {paginasAdmin.map(([destino, rotulo], indice) => (
            <button
              key={destino}
              className={pagina === destino ? "ativo" : ""}
              onClick={() => navegar(destino)}
            >
              <span>{String(indice + 1).padStart(2, "0")}</span>
              {rotulo}
            </button>
          ))}
        </nav>

        <footer>
          <div><strong>{usuario.nome}</strong><small>Administrador</small></div>
          <button className="botao botao-sidebar" onClick={sair}>Sair</button>
        </footer>
      </aside>

      <header className="admin-mobile-topo">
        <div className="mobile-linha">
          <Marca detalhe={usuario.nome} />
          <button className="botao botao-claro" onClick={sair}>Sair</button>
        </div>

        <nav>
          {paginasAdmin.map(([destino, rotulo]) => (
            <button
              key={destino}
              className={pagina === destino ? "ativo" : ""}
              onClick={() => navegar(destino)}
            >
              {rotulo}
            </button>
          ))}
        </nav>
      </header>

      <main className="admin-conteudo">
        {pagina === "comandas" && (
          <>
            <Cabecalho
              sobre="Operação ao vivo"
              titulo="Comandas"
              texto={
                ultimaAtualizacao
                  ? `Atualizado às ${ultimaAtualizacao.toLocaleTimeString("pt-BR")}`
                  : "As novas comandas aparecem automaticamente."
              }
              acao={
                <span className={`ao-vivo ${tempoRealAtivo ? "" : "reconectando"}`}>
                  <i /> {tempoRealAtivo ? "Ao vivo" : "Reconectando"}
                </span>
              }
            />

            <section className="metricas">
              <article><span>Abertas</span><strong>{comandasAbertas.length}</strong><small>{moeda(valorAberto)} em atendimento</small></article>
              <article><span>Fechadas</span><strong>{comandas.filter(item => item.status === "fechada").length}</strong><small>No histórico exibido</small></article>
              <article><span>Movimentação</span><strong>{moeda(totalExibido)}</strong><small>Total das comandas</small></article>
            </section>

            <section className="painel filtros">
              <label>
                Usuário
                <select value={filtroUsuario} onChange={event => setFiltroUsuario(event.target.value)}>
                  <option value="todos">Todos</option>
                  {usuarios.map(item => (
                    <option key={item.id} value={String(item.id)}>{item.nome}</option>
                  ))}
                </select>
              </label>

              <label>
                Tipo
                <select value={filtroTipo} onChange={event => setFiltroTipo(event.target.value)}>
                  <option value="todos">Todos</option>
                  <option value="aberta">Em aberto</option>
                  <option value="fechada">Fechadas</option>
                  <option value="dinheiro">Dinheiro</option>
                  <option value="cartao">Cartão</option>
                  <option value="pix">Pix</option>
                </select>
              </label>

              <span>{comandasFiltradas.length} resultado(s)</span>
            </section>

            {comandasFiltradas.length === 0 ? (
              <Vazio
                titulo="Nenhuma comanda encontrada"
                texto="Altere os filtros ou aguarde um novo atendimento."
              />
            ) : (
              <section className="grade-comandas grade-admin">
                {comandasFiltradas.map(item => (
                  <article className="comanda-card admin-comanda-card" key={item.id}>
                    <button className="comanda-abrir" onClick={() => abrirComanda(item.id)}>
                      <div>
                        <Selo tipo={item.status === "aberta" ? "aberta" : "fechada"}>
                          {tipoComanda(item)}
                        </Selo>
                        <span>#{item.id}</span>
                      </div>
                      <strong>Mesa {item.mesa}</strong>
                      <span>{item.usuario}</span>
                      <b>{moeda(item.total)}</b>
                      <small>Ver itens →</small>
                    </button>

                    <button
                      className="botao-excluir-card"
                      onClick={() => setComandaParaExcluir(item)}
                    >
                      Excluir
                    </button>
                  </article>
                ))}
              </section>
            )}
          </>
        )}

        {pagina === "usuarios" && (
          <>
            <Cabecalho
              sobre="Equipe"
              titulo="Usuários"
              texto="Crie acessos e edite nome, login, senha, perfil ou situação."
              acao={
                <button className="botao botao-primario" onClick={() => abrirEditorUsuario(null)}>
                  + Novo usuário
                </button>
              }
            />

            <section className="grade-cadastros">
              {usuarios.map(item => (
                <article className={`cadastro-card ${item.ativo ? "" : "inativo"}`} key={item.id}>
                  <span className="avatar">{item.nome.slice(0, 1).toUpperCase()}</span>
                  <div>
                    <strong>{item.nome}</strong>
                    <small>@{item.login}</small>
                    <Selo tipo={item.ativo ? "ativo" : "inativo"}>
                      {item.ativo ? "Ativo" : "Inativo"}
                    </Selo>
                  </div>
                  <span>{item.perfil === "admin" ? "Administrador" : "Atendente"}</span>
                  <button className="botao botao-claro" onClick={() => abrirEditorUsuario(item)}>Editar</button>
                </article>
              ))}
            </section>
          </>
        )}

        {pagina === "produtos" && (
          <>
            <Cabecalho
              sobre="Cardápio"
              titulo="Produtos"
              texto="Edite nome, categoria, preço e disponibilidade."
              acao={
                <button className="botao botao-primario" onClick={() => abrirEditorProduto(null)}>
                  + Novo produto
                </button>
              }
            />

            <section className="grade-cadastros produtos-admin">
              {produtos.map(item => (
                <article className={`cadastro-card ${item.ativo ? "" : "inativo"}`} key={item.id}>
                  <span className="avatar avatar-produto">{item.nome.slice(0, 1).toUpperCase()}</span>
                  <div>
                    <strong>{item.nome}</strong>
                    <small>
                      {item.categoria || "Geral"} • {item.ativo ? "Disponível no atendimento" : "Produto inativo"}
                    </small>
                  </div>
                  <b>{moeda(item.preco)}</b>
                  <button className="botao botao-claro" onClick={() => abrirEditorProduto(item)}>Editar</button>
                </article>
              ))}
            </section>
          </>
        )}

        {pagina === "configuracoes" && (
          <>
            <Cabecalho
              sobre="Administração"
              titulo="Configurações"
              texto="Altere os dados usados para gerar os pagamentos Pix."
            />

            <section className="painel">
              <form className="formulario" onSubmit={salvarConfiguracoes}>
                <label>
                  Chave Pix
                  <input
                    required
                    value={configuracoes.pix_chave}
                    onChange={event => setConfiguracoes({
                      ...configuracoes,
                      pix_chave: event.target.value
                    })}
                    placeholder="CPF, telefone, e-mail ou chave aleatória"
                  />
                </label>

                <label>
                  Nome do recebedor
                  <input
                    required
                    maxLength="25"
                    value={configuracoes.pix_nome}
                    onChange={event => setConfiguracoes({
                      ...configuracoes,
                      pix_nome: event.target.value
                    })}
                  />
                </label>

                <label>
                  Cidade
                  <input
                    required
                    maxLength="15"
                    value={configuracoes.pix_cidade}
                    onChange={event => setConfiguracoes({
                      ...configuracoes,
                      pix_cidade: event.target.value
                    })}
                  />
                </label>

                <footer className="formulario-rodape">
                  <button className="botao botao-primario" disabled={ocupado}>
                    {ocupado ? "Salvando..." : "Salvar configurações"}
                  </button>
                </footer>
              </form>
            </section>
          </>
        )}

        {pagina === "caixas" && (
          <>
            <Cabecalho
              sobre="Conferência"
              titulo="Fechamento de caixa"
              texto="Valores pagos e ainda não incluídos em um fechamento."
              acao={<button className="botao botao-claro" onClick={() => carregarCaixas()}>Atualizar</button>}
            />

            <section className="painel tabela-painel">
              <table className="tabela tabela-responsiva">
                <thead>
                  <tr>
                    <th>Usuário</th>
                    <th>Dinheiro</th>
                    <th>Cartão</th>
                    <th>Pix</th>
                    <th>Total</th>
                    <th>Ação</th>
                  </tr>
                </thead>

                <tbody>
                  {caixas.length === 0 ? (
                    <tr className="linha-vazia"><td colSpan="6">Nenhum usuário encontrado.</td></tr>
                  ) : (
                    caixas.map(item => (
                      <tr key={item.usuario_id}>
                        <td data-label="Usuário">
                          <strong>{item.usuario}</strong>
                          {!item.ativo && <small>Inativo</small>}
                        </td>
                        <td data-label="Dinheiro">{moeda(item.dinheiro)}</td>
                        <td data-label="Cartão">{moeda(item.cartao)}</td>
                        <td data-label="Pix">{moeda(item.pix)}</td>
                        <td data-label="Total"><strong>{moeda(item.total)}</strong></td>
                        <td data-label="Ação">
                          <button
                            className="botao botao-primario"
                            disabled={Number(item.total || 0) <= 0}
                            onClick={() => setCaixaParaFechar(item)}
                          >
                            Fechar caixa
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </section>
          </>
        )}

        {pagina === "relatorios" && (
          <>
            <Cabecalho
              sobre="Histórico"
              titulo="Relatório diário"
              texto="Consulte os totais do dia e o resultado de cada usuário."
              acao={
                <button className="botao botao-claro" onClick={() => window.print()}>
                  Imprimir
                </button>
              }
            />

            <section className="painel filtros">
              <label>
                Data
                <input
                  type="date"
                  value={dataRelatorio}
                  onChange={event => setDataRelatorio(event.target.value)}
                />
              </label>

              <button
                className="botao botao-primario"
                disabled={ocupado}
                onClick={() => carregarRelatorio(dataRelatorio)}
              >
                Consultar
              </button>

              {datasRelatorios.length > 0 && (
                <label>
                  Relatórios salvos
                  <select
                    value={dataRelatorio}
                    onChange={event => {
                      setDataRelatorio(event.target.value);
                      carregarRelatorio(event.target.value);
                    }}
                  >
                    {datasRelatorios.map(data => (
                      <option key={data} value={data}>{formatarData(data)}</option>
                    ))}
                  </select>
                </label>
              )}
            </section>

            {relatorio && (
              <section className="painel tabela-painel">
                <div className="secao-topo">
                  <div>
                    <h2>{formatarData(relatorio.data)}</h2>
                    <p>{relatorio.quantidade_comandas} comanda(s) fechada(s)</p>
                  </div>
                </div>

                <table className="tabela tabela-responsiva">
                  <thead>
                    <tr>
                      <th>Usuário</th>
                      <th>Comandas</th>
                      <th>Dinheiro</th>
                      <th>Cartão</th>
                      <th>Pix</th>
                      <th>Total</th>
                    </tr>
                  </thead>

                  <tbody>
                    {(relatorio.usuarios || []).length === 0 ? (
                      <tr className="linha-vazia"><td colSpan="6">Nenhum movimento nesta data.</td></tr>
                    ) : (
                      relatorio.usuarios.map(item => (
                        <tr key={item.usuario_id}>
                          <td data-label="Usuário"><strong>{item.usuario}</strong></td>
                          <td data-label="Comandas">{item.quantidade_comandas}</td>
                          <td data-label="Dinheiro">{moeda(item.dinheiro)}</td>
                          <td data-label="Cartão">{moeda(item.cartao)}</td>
                          <td data-label="Pix">{moeda(item.pix)}</td>
                          <td data-label="Total"><strong>{moeda(item.total)}</strong></td>
                        </tr>
                      ))
                    )}
                  </tbody>

                  <tfoot>
                    <tr>
                      <td data-label="Resumo">Total do dia</td>
                      <td data-label="Comandas">{relatorio.quantidade_comandas}</td>
                      <td data-label="Dinheiro">{moeda(relatorio.dinheiro)}</td>
                      <td data-label="Cartão">{moeda(relatorio.cartao)}</td>
                      <td data-label="Pix">{moeda(relatorio.pix)}</td>
                      <td data-label="Total">{moeda(relatorio.total)}</td>
                    </tr>
                  </tfoot>
                </table>
              </section>
            )}
          </>
        )}
      </main>

      {editorUsuario && (
        <Modal
          titulo={editorUsuario.id ? "Editar usuário" : "Novo usuário"}
          subtitulo={
            editorUsuario.id
              ? "A senha só muda se você preencher o campo."
              : "Crie um acesso para a equipe."
          }
          onClose={() => setEditorUsuario(null)}
        >
          <form className="formulario grade-formulario" onSubmit={salvarUsuario}>
            <label>
              Nome
              <input
                autoFocus
                required
                value={editorUsuario.nome}
                onChange={event => setEditorUsuario({ ...editorUsuario, nome: event.target.value })}
              />
            </label>

            <label>
              Login
              <input
                required
                autoComplete="off"
                value={editorUsuario.login}
                onChange={event => setEditorUsuario({ ...editorUsuario, login: event.target.value })}
              />
            </label>

            <label>
              {editorUsuario.id ? "Nova senha (opcional)" : "Senha"}
              <input
                required={!editorUsuario.id}
                minLength="4"
                type="password"
                autoComplete="new-password"
                value={editorUsuario.senha}
                onChange={event => setEditorUsuario({ ...editorUsuario, senha: event.target.value })}
              />
            </label>

            <label>
              Perfil
              <select
                disabled={editorUsuario.id === usuario.id}
                value={editorUsuario.perfil}
                onChange={event => setEditorUsuario({ ...editorUsuario, perfil: event.target.value })}
              >
                <option value="usuario">Atendente</option>
                <option value="admin">Administrador</option>
              </select>
            </label>

            <label className="campo-check">
              <input
                type="checkbox"
                disabled={editorUsuario.id === usuario.id}
                checked={editorUsuario.ativo}
                onChange={event => setEditorUsuario({ ...editorUsuario, ativo: event.target.checked })}
              />
              Usuário ativo
            </label>

            {editorUsuario.id === usuario.id && (
              <p className="nota-formulario">Seu próprio perfil deve permanecer como administrador ativo.</p>
            )}

            <footer className="modal-acoes formulario-rodape">
              <button type="button" className="botao botao-claro" onClick={() => setEditorUsuario(null)}>Cancelar</button>
              <button className="botao botao-primario" disabled={ocupado}>Salvar</button>
            </footer>
          </form>
        </Modal>
      )}

      {editorProduto && (
        <Modal
          titulo={editorProduto.id ? "Editar produto" : "Novo produto"}
          subtitulo="Defina como o produto aparecerá no atendimento."
          onClose={() => setEditorProduto(null)}
        >
          <form className="formulario grade-formulario" onSubmit={salvarProduto}>
            <label>
              Nome
              <input
                autoFocus
                required
                value={editorProduto.nome}
                onChange={event => setEditorProduto({ ...editorProduto, nome: event.target.value })}
              />
            </label>

            <label>
              Categoria
              <input
                required
                value={editorProduto.categoria || "Geral"}
                onChange={event => setEditorProduto({ ...editorProduto, categoria: event.target.value })}
                placeholder="Ex.: Bebidas, Lanches, Porções"
              />
            </label>

            <label>
              Preço
              <input
                required
                type="number"
                min="0"
                step="0.01"
                inputMode="decimal"
                value={editorProduto.preco}
                onChange={event => setEditorProduto({ ...editorProduto, preco: event.target.value })}
              />
            </label>

            <label className="campo-check">
              <input
                type="checkbox"
                checked={editorProduto.ativo}
                onChange={event => setEditorProduto({ ...editorProduto, ativo: event.target.checked })}
              />
              Produto ativo
            </label>

            <footer className="modal-acoes formulario-rodape">
              <button type="button" className="botao botao-claro" onClick={() => setEditorProduto(null)}>Cancelar</button>
              <button className="botao botao-primario" disabled={ocupado}>Salvar</button>
            </footer>
          </form>
        </Modal>
      )}

      {caixaParaFechar && (
        <Modal
          titulo="Fechar caixa?"
          subtitulo={caixaParaFechar.usuario}
          onClose={() => setCaixaParaFechar(null)}
        >
          <div className="resumo-fechamento">
            <div><span>Dinheiro</span><strong>{moeda(caixaParaFechar.dinheiro)}</strong></div>
            <div><span>Cartão</span><strong>{moeda(caixaParaFechar.cartao)}</strong></div>
            <div><span>Pix</span><strong>{moeda(caixaParaFechar.pix)}</strong></div>
            <div><span>Total</span><strong>{moeda(caixaParaFechar.total)}</strong></div>
          </div>

          <footer className="modal-acoes">
            <button className="botao botao-claro" onClick={() => setCaixaParaFechar(null)}>Cancelar</button>
            <button className="botao botao-primario" disabled={ocupado} onClick={confirmarFechamento}>Confirmar fechamento</button>
          </footer>
        </Modal>
      )}

      {comanda && (
        <Modal
          titulo={`Comanda #${comanda.id}`}
          subtitulo={`Mesa ${comanda.mesa} • ${comanda.usuario || ""}`}
          onClose={() => {
            setComanda(null);
            setItens([]);
          }}
          grande
        >
          <table className="tabela tabela-responsiva">
            <thead>
              <tr><th>Produto</th><th>Qtd.</th><th>Unitário</th><th>Subtotal</th></tr>
            </thead>
            <tbody>
              {itens.length === 0 ? (
                <tr className="linha-vazia"><td colSpan="4">Nenhum item nesta comanda.</td></tr>
              ) : (
                itens.map(item => (
                  <tr key={item.id}>
                    <td data-label="Produto"><strong>{item.nome}</strong></td>
                    <td data-label="Qtd.">{item.quantidade}</td>
                    <td data-label="Unitário">{moeda(item.preco)}</td>
                    <td data-label="Subtotal"><strong>{moeda(item.subtotal)}</strong></td>
                  </tr>
                ))
              )}
            </tbody>
            <tfoot>
              <tr>
                <td data-label="Resumo" colSpan="3">Total</td>
                <td data-label="Total">{moeda(comanda.total)}</td>
              </tr>
            </tfoot>
          </table>

          <footer className="modal-acoes">
            <button
              className="botao botao-perigo"
              onClick={() => {
                setComandaParaExcluir(comanda);
                setComanda(null);
                setItens([]);
              }}
            >
              Excluir comanda
            </button>

            <button
              className="botao botao-claro"
              onClick={() => {
                setComanda(null);
                setItens([]);
              }}
            >
              Fechar
            </button>
          </footer>
        </Modal>
      )}

      {comandaParaExcluir && (
        <Modal
          titulo="Excluir comanda?"
          subtitulo={`Comanda #${comandaParaExcluir.id} • Mesa ${comandaParaExcluir.mesa}`}
          onClose={() => setComandaParaExcluir(null)}
        >
          <div className="confirmacao-perigosa">
            <p>
              Os itens e o valor desta comanda serão removidos. Se ela estiver paga,
              o caixa e o relatório da data também serão atualizados.
            </p>
            <strong>Esta ação não pode ser desfeita.</strong>
          </div>

          <footer className="modal-acoes">
            <button className="botao botao-claro" onClick={() => setComandaParaExcluir(null)}>Cancelar</button>
            <button className="botao botao-perigo" disabled={ocupado} onClick={excluirComanda}>Excluir definitivamente</button>
          </footer>
        </Modal>
      )}

      {aviso && <div className={`aviso aviso-${aviso.tipo}`}>{aviso.texto}</div>}
    </div>
  );
}
