package main

import "context"

// generate produce, en orden de longitud creciente, todas las combinaciones
// del alfabeto cuya longitud va de minLen a maxLen (ambos incluidos), y las
// envía por out. Cierra out al terminar o si el contexto se cancela (clave
// encontrada por algún worker).
//
// Empezar por las claves más cortas es deliberado: lo más probable es que la
// contraseña sea corta, así que así se encuentra cuanto antes.
func generate(ctx context.Context, out chan<- []byte, alphabet string, minLen, maxLen int) {
	defer close(out)
	ab := []byte(alphabet)
	for length := minLen; length <= maxLen; length++ {
		if !emitLength(ctx, out, ab, length) {
			return // contexto cancelado
		}
	}
}

// emitLength recorre, como un cuentakilómetros, todas las palabras de una
// longitud fija. Devuelve false si el contexto se cancela a mitad.
func emitLength(ctx context.Context, out chan<- []byte, ab []byte, length int) bool {
	n := len(ab)
	idx := make([]int, length) // posición actual en cada dígito
	for {
		word := make([]byte, length)
		for i := 0; i < length; i++ {
			word[i] = ab[idx[i]]
		}
		select {
		case <-ctx.Done():
			return false
		case out <- word:
		}

		// Incrementar el cuentakilómetros desde el dígito menos significativo.
		pos := length - 1
		for pos >= 0 {
			idx[pos]++
			if idx[pos] < n {
				break
			}
			idx[pos] = 0
			pos--
		}
		if pos < 0 {
			return true // se agotaron todas las palabras de esta longitud
		}
	}
}
