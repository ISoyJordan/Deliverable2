package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"
)

// GenStrategy indica cómo se producen las candidatas durante el benchmark.
// Es el eje que el enunciado pide comparar: ¿compensa generarlas todas al
// principio, de una en una bajo demanda, o leerlas de un fichero?
type GenStrategy string

const (
	GenStream  GenStrategy = "stream"  // una a una, bajo demanda (lo que usa el crackeo real)
	GenPreload GenStrategy = "preload" // todas materializadas en memoria antes de empezar
	GenFile    GenStrategy = "file"    // leídas línea a línea de un fichero de texto preexistente
)

// invalidPassword devuelve la i-ésima contraseña GARANTIZADA inválida: lleva
// caracteres fuera del alfabeto de minúsculas, así que nunca puede ser la
// clave y siempre fuerza una derivación S2K completa (el peor caso, que es lo
// que interesa cronometrar).
func invalidPassword(i int) []byte {
	return []byte(fmt.Sprintf("INVALID-%08d", i))
}

// BenchResult resume una corrida del benchmark.
type BenchResult struct {
	Gen     GenStrategy
	Workers int
	N       int
	Elapsed time.Duration
}

// Rate devuelve las candidatas probadas por segundo.
func (b BenchResult) Rate() float64 { return float64(b.N) / b.Elapsed.Seconds() }

// Bench mide el tiempo de probar n contraseñas inválidas con la estrategia de
// generación indicada y `workers` goroutines. La creación del fichero en la
// estrategia GenFile queda FUERA del cronómetro (se simula un diccionario ya
// existente); todo lo demás —incluida la materialización en GenPreload— se
// cronometra.
func Bench(verify Verifier, gen GenStrategy, n, workers int) (BenchResult, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Preparación no cronometrada: solo el diccionario en disco de GenFile.
	var filePath string
	if gen == GenFile {
		var err error
		if filePath, err = writeInvalidFile(n); err != nil {
			return BenchResult{}, err
		}
		defer os.Remove(filePath)
	}

	candidates := make(chan []byte, workers*256)

	start := time.Now()
	go feed(ctx, candidates, gen, n, filePath)
	runPool(cancel, verify, candidates, workers)
	elapsed := time.Since(start)

	return BenchResult{Gen: gen, Workers: workers, N: n, Elapsed: elapsed}, nil
}

// feed produce las n candidatas inválidas en el canal según la estrategia.
func feed(ctx context.Context, out chan<- []byte, gen GenStrategy, n int, filePath string) {
	defer close(out)

	send := func(b []byte) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- b:
			return true
		}
	}

	switch gen {
	case GenPreload:
		all := make([][]byte, n) // se materializa todo antes de enviar
		for i := range all {
			all[i] = invalidPassword(i)
		}
		for _, c := range all {
			if !send(c) {
				return
			}
		}

	case GenFile:
		f, err := os.Open(filePath)
		if err != nil {
			return
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if !send(append([]byte(nil), sc.Bytes()...)) {
				return
			}
		}

	default: // GenStream: genera cada candidata justo cuando hace falta
		for i := 0; i < n; i++ {
			if !send(invalidPassword(i)) {
				return
			}
		}
	}
}

// writeInvalidFile vuelca n contraseñas inválidas, una por línea, a un fichero
// temporal y devuelve su ruta.
func writeInvalidFile(n int) (string, error) {
	f, err := os.CreateTemp("", "invalids-*.txt")
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for i := 0; i < n; i++ {
		w.Write(invalidPassword(i))
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return "", err
	}
	return f.Name(), nil
}
