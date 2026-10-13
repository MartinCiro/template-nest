// internal/core/auth/refresh_service.go
package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"api_go/internal/infrastructure/cookies"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/logger"
)

type RefreshService struct {
	redisCache       *redis.Cache
	cookieSigner     *cookies.CookieSigner
	sessionTTL       time.Duration
	refreshThreshold time.Duration
}

type RefreshResult struct {
	Refreshed    bool      `json:"refreshed"`
	NewCookie    string    `json:"new_cookie,omitempty"`
	NewExpiresAt time.Time `json:"new_expires_at,omitempty"`
	TTLSeconds   int64     `json:"ttl_seconds"`
	Message      string    `json:"message"`
}

func NewRefreshService(
	redisCache *redis.Cache,
	cookieSigner *cookies.CookieSigner,
	sessionTTL time.Duration,
	refreshThreshold time.Duration,
) *RefreshService {
	return &RefreshService{
		redisCache:       redisCache,
		cookieSigner:     cookieSigner,
		sessionTTL:       sessionTTL,
		refreshThreshold: refreshThreshold,
	}
}

// CheckAndRefresh verifica el TTL y refresca la sesión si es necesario
func (s *RefreshService) CheckAndRefresh(ctx context.Context, userID int, username string, role string, roleID int, currentCookie string) (*RefreshResult, error) {
	sessionKey := s.generateSessionKey(userID)

	// Obtener TTL actual
	ttl, err := s.redisCache.GetTTL(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo TTL: %v", err)
	}

	logger.Debug("Verificando sesión",
		"userID", userID,
		"sessionKey", sessionKey,
		"ttl", ttl,
		"threshold", s.refreshThreshold)

	result := &RefreshResult{
		TTLSeconds: int64(ttl.Seconds()),
	}

	// Si la sesión no existe o ya expiró
	if ttl <= 0 {
		result.Message = "Sesión expirada o no existe"
		return result, nil
	}

	// Si el TTL es menor al threshold, refrescar
	if ttl <= s.refreshThreshold {
		logger.Info("Refrescando sesión automáticamente",
			"userID", userID,
			"ttl_actual", ttl,
			"nuevo_ttl", s.sessionTTL)

		// Refrescar en Redis
		err = s.redisCache.RefreshTTL(ctx, sessionKey, s.sessionTTL)
		if err != nil {
			return nil, fmt.Errorf("error refrescando TTL en Redis: %v", err)
		}

		// Crear datos para la nueva cookie
		newExpiresAt := time.Now().Add(s.sessionTTL)
		cookieData := &cookies.SignedCookieData{
			UserID:    strconv.Itoa(userID),
			Username:  username,
			Role:      role,
			RoleID:    roleID,
			ExpiresAt: newExpiresAt,
			IssuedAt:  time.Now(),
		}

		// Firmar nueva cookie
		newSignedCookie, err := s.cookieSigner.Sign(cookieData)
		if err != nil {
			return nil, fmt.Errorf("error firmando nueva cookie: %v", err)
		}

		result.Refreshed = true
		result.NewCookie = newSignedCookie
		result.NewExpiresAt = newExpiresAt
		result.Message = "Sesión refrescada automáticamente"
		result.TTLSeconds = int64(s.sessionTTL.Seconds())

		logger.Info("Sesión refrescada exitosamente",
			"userID", userID,
			"nuevo_ttl", s.sessionTTL)
	} else {
		result.Message = "Sesión activa, no requiere refresh"
	}

	return result, nil
}

// CheckAndRefreshFromCookie verifica y refresca directamente desde la cookie actual
func (s *RefreshService) CheckAndRefreshFromCookie(ctx context.Context, currentCookie string) (*RefreshResult, error) {
	// Verificar y extraer datos de la cookie actual
	cookieData, err := s.cookieSigner.Verify(currentCookie)
	if err != nil {
		return nil, fmt.Errorf("cookie inválida: %v", err)
	}

	// Convertir UserID a int
	userID, err := strconv.Atoi(cookieData.UserID)
	if err != nil {
		return nil, fmt.Errorf("ID de usuario inválido en cookie: %v", err)
	}

	return s.CheckAndRefresh(ctx, userID, cookieData.Username, cookieData.Role, cookieData.RoleID, currentCookie)
}

// GetSessionInfo obtiene información detallada de la sesión
func (s *RefreshService) GetSessionInfo(ctx context.Context, userID int) (map[string]interface{}, error) {
	sessionKey := s.generateSessionKey(userID)

	info, err := s.redisCache.GetTTLInfo(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo información de sesión: %v", err)
	}

	// Agregar información específica de refresh
	ttl, _ := s.redisCache.GetTTL(ctx, sessionKey)
	needsRefresh := ttl > 0 && ttl <= s.refreshThreshold

	info["user_id"] = userID
	info["session_key"] = sessionKey
	info["needs_refresh"] = needsRefresh
	info["refresh_threshold_seconds"] = int64(s.refreshThreshold.Seconds())
	info["session_ttl_seconds"] = int64(s.sessionTTL.Seconds())
	info["will_auto_refresh"] = needsRefresh

	return info, nil
}

// CreateSession crea una nueva sesión en Redis
func (s *RefreshService) CreateSession(ctx context.Context, userID int, username string, role string, roleID int) error {
	sessionKey := s.generateSessionKey(userID)

	sessionData := map[string]interface{}{
		"user_id":      userID,
		"username":     username,
		"role":         role,
		"role_id":      roleID,
		"created_at":   time.Now().Format(time.RFC3339),
		"last_access":  time.Now().Format(time.RFC3339),
		"session_type": "user_session",
	}

	return s.redisCache.Set(ctx, sessionKey, sessionData, s.sessionTTL)
}

// UpdateSessionAccess actualiza el último acceso de la sesión
func (s *RefreshService) UpdateSessionAccess(ctx context.Context, userID int) error {
	sessionKey := s.generateSessionKey(userID)

	// Obtener datos actuales de la sesión
	var sessionData map[string]interface{}
	err := s.redisCache.GetJSON(ctx, sessionKey, &sessionData)
	if err != nil {
		return fmt.Errorf("error obteniendo datos de sesión: %v", err)
	}

	if sessionData == nil {
		sessionData = make(map[string]interface{})
	}

	// Actualizar último acceso
	sessionData["last_access"] = time.Now().Format(time.RFC3339)
	sessionData["access_count"] = getAccessCount(sessionData) + 1

	// Guardar actualización (mantener el TTL existente)
	ttl, err := s.redisCache.GetTTL(ctx, sessionKey)
	if err != nil {
		return err
	}

	return s.redisCache.Set(ctx, sessionKey, sessionData, ttl)
}

// DeleteSession elimina una sesión
func (s *RefreshService) DeleteSession(ctx context.Context, userID int) error {
	sessionKey := s.generateSessionKey(userID)
	return s.redisCache.Delete(ctx, sessionKey)
}

// SessionExists verifica si una sesión existe
func (s *RefreshService) SessionExists(ctx context.Context, userID int) (bool, error) {
	sessionKey := s.generateSessionKey(userID)
	return s.redisCache.Exists(ctx, sessionKey)
}

// ForceRefresh fuerza el refresh de una sesión
func (s *RefreshService) ForceRefresh(ctx context.Context, userID int, username string, role string, roleID int) (*RefreshResult, error) {
	sessionKey := s.generateSessionKey(userID)

	// Verificar que la sesión existe
	exists, err := s.redisCache.Exists(ctx, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("error verificando existencia de sesión: %v", err)
	}

	if !exists {
		return nil, fmt.Errorf("sesión no existe para el usuario %d", userID)
	}

	// Refrescar en Redis
	err = s.redisCache.RefreshTTL(ctx, sessionKey, s.sessionTTL)
	if err != nil {
		return nil, fmt.Errorf("error refrescando TTL: %v", err)
	}

	// Crear nueva cookie
	newExpiresAt := time.Now().Add(s.sessionTTL)
	cookieData := &cookies.SignedCookieData{
		UserID:    strconv.Itoa(userID),
		Username:  username,
		Role:      role,
		RoleID:    roleID,
		ExpiresAt: newExpiresAt,
		IssuedAt:  time.Now(),
	}

	newCookie, err := s.cookieSigner.Sign(cookieData)
	if err != nil {
		return nil, fmt.Errorf("error firmando nueva cookie: %v", err)
	}

	return &RefreshResult{
		Refreshed:    true,
		NewCookie:    newCookie,
		NewExpiresAt: newExpiresAt,
		TTLSeconds:   int64(s.sessionTTL.Seconds()),
		Message:      "Sesión refrescada forzadamente",
	}, nil
}

// generateSessionKey genera la key de sesión para Redis
func (s *RefreshService) generateSessionKey(userID int) string {
	return fmt.Sprintf("session:user:%d", userID)
}

// GetActiveSessions obtiene todas las sesiones activas (para admin)
func (s *RefreshService) GetActiveSessions(ctx context.Context) (map[string]time.Duration, error) {
	return s.redisCache.KeysWithTTL(ctx, "session:user:*")
}

// Helper function para obtener contador de accesos
func getAccessCount(sessionData map[string]interface{}) int {
	if count, ok := sessionData["access_count"].(float64); ok {
		return int(count)
	}
	return 0
}
