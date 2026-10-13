package routes

import (
	"log"
	"net/http"
	"time"
	"strings"

	"api_go/config"
	"api_go/internal/app"
	"api_go/internal/interfaces/api/common"
	"api_go/internal/interfaces/api/handlers/auth"
	common_handler "api_go/internal/interfaces/api/handlers/common"
	"api_go/internal/interfaces/api/handlers/estados"
	"api_go/internal/interfaces/api/handlers/permisos"
	"api_go/internal/interfaces/api/handlers/roles"
	"api_go/internal/interfaces/api/handlers/usuarios"
	"api_go/internal/interfaces/api/middlewares"
)

// responseWriter wrapper para interceptar status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SetupRouter configura todas las rutas de la aplicación
func SetupRouter(app *app.App, cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	// Servir archivos estáticos
	/* if cfg.IsDevelopment() {
		setupStaticFiles(mux)
	} */

	// Configurar todas las rutas
	setupAllRoutes(mux, app)

	// Aplicar middlewares globales
	return withGlobalMiddleware(mux, cfg)
}

// setupStaticFiles configura archivos estáticos
/* func setupStaticFiles(mux *http.ServeMux) {
	publicDir := "./public"

	if _, err := os.Stat(publicDir); err == nil {
		fs := http.FileServer(http.Dir(publicDir))
		mux.Handle("/api-docs/", http.StripPrefix("/api-docs", fs))
		log.Println("📚 Serviendo documentación en /api-docs")
	}
} */

// setupAllRoutes configura todas las rutas (públicas y protegidas)
func setupAllRoutes(mux *http.ServeMux, app *app.App) {

	// ========== RUTAS ACME para Let's Encrypt ==========
	// Este handler debe responder ANTES que cualquier otra ruta
	mux.HandleFunc("GET /.well-known/acme-challenge/", func(w http.ResponseWriter, r *http.Request) {
		// Let's Encrypt validation endpoint
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
		log.Printf("ACME challenge request: %s", r.URL.Path)
	})

	
	// Inicializar middlewares
	authMiddleware := middlewares.NewAuthMiddleware(app.JWTService)
	refreshMiddleware := middlewares.NewRefreshMiddleware(app.AuthService)

	// Configurar CookieSigner si está disponible
	if app.CookieSigner != nil {
		authMiddleware.SetCookieSigner(app.CookieSigner)
	}

	// Configurar auth service en el middleware
	authMiddleware.SetAuthService(app.AuthService)

	// Inicializar permisos middleware
	permsMiddleware := middlewares.NewPermissionsMiddleware(app.RedisCache)

	// Inicializar handlers
	estadosHandler := estados.NewEstadosHandler(app.EstadoService)
	permisosHandler := permisos.NewPermisosHandler(app.PermisoService)
	authHandler := auth.NewAuthHandler(app.AuthService, app.RegistrationService)      
	profileHandler := auth.NewProfileHandler(app.ProfileService)                      
	rolesHandler := roles.NewRolesHandler(app.RolService)
	usuariosHandler := usuarios.NewUsuariosHandler(app.UsuarioService)

	// ========== RUTAS PÚBLICAS ==========

	// Health checks
	mux.HandleFunc("GET /{$}", common_handler.HealthHandler)
	mux.HandleFunc("GET /health", common_handler.HealthHandler)
	mux.HandleFunc("GET /ready", common_handler.ReadyHandler(app.DB, app.RedisCache))

	// Autenticación (públicas)
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.Handle("POST /api/auth/register", authMiddleware.OptionalAuth(http.HandlerFunc(authHandler.Register)))

	// ========== RUTAS PROTEGIDAS ==========

	// Crear un subrouter para rutas protegidas
	protected := http.NewServeMux()

	// ✅ RUTAS DE USUARIOS (con refresh middleware)
	protected.Handle("GET /api/usuarios",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionUsuariosListar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(usuariosHandler.ObtenerUsuarios),
			),
		))

	protected.Handle("GET /api/usuarios/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionUsuariosListarXid)(
			refreshMiddleware.Handler(
				http.HandlerFunc(usuariosHandler.ObtenerUsuarioXid),
			),
		))

	protected.Handle("POST /api/usuarios",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionUsuariosCrear)(
			refreshMiddleware.Handler(
				http.HandlerFunc(authHandler.Register),
			),
		))

	protected.Handle("PATCH /api/usuarios/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionUsuariosEditar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(usuariosHandler.ActualizarUsuario),
			),
		))

	// ✅ RUTAS DE ESTADOS (con refresh middleware)
	protected.Handle("GET /api/estados",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionEstadosListar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(estadosHandler.ObtenerEstados),
			),
		))

	protected.Handle("GET /api/estados/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionEstadosVer)(
			refreshMiddleware.Handler(
				http.HandlerFunc(estadosHandler.ObtenerEstadoXid),
			),
		))

	protected.Handle("POST /api/estados",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionEstadosCrear)(
			refreshMiddleware.Handler(
				http.HandlerFunc(estadosHandler.CrearEstado),
			),
		))

	protected.Handle("PATCH /api/estados/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionEstadosEditar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(estadosHandler.ActualizarEstado),
			),
		))

	protected.Handle("DELETE /api/estados/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionEstadosEliminar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(estadosHandler.EliminarEstado),
			),
		))

	// ✅ RUTAS DE PERMISOS (con refresh middleware)
	protected.Handle("GET /api/permisos",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionPermisosListar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(permisosHandler.ObtenerPermisos),
			),
		))

	protected.Handle("GET /api/permisos/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionPermisosVer)(
			refreshMiddleware.Handler(
				http.HandlerFunc(permisosHandler.ObtenerPermisoXid),
			),
		))

	protected.Handle("POST /api/permisos",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionPermisosCrear)(
			refreshMiddleware.Handler(
				http.HandlerFunc(permisosHandler.CrearPermiso),
			),
		))

	protected.Handle("PATCH /api/permisos/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionPermisosEditar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(permisosHandler.ActualizarPermiso),
			),
		))

	protected.Handle("DELETE /api/permisos/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionPermisosEliminar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(permisosHandler.EliminarPermiso),
			),
		))

	// ✅ RUTAS DE ROLES (con refresh middleware)
	protected.Handle("GET /api/roles",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionRolesListar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(rolesHandler.ObtenerRoles),
			),
		))

	protected.Handle("GET /api/roles/permisos",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionRolesPermisos)(
			refreshMiddleware.Handler(
				http.HandlerFunc(rolesHandler.ObtenerPermisos),
			),
		))

	protected.Handle("GET /api/roles/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionRolesVer)(
			refreshMiddleware.Handler(
				http.HandlerFunc(rolesHandler.ObtenerRolXid),
			),
		))

	protected.Handle("POST /api/roles",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionRolesCrear)(
			refreshMiddleware.Handler(
				http.HandlerFunc(rolesHandler.CrearRol),
			),
		))

	protected.Handle("PATCH /api/roles/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionRolesEditar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(rolesHandler.ActualizarRol),
			),
		))

	protected.Handle("DELETE /api/roles/{id}",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionRolesEliminar)(
			refreshMiddleware.Handler(
				http.HandlerFunc(rolesHandler.EliminarRol),
			),
		))

	// ✅ RUTAS DE AUTH PROTEGIDAS (con refresh middleware)
	protected.Handle("POST /api/auth/logout",
		authMiddleware.RequireAuthAndPermission(permsMiddleware, middlewares.PermissionLoginLogout)(
			refreshMiddleware.Handler(
				http.HandlerFunc(authHandler.Logout),
			),
		))

	protected.Handle("GET /api/profile",
		authMiddleware.Handler(
			refreshMiddleware.Handler(
				http.HandlerFunc(profileHandler.GetProfile),
			),
		))

	// ✅ APLICAR MIDDLEWARE DE AUTENTICACIÓN AL ROUTER PROTEGIDO
	// El orden es importante: Auth primero, luego Refresh
	mux.Handle("/", authMiddleware.Handler(protected))
}

// withGlobalMiddleware aplica middlewares globales
func withGlobalMiddleware(handler http.Handler, cfg *config.Config) http.Handler {
	// ✅ CORS: ahora pasamos la configuración
	handler = withCORS(handler, cfg)
	// Logging
	handler = withLogging(handler, cfg)
	// Recovery (manejo de panics)
	handler = withRecovery(handler)
	return handler
}

// withCORS middleware para CORS (actualizado para cookies)
func withCORS(next http.Handler, cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Si no hay origen, es una petición same-origin o de herramienta como curl/postman
		if origin != "" {
			isAllowed := false
			
			for _, allowed := range cfg.CORSAllowedOrigins {
				// Coincidencia exacta
				if allowed == origin {
					isAllowed = true
					break
				}
				
				// Soporte seguro para *.trycloudflare.com
				if strings.HasSuffix(allowed, "trycloudflare.com") && strings.HasSuffix(origin, "trycloudflare.com") {
					isAllowed = true
					break
				}
			}

			if isAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Expose-Headers", "Set-Cookie")
			}
		}

		// Headers comunes (seguros de enviar siempre)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, jwt, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400") // Cache preflight por 24h

		// Responder preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// withLogging middleware para logging
func withLogging(next http.Handler, cfg *config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		if cfg.IsDevelopment() {
			log.Printf("%s %s %d %v", r.Method, r.URL.Path, rw.statusCode, duration)
		}
	})
}

// withRecovery middleware para recuperación de panics
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("⚠️ Panic recovered: %v", err)
				response := common.NewErrorResponse(500, "Internal Server Error")
				common.WriteJSONResponse(w, response, 500)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
