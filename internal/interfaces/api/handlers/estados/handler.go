package estados

import (
	"encoding/json"
	"net/http"
	"strconv"

	coreEstados "api_go/internal/core/estados"
	"api_go/internal/interfaces/api/common"
	estadosDTOs "api_go/internal/interfaces/api/estados"
	"api_go/pkg/utils"
)

type EstadosHandler struct {
	estadoService *coreEstados.EstadoService
}

func NewEstadosHandler(estadoService *coreEstados.EstadoService) *EstadosHandler {
	return &EstadosHandler{
		estadoService: estadoService,
	}
}

func (h *EstadosHandler) ObtenerEstados(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	estados, err := h.estadoService.ObtenerEstados(ctx)
	if err != nil {
		utils.WriteValidationError(w, err, 500)
		return
	}

	response := common.NewSuccessResponse(estadosDTOs.FromEstados(estados))
	common.WriteJSONResponse(w, response, 200)
}

func (h *EstadosHandler) ObtenerEstadoXid(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	ctx := r.Context()
	estadoData := coreEstados.EstadoDataXid{ID: id}
	estado, err := h.estadoService.ObtenerEstadoXid(ctx, estadoData)
	if err != nil {
		utils.WriteValidationError(w, err, 500)
		return
	}

	if estado == nil {
		common.WriteSimpleError(w, "Estado no encontrado", 404)
		return
	}

	response := common.NewSuccessResponse(estadosDTOs.FromEstado(estado))
	common.WriteJSONResponse(w, response, 200)
}

func (h *EstadosHandler) CrearEstado(w http.ResponseWriter, r *http.Request) {
	var req estadosDTOs.CreateEstadoRequest

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
	estadoData := estadosDTOs.ToEstadoData(req)
	_, err := h.estadoService.CrearEstado(ctx, estadoData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	successResponse := common.NewSuccessResponse("Se ha creado el estado correctamente")
	common.WriteJSONResponse(w, successResponse, 201)
}

func (h *EstadosHandler) ActualizarEstado(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	var req estadosDTOs.UpdateEstadoRequest
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
	estadoData := estadosDTOs.ToEstadoDataUpdate(req)

	_, err = h.estadoService.UpEstado(ctx, estadoData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	successResponse := common.NewSuccessResponse("Se ha actualizado el estado correctamente")
	common.WriteJSONResponse(w, successResponse, 200)
}

func (h *EstadosHandler) EliminarEstado(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 { // Validar que sea mayor a 0
		common.WriteSimpleError(w, "ID inválido", 400)
		return
	}

	ctx := r.Context()
	estadoData := coreEstados.EstadoDataXid{ID: id}
	err = h.estadoService.DelEstado(ctx, estadoData)
	if err != nil {
		common.WriteSimpleError(w, err.Error(), 400)
		return
	}

	response := common.NewSuccessResponse("Estado eliminado correctamente")
	common.WriteJSONResponse(w, response, 200)
}
