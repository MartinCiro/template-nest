// internal/infrastructure/cookies/cookie_signer.go
package cookies

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type SignedCookieData struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	RoleID    int       `json:"role_id"`
	ExpiresAt time.Time `json:"expires_at"`
	IssuedAt  time.Time `json:"issued_at"`
}

type CookieSigner struct {
	secretKey []byte
}

func NewCookieSigner(secretKey string) *CookieSigner {
	return &CookieSigner{
		secretKey: []byte(secretKey),
	}
}

// Sign crea una cookie autofirmada con los datos del usuario
func (cs *CookieSigner) Sign(data *SignedCookieData) (string, error) {
	// Convertir datos a JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("error marshaling cookie data: %v", err)
	}

	// Codificar datos en base64
	encodedData := base64.URLEncoding.EncodeToString(jsonData)

	// Crear firma HMAC
	mac := hmac.New(sha256.New, cs.secretKey)
	mac.Write([]byte(encodedData))
	signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	// Combinar datos + firma
	return fmt.Sprintf("%s.%s", encodedData, signature), nil
}

// Verify verifica y extrae datos de la cookie autofirmada
func (cs *CookieSigner) Verify(signedData string) (*SignedCookieData, error) {
	parts := strings.Split(signedData, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("formato de cookie inválido")
	}

	encodedData, receivedSignature := parts[0], parts[1]

	// Verificar firma
	mac := hmac.New(sha256.New, cs.secretKey)
	mac.Write([]byte(encodedData))
	expectedSignature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(receivedSignature), []byte(expectedSignature)) {
		return nil, fmt.Errorf("firma inválida")
	}

	// Decodificar datos
	jsonData, err := base64.URLEncoding.DecodeString(encodedData)
	if err != nil {
		return nil, fmt.Errorf("error decoding cookie data: %v", err)
	}

	var data SignedCookieData
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("error unmarshaling cookie data: %v", err)
	}

	// Verificar expiración
	if time.Now().After(data.ExpiresAt) {
		return nil, fmt.Errorf("cookie expirada")
	}

	return &data, nil
}

// SetAuthCookie configura la cookie de autenticación en la respuesta HTTP
func SetAuthCookie(w http.ResponseWriter, cookieValue string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tk",
		Value:    cookieValue,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   true, // Solo HTTPS en producción
		SameSite: http.SameSiteStrictMode,
	})
	fmt.Printf("COOKIE SET SUCCESSFULLY\n")
}

// ClearAuthCookie elimina la cookie de autenticación
func ClearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "tk",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}
