# Imagen reproducible para ejecutar los experimentos en un entorno ACOTADO.
# El enunciado recomienda medir con CPU y memoria limitadas para que el ruido
# del sistema operativo no falsee los resultados.
#
# Construir:
#   docker build -t gpg-cracker .
#
# Crackear (1 CPU, 512 MB):
#   docker run --rm --cpus=1 --memory=512m gpg-cracker \
#     ./gpg-cracker -file archive.gpg -min 1 -max 5
#
# Experimentos en entorno limitado:
#   docker run --rm --cpus=2 --memory=512m gpg-cracker ./bench/bench.sh 1000

# --- etapa de compilación ---
FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /gpg-cracker .

# --- imagen final: necesita gnupg para el backend "gpg" ---
FROM debian:stable-slim
RUN apt-get update && apt-get install -y --no-install-recommends gnupg bash coreutils \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build /gpg-cracker ./gpg-cracker
COPY archive.gpg ./archive.gpg
COPY bench ./bench
RUN chmod +x ./bench/bench.sh
ENTRYPOINT []
CMD ["./gpg-cracker", "-file", "archive.gpg", "-min", "1", "-max", "5"]
