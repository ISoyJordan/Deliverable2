# gpg-cracker

Recuperación por **fuerza bruta** de la contraseña de un archivo cifrado
simétricamente con GnuPG (`gpg -c`), cuando se sabe que la clave está formada
únicamente por caracteres de un alfabeto conocido (por defecto, minúsculas
`a–z`).

Escrito en **Go**: la búsqueda se reparte entre *goroutines* y se ofrecen dos
motores de verificación intercambiables para poder compararlos
experimentalmente.

---

## 1. Requisitos

- **Go** ≥ 1.21 (desarrollado y probado con 1.24).
- **GnuPG** (`gpg`) instalado y en el `PATH`. Imprescindible para el backend
  `gpg` y para el banco de pruebas; el backend `native` también lo usa solo si
  quieres crear tus propios archivos de prueba.
- Un entorno **GNU/Linux** (en Windows funciona perfectamente bajo **WSL**).

En Debian/Ubuntu/WSL:

```bash
sudo apt update && sudo apt install -y golang gnupg
```

## 2. Compilación

```bash
git clone <URL-de-tu-repo-privado> gpg-cracker
cd gpg-cracker
go build -o gpg-cracker .
```

Se genera un único binario estático, `gpg-cracker`.

## 3. Uso

Crackear el archivo (probando longitudes de 1 a 5 con todos los núcleos):

```bash
./gpg-cracker -file archive.gpg -min 1 -max 5
```

Opciones:

| Flag        | Por defecto            | Significado                                   |
|-------------|------------------------|-----------------------------------------------|
| `-file`     | `archive.gpg`          | Archivo `.gpg` a descifrar                     |
| `-alphabet` | `abcdefghijklmnopqrstuvwxyz` | Alfabeto de la contraseña               |
| `-min`      | `1`                    | Longitud mínima a probar                       |
| `-max`      | `6`                    | Longitud máxima a probar                       |
| `-workers`  | nº de núcleos          | Goroutines de trabajo en paralelo              |
| `-backend`  | `native`               | Motor de verificación: `native` o `gpg`        |
| `-bench N`  | `0`                    | Modo medición: prueba `N` contraseñas inválidas|
| `-gen`      | `stream`               | Generación en modo bench: `stream`/`preload`/`file` |

Ejemplo de salida:

```
Fichero : archive.gpg (62642 bytes)
Búsqueda: alfabeto de 26 símbolos, longitudes 1..5
Motor   : backend=native, workers=8

Probadas 12543 candidatas en 3m20s  (62.7/s)

  ✓ CONTRASEÑA ENCONTRADA: "sol"
```

### Verificar con una clave conocida

Tal como sugiere el enunciado, puedes crear un archivo con contraseña conocida
para comprobar que el programa funciona:

```bash
echo "texto" | gpg --batch --pinentry-mode loopback --passphrase "casa" -c -o prueba.gpg
./gpg-cracker -file prueba.gpg -min 1 -max 4   # debe devolver "casa"
```

## 4. Diseño

### Por qué Go

El problema es paralelo y limitado por CPU: cada candidata
se prueba de forma independiente. Go encaja de forma natural:

- *goroutines* + canales dan paralelismo real (sin GIL) con código sencillo;
- compila a un binario único, ideal para desplegar en WSL o en un contenedor;
- permite verificar **en memoria**, sin lanzar un proceso por intento.

### Arquitectura (productor–consumidores)

```
generate() ──► canal de candidatas ──► [worker 1]
 (1 goroutine)                         [worker 2]  ──► verify(candidata)
                                        [worker N]
```

- **`generate`** (`generate.go`) recorre el espacio como un cuentakilómetros,
  en **orden de longitud creciente** (primero las claves cortas, que son las
  más probables), y emite cada candidata por un canal.
- **El pool** (`crack.go`) lanza `N` *workers* que consumen del canal. En
  cuanto uno acierta, cancela el `context`: el generador para y el resto de
  *workers* termina (**terminación temprana**), evitando trabajo inútil.
- La cuenta de intentos es un contador atómico; el resultado se fija una sola
  vez con `sync.Once`.

### Dos motores de verificación

- **`native`** (`verify.go`): descifra en memoria con
  [`github.com/ProtonMail/go-crypto`](https://github.com/ProtonMail/go-crypto)
  (el sucesor mantenido de `x/crypto/openpgp`). Solo paga la derivación de
  clave y la comprobación de integridad; **ningún proceso externo**.
- **`gpg`** (`verify.go`): invoca el binario `gpg` una vez por candidata. Más
  lento por el coste de `fork/exec`, pero sirve de referencia honesta en los
  experimentos.

### El cuello de botella: el S2K

Inspeccionando el archivo (`gpg --list-packets archive.gpg`):

```
:symkey enc packet: version 4, cipher 9 (AES-256), s2k 3, hash 10 (SHA-512)
    salt ..., count 65011712 (255)
```

GnuPG deriva la clave con un **S2K iterado y salado** cuyo contador está al
**máximo (65 011 712)**: cada intento obliga a hashear ~65 MB con SHA-512. Ese
coste es **el mismo para los dos backends** y es, por diseño, la defensa contra
la fuerza bruta. Conclusión práctica: la velocidad la marca casi por completo
esa derivación, no los detalles de la implementación. Por eso la única palanca
real es **el paralelismo** (repartir derivaciones entre núcleos).

## 5. Experimentos

El script `bench/bench.sh` reproduce el protocolo del enunciado: medir el tiempo
de probar **N contraseñas inválidas** (una inválida es el peor caso, porque
siempre fuerza la derivación S2K completa).

```bash
go build -o gpg-cracker .
./bench/bench.sh 100        # genera bench/results.csv y una tabla
```

Realiza dos experimentos:

- **A — backend × workers** (generación fija `stream`): compara `native` vs `gpg` y mide el *speedup* al aumentar los *workers*.
- **B — estrategia de generación** (`native`, todos los núcleos): compara `stream`, `preload` y `file`.

### Resultados

Medidos en mi equipo (Windows + WSL, **8 núcleos lógicos**), con **N = 100** contraseñas inválidas.

**Experimento A — intentos/s según backend y número de workers**

| Workers | native | gpg |
|:-------:|:------:|:----:|
| 1 | 8.8 | 6.5 |
| 2 | 16.6 | 12.2 |
| 4 | 26.9 | 19.4 |
| 8 | 31.3 | 23.5 |

**Experimento B — estrategia de generación** (native, 8 workers)

| Estrategia | intentos/s |
|:----------:|:----------:|
| stream | 30.0 |
| preload | 30.3 |
| file | 30.4 |

### Conclusiones

1. **El paralelismo es la palanca clave.** Con el backend nativo el *speedup* es casi lineal hasta 4 *workers* (8.8 → 26.9 int/s, ×3.1) y se aplana al llegar a 8 (×3.6): coherente con una CPU de ~4 núcleos físicos con *hyperthreading*, donde los hilos adicionales comparten unidad de ejecución.
2. **`native` supera a `gpg` de forma constante (~1.35×)** a igual número de *workers*, porque evita el `fork/exec` de lanzar un proceso por intento. La ventaja es moderada porque el coste del S2K (la derivación de clave) domina el tiempo en ambos.
3. **La estrategia de generación es irrelevante en velocidad**: `stream`, `preload` y `file` quedan en ~30 int/s. Generar una candidata (nanosegundos) es despreciable frente a la derivación (decenas de ms). Sí importa en **memoria**: `preload` sobre un espacio grande (26⁶ ≈ 300 M) consumiría gigabytes, mientras que `stream` usa memoria constante. Por eso el crackeo real usa `stream`.

### Nota sobre el entorno de medición

El enunciado recomienda medir en un entorno acotado (contenedor con CPU y memoria limitadas) para reducir el ruido del sistema operativo. En el equipo utilizado (Windows con WSL) no se dispuso de Docker configurado, por lo que las medidas se tomaron directamente sobre WSL, controlando la carga de CPU mediante el número de *workers*. Se incluye un `Dockerfile` listo para ejecutar los experimentos en contenedor como vía recomendada para obtener medidas aún más reproducibles.

## 6. Estructura del proyecto

```
.
├── main.go            # CLI: flags, modo crackeo y modo benchmark
├── verify.go          # motores de verificación (native y gpg)
├── generate.go        # generación de candidatas (cuentakilómetros)
├── crack.go           # pool de workers + terminación temprana
├── bench.go           # arnés de medición (N inválidas, 3 estrategias)
├── cracker_test.go    # tests de corrección e integración
├── bench/bench.sh     # experimentos -> results.csv
├── Dockerfile         # entorno acotado para medir
├── archive.gpg        # archivo objetivo
├── go.mod / go.sum
└── README.md
```

## 7. Dependencias

Las dos líneas `replace` de `go.mod` redirigen `golang.org/x/crypto` y
`golang.org/x/sys` a sus espejos de GitHub. Se añadieron para poder compilar en
un entorno sin acceso al *proxy* de módulos de Go; **en una máquina con red
normal son innecesarias** y puedes eliminarlas con:

```bash
go mod edit -dropreplace=golang.org/x/crypto -dropreplace=golang.org/x/sys
go mod tidy
```

## 8. Alcance

La fuerza bruta pura solo es viable para contraseñas cortas: por el S2K, cada
intento cuesta ~0,1 s, así que `26ⁿ` crece hasta volverse inabordable alrededor
de `n = 6–7` incluso con muchos núcleos. Es el límite esperado del ejercicio
(la clave es corta) y la razón de recorrer de menor a mayor longitud.
