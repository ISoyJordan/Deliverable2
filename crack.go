package main

import (
	"context"
	"sync"
	"sync/atomic"
)

// Result resume el desenlace de una búsqueda.
type Result struct {
	Password string // contraseña hallada (válida solo si Found)
	Found    bool
	Attempts uint64 // candidatas probadas en total
}

// runPool consume candidatas del canal con `workers` goroutines, aplicando
// `verify` a cada una. En cuanto una acierta, guarda el resultado y cancela
// el contexto (terminación temprana): el productor deja de generar y el resto
// de goroutines termina. Es el núcleo compartido por el crackeo y el benchmark.
func runPool(cancel context.CancelFunc, verify Verifier, candidates <-chan []byte, workers int) Result {
	var (
		attempts uint64
		once     sync.Once
		res      Result
		wg       sync.WaitGroup
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cand := range candidates {
				atomic.AddUint64(&attempts, 1)
				if verify(cand) {
					once.Do(func() {
						res.Password = string(cand)
						res.Found = true
						cancel()
					})
					return
				}
			}
		}()
	}

	wg.Wait()
	res.Attempts = atomic.LoadUint64(&attempts)
	return res
}

// Crack reparte todo el espacio de búsqueda (alfabeto, longitudes minLen..maxLen)
// entre `workers` goroutines y prueba cada candidata con `verify`.
func Crack(verify Verifier, alphabet string, minLen, maxLen, workers int) Result {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	candidates := make(chan []byte, workers*256)
	go generate(ctx, candidates, alphabet, minLen, maxLen)

	return runPool(cancel, verify, candidates, workers)
}
