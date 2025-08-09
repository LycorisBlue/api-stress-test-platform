# ---- build ----
FROM golang:1.22-alpine AS build
WORKDIR /app

# précharger les deps pour profiter du cache
COPY go.mod ./
RUN go mod download

# copier le reste du code
COPY . .

# build statique linux/arm64 (ok pour Mac M1 et VPS ARM/AMD)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /worker

# ---- runtime ----
FROM alpine:3.20
WORKDIR /
EXPOSE 8090
# utilisateur non-root (optionnel)
USER nobody
COPY --from=build /worker /worker
ENTRYPOINT ["/worker"]
