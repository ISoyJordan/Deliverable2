package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const alfabeto = "abcdefghijklmnopqrstuvwxyz"

// encryptWith cifra un pequeño texto con gpg usando la contraseña dada y
// devuelve el criptograma. Omite el test si gpg no está instalado.
func encryptWith(t *testing.T, password string) []byte {
	t.Helper()
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg no está instalado; se omite el test de integración")
	}
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(plain, []byte("contenido de prueba"), 0o600); err != nil {
		t.Fatalf("no se pudo escribir el texto plano: %v", err)
	}
	out := filepath.Join(dir, "cifrado.gpg")
	cmd := exec.Command("gpg", "--batch", "--yes", "--pinentry-mode", "loopback",
		"--passphrase", password, "-c", "-o", out, plain)
	if err := cmd.Run(); err != nil {
		t.Fatalf("gpg no pudo cifrar: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("no se pudo leer el cifrado: %v", err)
	}
	return data
}

// El verificador nativo acepta la contraseña correcta y rechaza las demás.
func TestNativeVerifier(t *testing.T) {
	ct := encryptWith(t, "k")
	verify := NewNativeVerifier(ct)

	if !verify([]byte("k")) {
		t.Error("rechazó la contraseña correcta")
	}
	if verify([]byte("z")) {
		t.Error("aceptó una contraseña incorrecta")
	}
}

// El crackeo completo recupera una clave conocida corta.
func TestCrackFindsKnownKey(t *testing.T) {
	ct := encryptWith(t, "k") // 'k' se alcanza en 11 intentos de longitud 1
	verify := NewNativeVerifier(ct)

	res := Crack(verify, alfabeto, 1, 1, 2)
	if !res.Found {
		t.Fatal("no encontró la clave estando en el espacio de búsqueda")
	}
	if res.Password != "k" {
		t.Errorf("clave = %q, se esperaba %q", res.Password, "k")
	}
}

// El generador produce exactamente la cantidad y el orden esperados.
func TestGenerateCountAndOrder(t *testing.T) {
	ch := make(chan []byte)
	go generate(context.Background(), ch, "ab", 1, 2)

	var got []string
	for w := range ch {
		got = append(got, string(w))
	}
	// longitudes 1..2 sobre {a,b}: a, b, aa, ab, ba, bb
	want := []string{"a", "b", "aa", "ab", "ba", "bb"}
	if len(got) != len(want) {
		t.Fatalf("generadas %d, se esperaban %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("posición %d = %q, se esperaba %q", i, got[i], want[i])
		}
	}
}
