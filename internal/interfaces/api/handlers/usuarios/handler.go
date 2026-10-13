// internal/interfaces/api/handlers/usuarios/handler.go
package usuarios

import (
	"encoding/json"
	"net/http"
	"strconv"

	coreUsuarios "api_go/internal/core/usuarios"
	"api_go/internal/interfaces/api/common"
	usuariosDTOs "api_go/internal/interfaces/api/usuarios"
	"api_go/pkg/utils"
)

type UsuariosHandler struct {
	usuarioService *coreUsuarios.UsuarioService
}

func NewUsuariosHandler(usuarioService *coreUsuarios.UsuarioService) *UsuariosHandler {
	return &UsuariosHandler{
		usuarioService: usuarioService,
	}
}

func (h *UsuariosHandler) ObtenerUsuarios(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	usuarios, err := h.usuarioService.ObtenerUsuarios(ctx)
	if err != nil {
		utils.WriteValidationError(w, err, 500)
		return
	}

	response := common.NewSuccessResponse(usuariosDTOs.FromUsuarios(usuarios))
	common.WriteJSONResponse(w, response, 200)
}

func (h *UsuariosHandler) ObtenerUsuarioXid(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	ctx := r.Context()
	usuarioData := coreUsuarios.UsuarioDataXid{Documento: strconv.Itoa(id)}
	usuario, err := h.usuarioService.ObtenerUsuarioXid(ctx, usuarioData)
	if err != nil {
		utils.WriteValidationError(w, err, 500)
		return
	}

	if usuario == nil {
		common.WriteSimpleError(w, "Usuario no encontrado", 404)
		return
	}

	response := common.NewSuccessResponse(usuariosDTOs.FromUsuario(usuario))
	common.WriteJSONResponse(w, response, 200)
}

func (h *UsuariosHandler) ActualizarUsuario(w http.ResponseWriter, r *http.Request) {
	// Obtener ID desde la URL
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	// Decodificar el request body
	var req usuariosDTOs.UpdateUsuarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteSimpleError(w, "Solicitud inválida: formato JSON incorrecto", 400)
		return
	}

	// Asignar el ID obtenido de la URL al documento
	req.Documento = strconv.Itoa(id) // Convertir a string si Documento es string

	// VALIDAR DESPUÉS de asignar el documento
	if errors := common.ValidateRequest(req); errors != nil {
		common.WriteValidationErrors(w, errors, 400)
		return
	}

	ctx := r.Context()
	usuarioData := usuariosDTOs.ToUsuarioDataUpdate(req)

	_, err = h.usuarioService.UpUsuario(ctx, usuarioData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	successResponse := common.NewSuccessResponse("Se ha actualizado el usuario correctamente")
	common.WriteJSONResponse(w, successResponse, 200)
}

func (h *UsuariosHandler) EliminarUsuario(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	docu, err := strconv.Atoi(idStr)
	if err != nil || docu <= 0 { // Validar que sea mayor a 0
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	ctx := r.Context()
	usuarioData := coreUsuarios.UsuarioDataXid{Documento: strconv.Itoa(docu)}
	err = h.usuarioService.DelUsuario(ctx, usuarioData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	response := common.NewSuccessResponse("Usuario eliminado correctamente")
	common.WriteJSONResponse(w, response, 200)
}

// Handler para cambiar contraseña
/* func (h *UsuariosHandler) CambiarPassword(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	var req usuariosDTOs.CambiarPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteSimpleError(w, "Solicitud inválida: formato JSON incorrecto", 400)
		return
	}

	req.ID = id

	// VALIDAR DESPUÉS de asignar el ID
	if errors := common.ValidateRequest(req); errors != nil {
		common.WriteValidationErrors(w, errors, 400)
		return
	}

	ctx := r.Context()
	err = h.usuarioService.CambiarPassword(ctx, req.ID, req.PasswordActual, req.NuevoPassword)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	successResponse := common.NewSuccessResponse("Contraseña actualizada correctamente")
	common.WriteJSONResponse(w, successResponse, 200)
} */
