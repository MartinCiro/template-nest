// internal/interfaces/api/middlewares/refresh.go
package middlewares

import (
	"context"
	"net/http"
	"strconv"

	"api_go/internal/core/session"
	"api_go/internal/infrastructure/cookies"
	"api_go/pkg/logger"
)

type RefreshMiddleware struct {
	sessionService *session.SessionService
}

func NewRefreshMiddleware(sessionService *session.SessionService) *RefreshMiddleware {
	return &RefreshMiddleware{
		sessionService: sessionService,
	}
}

// Handler es el middleware principal
func (m *RefreshMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Obtener cookie actual
		cookie, err := r.Cookie("tk")
		if err != nil {
			// No hay cookie, continuar
			next.ServeHTTP(w, r)
			return
		}

		// Usar SessionService para verificar y refrescar
		refreshResult, err := m.sessionService.CheckAndRefreshFromCookie(ctx, cookie.Value)
		if err != nil {
			logger.Debug("Error en refresh middleware",
				"error", err,
				"path", r.URL.Path)
			// Continuar sin refrescar
			next.ServeHTTP(w, r)
			return
		}

		// Si se refrescó, setear nueva cookie
		if refreshResult.Refreshed && refreshResult.NewCookie != "" {
			logger.Info("🔄 Sesión refrescada automáticamente",
				"path", r.URL.Path,
				"nuevo_ttl", refreshResult.TTLSeconds)
			cookies.SetAuthCookie(w, refreshResult.NewCookie, refreshResult.NewExpiresAt)
		}

		// Actualizar último acceso si tenemos userID
		if userID := m.getUserIDFromContext(ctx); userID > 0 {
			_ = m.sessionService.UpdateSessionAccess(ctx, userID)
		}

		next.ServeHTTP(w, r)
	})
}

// getUserIDFromContext extrae el userID del contexto
func (m *RefreshMiddleware) getUserIDFromContext(ctx context.Context) int {
	if userIDVal := ctx.Value(UserIDKey); userIDVal != nil {
		switch v := userIDVal.(type) {
		case int:
			return v
		case string:
			if id, err := strconv.Atoi(v); err == nil {
				return id
			}
		}
	}
	return 0
}
