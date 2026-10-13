package auth

import (
	"log"
	"net/http"

	"api_go/internal/core/profile"
	"api_go/internal/interfaces/api/common"
	"api_go/internal/interfaces/api/middlewares"
)

type ProfileHandler struct {
	profileService *profile.ProfileService
}

func NewProfileHandler(profileService *profile.ProfileService) *ProfileHandler {
	return &ProfileHandler{profileService: profileService}
}

func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Printf("❌ PANIC en GetProfile: %v", err)
			common.InternalServerError(w, "Error interno del servidor")
		}
	}()

	// Obtener userID del contexto usando la clave tipada
	userID, ok := r.Context().Value(middlewares.UserIDKey).(string)
	if !ok || userID == "" {
		common.Unauthorized(w, "Usuario no autenticado")
		return
	}

	profileData, err := h.profileService.GetProfile(r.Context(), userID)
	if err != nil {
		switch err.Error() {
		case "ID de usuario inválido":
			common.BadRequest(w, err.Error())
		case "usuario no encontrado":
			common.NotFound(w, err.Error())
		default:
			common.InternalServerError(w, "Error al obtener perfil: "+err.Error())
		}
		return
	}

	response := common.ResponseBody[profile.Profile]{
		Success: true,
		Code:    200,
		Data:    *profileData,
	}
	common.WriteJSONResponse(w, response, 200)
}