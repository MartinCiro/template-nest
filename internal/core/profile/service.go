package profile

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"api_go/internal/core/auth"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/logger"
)

// Profile representa los datos del perfil de un usuario
type Profile struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Role     string   `json:"role"`
	Perms    []string `json:"permisos,omitempty"`
}

// ProfileService se encarga exclusivamente de obtener perfiles de usuario
type ProfileService struct {
	authPort     auth.AuthPort
	redisService *redis.Cache
}

func NewProfileService(authPort auth.AuthPort, redisService *redis.Cache) *ProfileService {
	return &ProfileService{
		authPort:     authPort,
		redisService: redisService,
	}
}

// GetProfile obtiene el perfil de un usuario por su ID
// Primero intenta desde caché, si no existe consulta la BD
func (s *ProfileService) GetProfile(ctx context.Context, userID string) (*Profile, error) {
	userIDInt, err := s.parseUserID(userID)
	if err != nil {
		return nil, err
	}

	userCacheKey := fmt.Sprintf("user:%d", userIDInt)

	// 1. Intentar obtener del caché
	if cachedUser, err := s.redisService.Get(ctx, userCacheKey); err == nil && cachedUser != "" {
		var profile Profile
		if err := json.Unmarshal([]byte(cachedUser), &profile); err == nil {
			return &profile, nil
		}
	}

	// 2. Consultar la base de datos
	user, err := s.authPort.RetrieveUserByID(ctx, userIDInt)
	if err != nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	profile := &Profile{
		ID:       user.ID,
		Username: user.Username,
		Role:     user.RolNombre,
		Perms:    user.Permisos,
	}

	// 3. Guardar en caché para próximas consultas
	profileJSON, err := json.Marshal(profile)
	if err == nil {
		if err := s.redisService.Set(ctx, userCacheKey, string(profileJSON), 24*time.Hour); err != nil {
			logger.Warn("no se pudo guardar perfil en caché", "error", err, "userID", userID)
		}
	}

	return profile, nil
}

// InvalidateProfile elimina el perfil del caché (útil tras actualizaciones)
func (s *ProfileService) InvalidateProfile(ctx context.Context, userID string) error {
	userIDInt, err := s.parseUserID(userID)
	if err != nil {
		return err
	}
	return s.redisService.Delete(ctx, fmt.Sprintf("user:%d", userIDInt))
}

func (s *ProfileService) parseUserID(userID string) (int, error) {
	var id int
	if _, err := fmt.Sscanf(userID, "%d", &id); err != nil {
		return 0, fmt.Errorf("ID de usuario inválido")
	}
	return id, nil
}