// internal/infrastructure/redis/cache.go
package redis

import (
	"api_go/config"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
	config *config.Config
}

func NewCache(config *config.Config) *Cache {
	// Parsear la URL de Redis (puedes mejorar esto según tus necesidades)
	// Asumiendo que RedisURL es algo como "redis://localhost:6379"
	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		// Fallback a configuración por defecto si hay error
		opts = &redis.Options{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
		}
	}

	client := redis.NewClient(opts)

	// Usar RedisTTL de la configuración (en segundos)
	ttl := time.Duration(config.RedisTTL) * time.Second

	return &Cache{
		client: client,
		ttl:    ttl,
		config: config,
	}
}

// NewCacheWithConfig crea una instancia con configuración personalizada
func NewCacheWithConfig(addr, password string, db int, ttlSeconds int) *Cache {
	cache := NewCache(&config.Config{
		RedisURL: fmt.Sprintf("redis://%s", addr),
		RedisTTL: ttlSeconds,
	})
	cache.ttl = time.Duration(ttlSeconds) * time.Second
	return cache
}

// Set almacena un valor en Redis con TTL
func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	// Usar TTL proporcionado o el por defecto
	expiration := c.ttl
	if len(ttl) > 0 {
		expiration = ttl[0]
	}

	var finalValue interface{} = value

	// DETECTAR si ya es un JSON string para no serializar dos veces
	switch v := value.(type) {
	case string:
		// Intentar detectar si es JSON válido
		var js json.RawMessage
		if json.Unmarshal([]byte(v), &js) == nil {
			// Es JSON válido, guardar como string directamente
			finalValue = v
		} else {
			// No es JSON, serializar normalmente
			jsonValue, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("error serializando valor para Redis: %v", err)
			}
			finalValue = string(jsonValue)
		}
	case []byte:
		// Ya es bytes, usar directamente
		finalValue = v
	default:
		// Serializar otros tipos
		jsonValue, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("error serializando valor para Redis: %v", err)
		}
		finalValue = string(jsonValue)
	}

	err := c.client.Set(ctx, key, finalValue, expiration).Err()
	if err != nil {
		return fmt.Errorf("error guardando en Redis: %v", err)
	}

	return nil
}

// Get obtiene un valor de Redis y lo deserializa
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Key no existe, no es error
	}
	if err != nil {
		return "", fmt.Errorf("error obteniendo de Redis: %v", err)
	}

	return val, nil
}

// GetJSON obtiene un valor y lo deserializa a una estructura específica
func (c *Cache) GetJSON(ctx context.Context, key string, target interface{}) error {
	val, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if val == "" {
		return nil // Key no existe
	}

	if err := json.Unmarshal([]byte(val), target); err != nil {
		return fmt.Errorf("error deserializando JSON de Redis: %v", err)
	}

	return nil
}

// Delete elimina una clave de Redis
func (c *Cache) Delete(ctx context.Context, key string) error {
	err := c.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("error eliminando de Redis: %v", err)
	}
	return nil
}

// DeletePattern elimina todas las claves que coincidan con un patrón
func (c *Cache) DeletePattern(ctx context.Context, pattern string) error {
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("error buscando claves con patrón: %v", err)
	}

	if len(keys) > 0 {
		err = c.client.Del(ctx, keys...).Err()
		if err != nil {
			return fmt.Errorf("error eliminando claves con patrón: %v", err)
		}
	}

	return nil
}

// Exists verifica si una clave existe
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("error verificando existencia: %v", err)
	}
	return count > 0, nil
}

// Increment incrementa un valor numérico
func (c *Cache) Increment(ctx context.Context, key string, value int64, ttl ...time.Duration) (int64, error) {
	result, err := c.client.IncrBy(ctx, key, value).Result()
	if err != nil {
		return 0, fmt.Errorf("error incrementando valor: %v", err)
	}

	// Si se proporciona TTL, establecer expiración
	if len(ttl) > 0 {
		c.client.Expire(ctx, key, ttl[0])
	}

	return result, nil
}

// HealthCheck verifica la conexión con Redis
func (c *Cache) HealthCheck(ctx context.Context) error {
	_, err := c.client.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("error en health check de Redis: %v", err)
	}
	return nil
}

// GetClient retorna el cliente de Redis (para operaciones avanzadas)
func (c *Cache) GetClient() *redis.Client {
	return c.client
}

// Close cierra la conexión con Redis
// GetTTL obtiene el tiempo de vida restante de una key
func (c *Cache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	if c.client == nil {
		return 0, fmt.Errorf("redis client no inicializado")
	}

	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("error obteniendo TTL: %v", err)
	}

	return ttl, nil
}

// RefreshTTL refresca el TTL de una key
func (c *Cache) RefreshTTL(ctx context.Context, key string, expiration time.Duration) error {
	if c.client == nil {
		return fmt.Errorf("redis client no inicializado")
	}

	// Verificar si la key existe
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("error verificando existencia de key: %v", err)
	}

	if exists == 0 {
		return fmt.Errorf("key no existe: %s", key)
	}

	// Refrescar el TTL
	_, err = c.client.Expire(ctx, key, expiration).Result()
	if err != nil {
		return fmt.Errorf("error refrescando TTL: %v", err)
	}

	return nil
}

// GetWithTTL obtiene el valor y el TTL restante
func (c *Cache) GetWithTTL(ctx context.Context, key string) (string, time.Duration, error) {
	if c.client == nil {
		return "", 0, fmt.Errorf("redis client no inicializado")
	}

	// Obtener valor
	val, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", 0, nil // Key no existe
	}
	if err != nil {
		return "", 0, fmt.Errorf("error obteniendo valor: %v", err)
	}

	// Obtener TTL
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return "", 0, fmt.Errorf("error obteniendo TTL: %v", err)
	}

	return val, ttl, nil
}

// GetTTLInSeconds obtiene el TTL en segundos (entero)
func (c *Cache) GetTTLInSeconds(ctx context.Context, key string) (int64, error) {
	ttl, err := c.GetTTL(ctx, key)
	if err != nil {
		return 0, err
	}
	return int64(ttl.Seconds()), nil
}

// GetTTLInfo obtiene información completa del TTL
func (c *Cache) GetTTLInfo(ctx context.Context, key string) (map[string]interface{}, error) {
	val, ttl, err := c.GetWithTTL(ctx, key)
	if err != nil {
		return nil, err
	}

	info := map[string]interface{}{
		"key":           key,
		"value":         val,
		"ttl":           ttl.String(),
		"ttl_seconds":   int64(ttl.Seconds()),
		"ttl_minutes":   ttl.Minutes(),
		"expires_in":    int64(ttl.Seconds()),
		"will_expire":   ttl > 0,
		"is_persistent": ttl == -1, // -1 significa que no expira
	}

	return info, nil
}

// KeysWithTTL obtiene todas las keys que coinciden con un patrón y su TTL
func (c *Cache) KeysWithTTL(ctx context.Context, pattern string) (map[string]time.Duration, error) {
	if c.client == nil {
		return nil, fmt.Errorf("redis client no inicializado")
	}

	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("error buscando keys: %v", err)
	}

	result := make(map[string]time.Duration)
	for _, key := range keys {
		ttl, err := c.client.TTL(ctx, key).Result()
		if err != nil {
			continue // Saltar keys con error
		}
		result[key] = ttl
	}

	return result, nil
}

// IsAboutToExpire verifica si una key está por expirar (menos del threshold)
func (c *Cache) IsAboutToExpire(ctx context.Context, key string, threshold time.Duration) (bool, time.Duration, error) {
	ttl, err := c.GetTTL(ctx, key)
	if err != nil {
		return false, 0, err
	}

	// Si ttl es negativo, no expira o no existe
	if ttl <= 0 {
		return false, ttl, nil
	}

	return ttl <= threshold, ttl, nil
}

// Close cierra la conexión con Redis
func (c *Cache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
