// internal/interfaces/api/handlers/roles/handler.go
package roles

import (
	"encoding/json"
	"net/http"
	"strconv"

	coreRoles "api_go/internal/core/roles"
	"api_go/internal/interfaces/api/common"
	rolesDTOs "api_go/internal/interfaces/api/roles"
	"api_go/pkg/logger"
	"api_go/pkg/utils"
)

type RolesHandler struct {
	rolService *coreRoles.RolService
}

func NewRolesHandler(rolService *coreRoles.RolService) *RolesHandler {
	return &RolesHandler{
		rolService: rolService,
	}
}

func (h *RolesHandler) ObtenerRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	roles, err := h.rolService.ObtenerRoles(ctx)
	if err != nil {
		logger.Error("❌ Error obteniendo roles", "error", err)
		utils.WriteValidationError(w, err, 500)
		return
	}

	response := common.NewSuccessResponse(rolesDTOs.FromRoles(roles))
	common.WriteJSONResponse(w, response, 200)
}

func (h *RolesHandler) ObtenerRolXid(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logger.Error("❌ ID de rol inválido", "id", idStr, "error", err)
		response := common.NewErrorResponse(400, "ID de rol inválido")
		common.WriteJSONResponse(w, response, 400)
		return
	}

	ctx := r.Context()
	rolData := coreRoles.RolDataXid{ID: id}

	rol, err := h.rolService.ObtenerRolXid(ctx, rolData)
	if err != nil {
		logger.Error("❌ Error obteniendo rol", "id", id, "error", err)
		utils.WriteValidationError(w, err, 500)
		return
	}

	if rol == nil {
		response := common.NewErrorResponse(404, "Rol no encontrado")
		common.WriteJSONResponse(w, response, 404)
		return
	}

	response := common.NewSuccessResponse(rolesDTOs.FromRol(rol))
	common.WriteJSONResponse(w, response, 200)
}

func (h *RolesHandler) CrearRol(w http.ResponseWriter, r *http.Request) {
	var req rolesDTOs.CreateRolRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("❌ Error decodificando solicitud", "error", err)
		response := common.NewErrorResponse(400, "Solicitud inválida")
		common.WriteJSONResponse(w, response, 400)
		return
	}

	ctx := r.Context()
	rolData := rolesDTOs.ToRolData(req) // Usar adapter

	rol, err := h.rolService.CrearRol(ctx, rolData)
	if err != nil {
		logger.Error("❌ Error creando rol", "error", err, "nombre", req.Nombre)
		utils.WriteValidationError(w, err, 400)
		return
	}

	response := common.NewSuccessResponse(rolesDTOs.FromRol(rol))
	common.WriteJSONResponse(w, response, 201)
}

func (h *RolesHandler) ActualizarRol(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logger.Error("❌ ID de rol inválido", "id", idStr, "error", err)
		response := common.NewErrorResponse(400, "ID de rol inválido")
		common.WriteJSONResponse(w, response, 400)
		return
	}

	var req rolesDTOs.UpdateRolRequest // Usar DTO definido
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("❌ Error decodificando solicitud", "error", err)
		response := common.NewErrorResponse(400, "Solicitud inválida")
		common.WriteJSONResponse(w, response, 400)
		return
	}

	// Asegurar que el ID de la URL coincide con el del body
	req.ID = id

	ctx := r.Context()
	rolData := rolesDTOs.ToRolDataUpdate(req)

	_, err = h.rolService.ActualizarRol(ctx, rolData)
	if err != nil {
		logger.Error("❌ Error actualizando rol", "error", err, "id", id)
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	successResponse := common.NewSuccessResponse("Se ha actualizado el rol correctamente")
	common.WriteJSONResponse(w, successResponse, 200)
}

func (h *RolesHandler) EliminarRol(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		logger.Error("❌ ID de rol inválido", "id", idStr, "error", err)
		response := common.NewErrorResponse(400, "ID de rol inválido")
		common.WriteJSONResponse(w, response, 400)
		return
	}

	ctx := r.Context()
	rolData := coreRoles.RolDataXid{ID: id}

	err = h.rolService.EliminarRol(ctx, rolData)
	if err != nil {
		logger.Error("❌ Error eliminando rol", "error", err, "id", id)
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	response := common.NewSuccessResponse("Rol eliminado correctamente")
	common.WriteJSONResponse(w, response, 200)
}

func (h *RolesHandler) ObtenerPermisos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	permisos, err := h.rolService.ObtenerTodosLosPermisos(ctx)
	if err != nil {
		logger.Error("❌ Error obteniendo permisos", "error", err)
		utils.WriteValidationError(w, err, 500)
		return
	}

	response := common.NewSuccessResponse(permisos)
	common.WriteJSONResponse(w, response, 200)
}
