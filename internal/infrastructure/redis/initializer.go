// internal/infrastructure/redis/initializer.go
package redis

import (
	"api_go/config"
	"context"
	"fmt"
	"strings"

	"api_go/pkg/logger"
)

// InitializeCache inicializa y retorna el servicio de Redis con configuración
func InitializeCache(cfg *config.Config) *Cache {
	cache := NewCache(cfg)

	// Verificar conexión
	ctx := context.Background()
	if err := cache.HealthCheck(ctx); err != nil {
		// MEJORADO: Log con logger estructurado
		logger.Warn("no se pudo conectar a Redis",
			"error", err,
			"redis_url", maskPassword(cfg.RedisURL)) // ← Oculta password en logs

		logger.Warn("la aplicación funcionará sin cache de Redis")
		// No hacemos fatal para que la app pueda funcionar sin Redis
	} else {
		logger.Info("conexión a Redis establecida correctamente",
			"ttl_seconds", cfg.RedisTTL,
			"redis_url", maskPassword(cfg.RedisURL))
	}

	return cache
}

// maskPassword oculta la contraseña en los logs por seguridad
func maskPassword(redisURL string) string {
	if strings.Contains(redisURL, "@") {
		parts := strings.Split(redisURL, "@")
		if len(parts) == 2 {
			// Ocultar contraseña en la parte de autenticación
			authPart := parts[0]
			if strings.Contains(authPart, ":") {
				authParts := strings.Split(authPart, ":")
				if len(authParts) == 3 {
					// redis://user:password@host
					return fmt.Sprintf("redis://%s:***@%s", authParts[1], parts[1])
				}
			}
		}
	}
	return redisURL
}

// InitializeCacheWithOptions inicializa Redis con configuración y opciones adicionales
func InitializeCacheWithOptions(cfg *config.Config, customURL string) *Cache {
	var cache *Cache

	if customURL != "" {
		tempCfg := &config.Config{
			RedisURL: customURL,
			RedisTTL: cfg.RedisTTL,
		}
		cache = NewCache(tempCfg)
	} else {
		cache = NewCache(cfg)
	}

	// Verificar conexión
	ctx := context.Background()
	if err := cache.HealthCheck(ctx); err != nil {
		logger.Warn("no se pudo conectar a Redis con opciones personalizadas",
			"error", err,
			"custom_url", maskPassword(customURL))
	} else {
		logger.Info("conexión a Redis personalizada establecida",
			"custom_url", maskPassword(customURL))
	}

	return cache
}
