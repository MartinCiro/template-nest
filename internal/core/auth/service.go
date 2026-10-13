package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"api_go/config"
	"api_go/internal/infrastructure/cookies"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/logger"
	"api_go/pkg/utils"
)

type AuthService struct {
	authPort        AuthPort
	redisService    *redis.Cache
	cookieSigner    *cookies.CookieSigner
	passwordService *utils.PasswordService
	config          *config.Config
	refreshService  *RefreshService
}

func NewAuthService(
	authPort AuthPort,
	redisService *redis.Cache,
	cookieSigner *cookies.CookieSigner,
	passwordService *utils.PasswordService,
	config *config.Config,
) *AuthService {
	sessionTTL := time.Duration(config.SessionTTL) * time.Second
	refreshThreshold := time.Duration(config.RefreshThreshold) * time.Second
	refreshService := NewRefreshService(redisService, cookieSigner, sessionTTL, refreshThreshold)

	return &AuthService{
		authPort:        authPort,
		redisService:    redisService,
		cookieSigner:    cookieSigner,
		passwordService: passwordService,
		config:          config,
		refreshService:  refreshService,
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

type UserCacheData struct {
	IDUser   string   `json:"id_user"`
	Nombre   string   `json:"nombre"`
	IDRol    *int     `json:"id_rol"`
	Permisos []string `json:"permisos"`
}

// LoginUser maneja el login
func (s *AuthService) LoginUser(ctx context.Context, req LoginRequest) (*AuthResponse, string, time.Time, error) {
	if req.Email == "" {
		return nil, "", time.Time{}, fmt.Errorf("email o contraseña inválida")
	}

	usuarioRetrieved, err := s.authPort.RetrieveUser(ctx, AuthData{Email: req.Email})
	if err != nil {
		logger.Error("❌ Error buscando usuario", "email", req.Email, "error", err)
		return nil, "", time.Time{}, fmt.Errorf("email o contraseña inválida")
	}
	if usuarioRetrieved == nil || usuarioRetrieved.ID == "" {
		return nil, "", time.Time{}, fmt.Errorf("email o contraseña inválida")
	}

	usuario := NewUsuarioFromEncrypted(
		usuarioRetrieved.Username,
		usuarioRetrieved.PasswordHash,
		usuarioRetrieved.IDRol,
		usuarioRetrieved.IDEstado,
	)

	if !usuario.ComparePassword(req.Password, s.passwordService) {
		return nil, "", time.Time{}, fmt.Errorf("email o contraseña inválida")
	}

	permisos, err := s.authPort.GetPermissionsByRoleID(ctx, *usuarioRetrieved.IDRol)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("error obteniendo permisos: %v", err)
	}
	if len(permisos) == 0 {
		return nil, "", time.Time{}, fmt.Errorf("usuario sin permisos asignados")
	}

	// Cache de usuario
	userCacheKey := fmt.Sprintf("user:%s", usuarioRetrieved.ID)
	userData := UserCacheData{
		IDUser:   usuarioRetrieved.ID,
		Nombre:   usuarioRetrieved.Username,
		IDRol:    usuarioRetrieved.IDRol,
		Permisos: permisos,
	}
	userDataJSON, _ := json.Marshal(userData)
	_ = s.redisService.Set(ctx, userCacheKey, userDataJSON, 24*time.Hour)

	// Cookie
	cookieData := &cookies.SignedCookieData{
		UserID:    usuarioRetrieved.ID,
		Username:  usuarioRetrieved.Username,
		Role:      usuarioRetrieved.RolNombre,
		RoleID:    *usuarioRetrieved.IDRol,
		ExpiresAt: time.Now().Add(time.Duration(s.config.JWTExpireTime) * time.Second),
		IssuedAt:  time.Now(),
	}
	signedCookie, err := s.cookieSigner.Sign(cookieData)
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("error generando cookie: %v", err)
	}

	// Sesión en Redis
	if userIDInt, err := strconv.Atoi(usuarioRetrieved.ID); err == nil {
		_ = s.CreateUserSession(ctx, userIDInt, usuarioRetrieved.Username, usuarioRetrieved.RolNombre, *usuarioRetrieved.IDRol)
	}

	return &AuthResponse{
		Message:   "Login exitoso",
		ExpiresAt: cookieData.ExpiresAt,
	}, signedCookie, cookieData.ExpiresAt, nil
}

// ValidateCookie verifica la cookie y retorna el usuario con permisos
func (s *AuthService) ValidateCookie(cookieValue string) (*User, error) {
	cookieData, err := s.cookieSigner.Verify(cookieValue)
	if err != nil {
		return nil, fmt.Errorf("cookie inválida: %v", err)
	}

	ctx := context.Background()
	permisos, err := s.authPort.GetPermissionsByRoleID(ctx, cookieData.RoleID)
	if err != nil {
		return nil, fmt.Errorf("error validando permisos: %v", err)
	}
	if len(permisos) == 0 {
		return nil, fmt.Errorf("usuario sin permisos asignados")
	}

	return &User{
		ID:        cookieData.UserID,
		Username:  cookieData.Username,
		RolNombre: cookieData.Role,
		IDRol:     &cookieData.RoleID,
		Permisos:  permisos,
	}, nil
}

// Logout cierra la sesión usando el refreshService interno
func (s *AuthService) Logout(ctx context.Context, userID int) error {
	// 1. Eliminar sesión usando el refreshService que YA tenemos internamente
	err := s.refreshService.DeleteSession(ctx, userID)
	if err != nil {
		logger.Error("❌ Error eliminando sesión", "userID", userID, "error", err)
	} else {
		logger.Info("Sesión eliminada", "userID", userID)
	}

	// 2. Limpiar cache de usuario
	userCacheKey := fmt.Sprintf("user:%d", userID)
	if s.redisService != nil {
		err := s.redisService.Delete(ctx, userCacheKey)
		if err != nil {
			logger.Error("Error clearing user cache", "userID", userID, "error", err)
		} else {
			logger.Info("Cache de usuario eliminado", "userID", userID)
		}
	}

	return nil
}

// ===== Delegaciones a RefreshService =====

func (s *AuthService) CheckAndRefreshSessionFromCookie(ctx context.Context, cookie string) (*RefreshResult, error) {
	return s.refreshService.CheckAndRefreshFromCookie(ctx, cookie)
}

func (s *AuthService) CreateUserSession(ctx context.Context, userID int, username, role string, roleID int) error {
	return s.refreshService.CreateSession(ctx, userID, username, role, roleID)
}

func (s *AuthService) UpdateUserSessionAccess(ctx context.Context, userID int) error {
	return s.refreshService.UpdateSessionAccess(ctx, userID)
}

func (s *AuthService) DeleteUserSession(ctx context.Context, userID int) error {
	return s.refreshService.DeleteSession(ctx, userID)
}

// hasPermission helper (puede eliminarse si no se usa)
func (s *AuthService) hasPermission(permisos []string, permission string) bool {
	for _, perm := range permisos {
		if perm == permission {
			return true
		}
	}
	return false
}

// GetRefreshService expone el RefreshService para SessionService
func (s *AuthService) GetRefreshService() *RefreshService {
	return s.refreshService
}

// Evitar import no usado
var _ = strings.Contains