// internal/infrastructure/jwt/jwt_service.go
package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTService struct {
	secretKey   string
	environment string
}

type JwtPayload struct {
	IDUser   string `json:"id_user"`
	Username string `json:"username"`
	IDRol    int    `json:"id_rol"`
	Doc      string `json:"doc"`
	jwt.RegisteredClaims
}

type VerifyResponse struct {
	UserInfo *JwtPayload // ← Ahora solo un nivel
	JWT      *string
}

func NewJWTService(secretKey, environment string) *JWTService {
	return &JWTService{
		secretKey:   secretKey,
		environment: environment,
	}
}

// GenerateJWT genera un nuevo token JWT
func (js *JWTService) GenerateJWT(userInfo JwtPayload) (string, error) {
	token, _, err := js.GenerateJWTWithExpiry(userInfo, 24*time.Hour) // 24 horas por defecto
	return token, err
}

// GenerateJWTWithExpiry genera un token JWT con tiempo de expiración personalizado
func (js *JWTService) GenerateJWTWithExpiry(userInfo JwtPayload, expiry time.Duration) (string, time.Time, error) {
	if js.secretKey == "" {
		return "", time.Time{}, errors.New("JWT_SECRETO no está definido en la configuración")
	}

	expirationTime := time.Now().Add(expiry)

	claims := &JwtPayload{
		IDUser:   userInfo.IDUser,
		Username: userInfo.Username,
		IDRol:    userInfo.IDRol,
		Doc:      userInfo.Doc,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(js.secretKey))
	return tokenString, expirationTime, err
}

// VerifyJWT verifica y valida un token JWT
func (js *JWTService) VerifyJWT(tokenString string) (*VerifyResponse, error) {
	response := &VerifyResponse{}

	// Decodificar sin verificar primero (para desarrollo)
	if js.environment == "Dev" {
		decoded, err := js.decodeWithoutVerification(tokenString)
		if err != nil {
			return nil, js.handleJWTError(err)
		}
		response.UserInfo = decoded
		return response, nil
	}

	// Verificación completa para otros entornos
	verified, err := js.verifyWithSecret(tokenString)
	if err != nil {
		return nil, js.handleJWTError(err)
	}

	response.UserInfo = verified

	// Verificar si el token está cerca de expirar y regenerar
	if js.shouldRegenerateToken(verified) {
		newToken, err := js.GenerateJWT(*verified)
		if err != nil {
			return nil, err
		}
		response.JWT = &newToken
	}

	return response, nil
}

// decodeWithoutVerification decodifica el token sin verificar la firma (solo para desarrollo)
func (js *JWTService) decodeWithoutVerification(tokenString string) (*JwtPayload, error) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	token, _, err := parser.ParseUnverified(tokenString, &JwtPayload{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JwtPayload)
	if !ok {
		return nil, errors.New("no se pudieron obtener los claims del token")
	}

	// CORREGIDO: Validación correcta
	if len(claims.IDUser) == 0 {
		return nil, errors.New("el JWT es inválido: falta id_user")
	}

	return claims, nil
}

// verifyWithSecret verifica el token con la clave secreta
func (js *JWTService) verifyWithSecret(tokenString string) (*JwtPayload, error) {
	if js.secretKey == "" {
		return nil, errors.New("JWT_SECRETO no está definido en la configuración")
	}

	token, err := jwt.ParseWithClaims(tokenString, &JwtPayload{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de firma inesperado: %v", token.Header["alg"])
		}
		return []byte(js.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JwtPayload)
	if !ok || !token.Valid {
		return nil, errors.New("token inválido")
	}

	return claims, nil
}

// shouldRegenerateToken verifica si el token está cerca de expirar (menos de 10 minutos)
func (js *JWTService) shouldRegenerateToken(claims *JwtPayload) bool {
	expTime := claims.ExpiresAt
	if expTime == nil {
		return false
	}

	timeUntilExpiry := time.Until(expTime.Time)
	return timeUntilExpiry < 10*time.Minute
}

// handleJWTError maneja los diferentes tipos de errores de JWT
func (js *JWTService) handleJWTError(err error) error {
	switch {
	case errors.Is(err, jwt.ErrTokenExpired):
		return errors.New("JWT expirado. Por favor inicie sesión nuevamente")
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return errors.New("JWT aún no es válido")
	case errors.Is(err, jwt.ErrTokenUsedBeforeIssued):
		return errors.New("JWT usado antes de la fecha de emisión")
	case errors.Is(err, jwt.ErrTokenInvalidAudience):
		return errors.New("Audiencia del JWT inválida")
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return errors.New("Emisor del JWT inválido")
	case errors.Is(err, jwt.ErrTokenInvalidSubject):
		return errors.New("Sujeto del JWT inválido")
	case errors.Is(err, jwt.ErrTokenRequiredClaimMissing):
		return errors.New("Falta un claim requerido en el JWT")
	default:
		return errors.New("el JWT es inválido")
	}
}
