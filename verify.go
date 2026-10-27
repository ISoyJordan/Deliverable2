package main

import (
	"bytes"
	"errors"
	"io"
	"os/exec"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Verifier comprueba si una contraseña descifra el mensaje. Devuelve true
// cuando la contraseña es correcta.
type Verifier func(password []byte) bool

// errStop corta los reintentos internos de go-crypto: se devuelve la
// contraseña una sola vez y, si falla, se aborta en lugar de volver a pedirla.
var errStop = errors.New("contraseña incorrecta")

// NewNativeVerifier descifra en memoria con go-crypto, sin lanzar procesos.
// Es el camino rápido: solo paga la derivación de clave (S2K) y la
// comprobación de integridad, nada de fork/exec por intento.
func NewNativeVerifier(ciphertext []byte) Verifier {
	return func(password []byte) bool {
		used := false
		prompt := func(_ []openpgp.Key, symmetric bool) ([]byte, error) {
			if used {
				return nil, errStop
			}
			used = true
			return password, nil
		}

		md, err := openpgp.ReadMessage(bytes.NewReader(ciphertext), nil, prompt, nil)
		if err != nil {
			return false
		}
		// Agotar el cuerpo dispara la verificación de integridad (MDC/AEAD):
		// sin esto, una contraseña incorrecta podría pasar inadvertida.
		if _, err := io.Copy(io.Discard, md.UnverifiedBody); err != nil {
			return false
		}
		return true
	}
}

// NewGPGVerifier invoca el binario gpg una vez por intento. Es el camino
// lento (coste de fork/exec por candidata) y existe para comparar en los
// experimentos frente al verificador nativo.
func NewGPGVerifier(file string) Verifier {
	return func(password []byte) bool {
		cmd := exec.Command("gpg",
			"--batch", "--quiet", "--no-tty", "--yes",
			"--pinentry-mode", "loopback",
			"--passphrase-fd", "0",
			"--decrypt", "--output", "/dev/null",
			file,
		)
		cmd.Stdin = bytes.NewReader(append(password, '\n'))
		return cmd.Run() == nil
	}
}
