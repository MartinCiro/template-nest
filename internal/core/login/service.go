package login

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"api_go/config"
	"api_go/internal/core/auth"
	"api_go/internal/infrastructure/jwt"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/utils"
)

type LoginService struct {
	authPort        auth.AuthPort
	jwtService      *jwt.JWTService
	redisService    *redis.Cache
	passwordService *utils.PasswordService
	config          *config.Config
}

func NewLoginService(
	authPort auth.AuthPort,
	jwtService *jwt.JWTService,
	redisService *redis.Cache,
	passwordService *utils.PasswordService,
	config *config.Config,
) *LoginService {
	return &LoginService{
		config:          config,
		authPort:        authPort,
		jwtService:      jwtService,
		redisService:    redisService,
		passwordService: passwordService,
	}
}

type LoginCredentials struct {
	Email    string `json:"email"`
	Password string `json:"passwd"`
}

// LoginResult es un objeto del dominio, NO una respuesta HTTP
type LoginResult struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// Execute retorna objetos del dominio, NO estructuras HTTP
func (s *LoginService) Execute(ctx context.Context, credentials LoginCredentials) (*LoginResult, error) {

	// 1. Validar credenciales con AuthPort
	user, err := s.authPort.RetrieveUser(ctx, auth.AuthData{
		Username: credentials.Email,
	})

	if err != nil {
		fmt.Printf("❌ Error en RetrieveUser: %v\n", err)
		return nil, fmt.Errorf("credenciales inválidas")
	}

	if user == nil {
		fmt.Printf("❌ Usuario no encontrado: %s\n", credentials.Email)
		return nil, fmt.Errorf("credenciales inválidas")
	}

	// 2. Verificar contraseña
	passwordMatch := s.passwordService.ComparePasswords(credentials.Password, user.PasswordHash)

	if !passwordMatch {
		fmt.Printf("❌ Contraseña incorrecta para: %s\n", credentials.Email)
		return nil, fmt.Errorf("credenciales inválidas")
	}

	// 3. Generar token JWT
	token, err := s.jwtService.GenerateJWT(jwt.JwtPayload{
		IDUser:   user.ID,
		Username: user.Username,
		IDRol:    *user.IDRol,
	})
	if err != nil {
		return nil, fmt.Errorf("error generando token: %v", err)
	}

	// 4. Guardar en cache
	userCacheKey := fmt.Sprintf("user:%d", user.ID)
	userData := map[string]interface{}{
		"id_user":  user.ID,
		"nombre":   user.Username,
		"id_rol":   user.IDRol,
		"permisos": user.Permisos,
	}

	userDataJSON, _ := json.Marshal(userData)
	s.redisService.Set(ctx, userCacheKey, string(userDataJSON), 24*time.Hour)

	// 5. Construir resultado del dominio
	return &LoginResult{
		Token:     token,
		ExpiresIn: s.config.JWTExpireTime, // En segundos
	}, nil
}
