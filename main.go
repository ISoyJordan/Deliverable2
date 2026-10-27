// Command gpg-cracker recupera por fuerza bruta la contraseña de un fichero
// cifrado simétricamente con gpg (gpg -c), cuando se sabe que la clave está
// formada por caracteres de un alfabeto conocido (por defecto, minúsculas).
//
// Paraleliza la búsqueda con goroutines y ofrece dos backends de verificación
// —nativo en memoria y mediante el binario gpg— para poder compararlos. Con la
// opción -bench entra en modo medición (protocolo de N contraseñas inválidas).
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"
)

func main() {
	var (
		file     = flag.String("file", "archive.gpg", "fichero .gpg a descifrar")
		alphabet = flag.String("alphabet", "abcdefghijklmnopqrstuvwxyz", "alfabeto de la contraseña")
		minLen   = flag.Int("min", 1, "longitud mínima a probar")
		maxLen   = flag.Int("max", 6, "longitud máxima a probar")
		workers  = flag.Int("workers", runtime.NumCPU(), "número de goroutines de trabajo")
		backend  = flag.String("backend", "native", "verificador: native (en memoria) | gpg (subproceso)")
		bench    = flag.Int("bench", 0, "modo benchmark: nº de contraseñas inválidas a medir (0 = crackear)")
		gen      = flag.String("gen", "stream", "generación en benchmark: stream | preload | file")
	)
	flag.Parse()

	ciphertext, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: no se pudo leer %q: %v\n", *file, err)
		os.Exit(1)
	}

	verify, err := newVerifier(*backend, *file, ciphertext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *bench > 0 {
		runBenchmark(verify, *backend, GenStrategy(*gen), *bench, *workers)
		return
	}

	runCrack(verify, *file, len(ciphertext), *alphabet, *backend, *minLen, *maxLen, *workers)
}

func newVerifier(backend, file string, ciphertext []byte) (Verifier, error) {
	switch backend {
	case "native":
		return NewNativeVerifier(ciphertext), nil
	case "gpg":
		return NewGPGVerifier(file), nil
	default:
		return nil, fmt.Errorf("backend desconocido %q (usa native o gpg)", backend)
	}
}

func runCrack(verify Verifier, file string, size int, alphabet, backend string, minLen, maxLen, workers int) {
	fmt.Printf("Fichero : %s (%d bytes)\n", file, size)
	fmt.Printf("Búsqueda: alfabeto de %d símbolos, longitudes %d..%d\n", len([]rune(alphabet)), minLen, maxLen)
	fmt.Printf("Motor   : backend=%s, workers=%d\n\n", backend, workers)

	start := time.Now()
	res := Crack(verify, alphabet, minLen, maxLen, workers)
	elapsed := time.Since(start)

	fmt.Printf("Probadas %d candidatas en %s  (%.1f/s)\n",
		res.Attempts, elapsed.Round(time.Millisecond), float64(res.Attempts)/elapsed.Seconds())

	if res.Found {
		fmt.Printf("\n  ✓ CONTRASEÑA ENCONTRADA: %q\n", res.Password)
		return
	}
	fmt.Println("\n  ✗ no está en el espacio de búsqueda indicado (prueba a ampliar -max)")
	os.Exit(2)
}

func runBenchmark(verify Verifier, backend string, gen GenStrategy, n, workers int) {
	res, err := Bench(verify, gen, n, workers)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error en benchmark: %v\n", err)
		os.Exit(1)
	}
	// Línea legible y fácil de parsear para el script de experimentos.
	fmt.Printf("BENCH backend=%s gen=%s workers=%d n=%d elapsed_ms=%d rate=%.1f\n",
		backend, res.Gen, res.Workers, res.N, res.Elapsed.Milliseconds(), res.Rate())
}
