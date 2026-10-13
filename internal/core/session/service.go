package session

import (
	"context"
	"time"

	"api_go/internal/core/auth"
)

// SessionService se encarga exclusivamente de la gestión de sesiones de usuario
type SessionService struct {
	refreshService *auth.RefreshService
}

func NewSessionService(refreshService *auth.RefreshService) *SessionService {
	return &SessionService{
		refreshService: refreshService,
	}
}

// CheckAndRefreshFromCookie verifica y refresca la sesión desde la cookie actual
func (s *SessionService) CheckAndRefreshFromCookie(ctx context.Context, cookie string) (*auth.RefreshResult, error) {
	return s.refreshService.CheckAndRefreshFromCookie(ctx, cookie)
}

// CreateSession crea una nueva sesión para el usuario
func (s *SessionService) CreateSession(ctx context.Context, userID int, username, role string, roleID int) error {
	return s.refreshService.CreateSession(ctx, userID, username, role, roleID)
}

// UpdateSessionAccess actualiza el último acceso de la sesión
func (s *SessionService) UpdateSessionAccess(ctx context.Context, userID int) error {
	return s.refreshService.UpdateSessionAccess(ctx, userID)
}

// DeleteSession elimina la sesión del usuario
func (s *SessionService) DeleteSession(ctx context.Context, userID int) error {
	return s.refreshService.DeleteSession(ctx, userID)
}

// SessionExists verifica si el usuario tiene una sesión activa
func (s *SessionService) SessionExists(ctx context.Context, userID int) (bool, error) {
	return s.refreshService.SessionExists(ctx, userID)
}

// GetSessionInfo obtiene información detallada de la sesión
func (s *SessionService) GetSessionInfo(ctx context.Context, userID int) (map[string]interface{}, error) {
	return s.refreshService.GetSessionInfo(ctx, userID)
}

// ForceRefresh fuerza el refresh de una sesión
func (s *SessionService) ForceRefresh(ctx context.Context, userID int, username, role string, roleID int) (*auth.RefreshResult, error) {
	return s.refreshService.ForceRefresh(ctx, userID, username, role, roleID)
}

// GetActiveSessions obtiene todas las sesiones activas (para administración)
func (s *SessionService) GetActiveSessions(ctx context.Context) (map[string]time.Duration, error) {
	return s.refreshService.GetActiveSessions(ctx)
}