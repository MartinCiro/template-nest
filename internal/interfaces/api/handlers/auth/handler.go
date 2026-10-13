package auth

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"api_go/internal/core/auth"
	"api_go/internal/core/registration"
	"api_go/internal/core/session"
	"api_go/internal/infrastructure/cookies"
	"api_go/internal/interfaces/api/common"
	"api_go/internal/interfaces/api/middlewares"
)

type AuthHandler struct {
	authService         *auth.AuthService
	registrationService *registration.RegistrationService
	sessionService      *session.SessionService
}

func NewAuthHandler(
	authService *auth.AuthService,
	registrationService *registration.RegistrationService,
	sessionService *session.SessionService,
) *AuthHandler {
	return &AuthHandler{
		authService:         authService,
		registrationService: registrationService,
		sessionService:      sessionService,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteSimpleError(w, "Solicitud inválida", 400)
		return
	}
	if req.Email == "" || req.Password == "" {
		common.WriteSimpleError(w, "Email y contraseña son requeridos", 400)
		return
	}

	loginReq := auth.LoginRequest{Email: req.Email, Password: req.Password}
	authResponse, signedCookie, expiresAt, err := h.authService.LoginUser(r.Context(), loginReq)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 401)
		return
	}

	if signedCookie != "" {
		cookies.SetAuthCookie(w, signedCookie, expiresAt)
	}

	common.WriteJSONResponse(w, common.NewSuccessResponse(authResponse.Message), 200)
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registration.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteJSONResponse(w, common.NewErrorResponse(400, "Solicitud inválida"), 400)
		return
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		common.WriteJSONResponse(w, common.NewErrorResponse(400, "Username, email y password son requeridos"), 400)
		return
	}
	if len(req.Password) < 6 {
		common.WriteJSONResponse(w, common.NewErrorResponse(400, "El password debe tener al menos 6 caracteres"), 400)
		return
	}

	var currentUser *auth.User
	if user, ok := r.Context().Value(middlewares.UserKey).(*auth.User); ok {
		currentUser = user
	}

	result, err := h.registrationService.Register(r.Context(), req, currentUser)
	if err != nil {
		statusCode := 400
		if strings.Contains(err.Error(), "Ha ocurrido un error en el servidor") {
			statusCode = 500
		} else if strings.Contains(err.Error(), "No tiene permisos") {
			statusCode = 403
		}
		common.WriteJSONResponse(w, common.NewErrorResponse(statusCode, err.Error()), statusCode)
		return
	}

	if result.SignedCookie != "" {
		cookies.SetAuthCookie(w, result.SignedCookie, result.ExpiresAt)
	}

	common.WriteJSONResponse(w, common.NewSuccessResponse(result.Message), 201)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if userIDStr, ok := ctx.Value(middlewares.UserIDKey).(string); ok {
		if userID, err := strconv.Atoi(userIDStr); err == nil {
			// Ya no pasamos sessionService, AuthService lo maneja internamente
			h.authService.Logout(ctx, userID)
		}
	}
	cookies.ClearAuthCookie(w)
	successResponse := common.NewSuccessResponse("Sesión cerrada exitosamente")
	common.WriteJSONResponse(w, successResponse, 200)
}