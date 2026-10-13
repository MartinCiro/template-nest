// cmd/api/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api_go/config"
	"api_go/internal/app"
	"api_go/internal/interfaces/api/routes"
	"api_go/pkg/logger"
)

func main() {
	// Cargar configuración
	cfg := config.Load()

	if cfg.Env == "Production" {
		logger.SetupProduction()
	} else {
		logger.SetupDevelopment()
	}

	logger.Info("iniciando aplicación",
		"version", "1.0.0",
		"environment", cfg.Env,
		"server_address", ":"+cfg.Port)

	// Inicializar aplicación
	application := app.NewApp()
	if err := application.Initialize(cfg); err != nil {
		logger.Fatal("fallo al inicializar aplicación", "error", err)
	}
	defer application.Shutdown()

	// VERIFICAR que la aplicación esté saludable antes de iniciar servidor
	if err := application.HealthCheck(); err != nil {
		logger.Fatal("health check falló antes de iniciar servidor", "error", err)
	}

	// Configurar servidor HTTP
	server := &http.Server{
		Addr:         "0.0.0.0:" + cfg.Port,
		Handler:      routes.SetupRouter(application, cfg),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Canal para errores del servidor
	serverErr := make(chan error, 1)

	// Iniciar servidor en goroutine
	go func() {
		//logger.Info("iniciando servidor HTTP", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			logger.Error("error en servidor HTTP", "error", err)
		}
	}()

	// HEALTH CHECK INMEDIATO - Verificar que el servidor responde
	time.Sleep(1 * time.Second) // Dar tiempo a que el servidor arranque
	if err := checkServerHealth(cfg.Port); err != nil {
		logger.Fatal("servidor no responde a health checks", "error", err)
	}

	logger.Info("Servidor iniciado y respondiendo correctamente")

	// Esperar señal de interrupción o error del servidor
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case <-stop:
		logger.Info("🛑 Recibida señal de apagado, cerrando servidor...")
	case err := <-serverErr:
		logger.Error("❌ Error en el servidor", "error", err)
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("⚠️ Error durante shutdown", "error", err)
	} else {
		logger.Info("Servidor cerrado correctamente")
	}
}

// FUNCIÓN NUEVA: Verificar que el servidor responde
func checkServerHealth(port string) error {
	// Intentar conectarse al health check
	resp, err := http.Get("http://localhost:" + port + "/")
	if err != nil {
		return fmt.Errorf("no se pudo conectar al servidor: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check retornó status: %d", resp.StatusCode)
	}

	return nil
}

// responseWriter wrapper para interceptar status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
