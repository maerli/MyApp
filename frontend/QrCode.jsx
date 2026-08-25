import { useEffect, useState } from "react";
import { Html5QrcodeScanner } from "html5-qrcode";

export default function QrReader() {
  const [resultado, setResultado] = useState("");

  useEffect(() => {
    const scanner = new Html5QrcodeScanner(
      "qr-reader",
      {
        fps: 10,
        qrbox: {
          width: 250,
          height: 250,
        },
      },
      false
    );

    scanner.render(
      (texto) => {
        setResultado(texto);

        scanner.clear().catch(() => {});
      },
      () => {
        // erros de leitura contínua podem ser ignorados
      }
    );

    return () => {
      scanner.clear().catch(() => {});
    };
  }, []);

  return (
    <div style={styles.pagina}>
      <h2>Ler QR Code</h2>

      <div
        id="qr-reader"
        style={styles.leitor}
      />

      {resultado && (
        <div style={styles.resultado}>
          <strong>Resultado:</strong>

          <p>{resultado}</p>

          <button
            onClick={() => {
              navigator.clipboard.writeText(resultado);
            }}
          >
            Copiar
          </button>
        </div>
      )}
    </div>
  );
}

const styles = {
  pagina: {
    padding: "25px",
  },

  leitor: {
    maxWidth: "500px",
    margin: "20px auto",
  },

  resultado: {
    marginTop: "20px",
    padding: "15px",
    background: "#1e293b",
    borderRadius: "10px",
    color: "white",
  },
};