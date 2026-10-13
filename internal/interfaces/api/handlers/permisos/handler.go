package permisos

import (
	"encoding/json"
	"net/http"
	"strconv"

	corePermisos "api_go/internal/core/permisos"
	"api_go/internal/interfaces/api/common"
	permisosDTOs "api_go/internal/interfaces/api/permisos"
	"api_go/pkg/utils"
)

type PermisosHandler struct {
	permisoService *corePermisos.PermisoService
}

func NewPermisosHandler(permisoService *corePermisos.PermisoService) *PermisosHandler {
	return &PermisosHandler{
		permisoService: permisoService,
	}
}

func (h *PermisosHandler) ObtenerPermisos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	permisos, err := h.permisoService.ObtenerPermisos(ctx)
	if err != nil {
		utils.WriteValidationError(w, err, 500)
		return
	}

	response := common.NewSuccessResponse(permisosDTOs.FromPermisos(permisos))
	common.WriteJSONResponse(w, response, 200)
}

func (h *PermisosHandler) ObtenerPermisoXid(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	ctx := r.Context()
	permisoData := corePermisos.PermisoDataXid{ID: id}
	permiso, err := h.permisoService.ObtenerPermisoXid(ctx, permisoData)
	if err != nil {
		utils.WriteValidationError(w, err, 500)
		return
	}

	if permiso == nil {
		common.WriteSimpleError(w, "Permiso no encontrado", 404)
		return
	}

	response := common.NewSuccessResponse(permisosDTOs.FromPermiso(permiso))
	common.WriteJSONResponse(w, response, 200)
}

func (h *PermisosHandler) CrearPermiso(w http.ResponseWriter, r *http.Request) {
	var req permisosDTOs.CreatePermisoRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.WriteSimpleError(w, "Solicitud inválida: formato JSON incorrecto", 400)
		return
	}

	// VALIDAR EL DTO ANTES de procesar
	if errors := common.ValidateRequest(req); errors != nil {
		common.WriteValidationErrors(w, errors, 400)
		return
	}

	ctx := r.Context()
	permisoData := permisosDTOs.ToPermisoData(req)
	_, err := h.permisoService.CrearPermiso(ctx, permisoData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	successResponse := common.NewSuccessResponse("Se ha creado el permiso correctamente")
	common.WriteJSONResponse(w, successResponse, 201)
}

func (h *PermisosHandler) ActualizarPermiso(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	var req permisosDTOs.UpdatePermisoRequest
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
	permisoData := permisosDTOs.ToPermisoDataUpdate(req)

	_, err = h.permisoService.UpPermiso(ctx, permisoData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	successResponse := common.NewSuccessResponse("Se ha actualizado el permiso correctamente")
	common.WriteJSONResponse(w, successResponse, 200)
}

func (h *PermisosHandler) EliminarPermiso(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 { // Validar que sea mayor a 0
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	ctx := r.Context()
	permisoData := corePermisos.PermisoDataXid{ID: id}
	err = h.permisoService.DelPermiso(ctx, permisoData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	response := common.NewSuccessResponse("Permiso eliminado correctamente")
	common.WriteJSONResponse(w, response, 200)
}
