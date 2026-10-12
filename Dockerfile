# --- Etapa de compilación ---
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/api

# --- Etapa de producción ---
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 1. Crear un grupo y un usuario sin privilegios
RUN addgroup -g 1001 appgroup && \
    adduser -D -u 1001 -G appgroup appuser

# 2. Copiar el binario compilado
COPY --from=builder /app/main .

# 3. Dar propiedad del binario al usuario no-root
RUN chown appuser:appgroup /app/main

# 4. Cambiar al usuario no-root ANTES de ejecutar
USER appuser

EXPOSE 8080
CMD ["./main"]