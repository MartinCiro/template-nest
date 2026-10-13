package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"api_go/internal/core/estados"
	"api_go/internal/infrastructure/database/models"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/utils"

	"gorm.io/gorm"
)

type EstadosAdapter struct {
	db           *gorm.DB
	redisService *redis.Cache
}

func NewEstadosAdapter(db *gorm.DB, redisService *redis.Cache) *EstadosAdapter {
	return &EstadosAdapter{
		db:           db,
		redisService: redisService,
	}
}

// CrearEstados implementa el puerto EstadosPort
func (a *EstadosAdapter) CrearEstados(ctx context.Context, estadoData estados.EstadoData) (*estados.Estado, error) {
	// VALIDAR QUE LA CONEXIÓN A BD NO SEA NIL
	if a.db == nil {
		return nil, fmt.Errorf("error de configuración: conexión a base de datos no disponible")
	}

	// VALIDAR QUE REDIS NO SEA NIL
	if a.redisService == nil {
		return nil, fmt.Errorf("error de configuración: servicio de cache no disponible")
	}

	// Usar el modelo existente models.Estado
	estadoDB := models.Estado{
		NombreEstado: estadoData.Nombre, // Usar NombreEstado en lugar de Nombre
		Descripcion:  estadoData.Descripcion,
	}

	err := a.db.WithContext(ctx).Create(&estadoDB).Error
	if err != nil {
		return nil, a.handleCreateError(err, estadoData.Nombre)
	}

	// Limpiar cache de lista de estados
	a.redisService.Delete(ctx, "estados:lista")

	// Mapear a entidad del core
	return a.toEstadoEntity(&estadoDB), nil
}

// ObtenerEstados implementa el puerto EstadosPort
func (a *EstadosAdapter) ObtenerEstados(ctx context.Context) ([]estados.Estado, error) {
	cacheKey := "estados:lista"

	// Intentar obtener del cache
	cachedEstados, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedEstados != "" {
		var estadosCache []estados.Estado
		if err := json.Unmarshal([]byte(cachedEstados), &estadosCache); err == nil {
			return estadosCache, nil
		}
	}

	// Consultar base de datos usando models.Estado
	var estadosDB []models.Estado
	err = a.db.WithContext(ctx).Find(&estadosDB).Error

	if err != nil {
		return nil, a.handleQueryError(err, "consultando estados")
	}

	if len(estadosDB) == 0 {
		return []estados.Estado{}, nil // Retornar slice vacío en lugar de error
	}

	// Mapear a entidades del core
	estadosList := make([]estados.Estado, len(estadosDB))
	for i, estadoDB := range estadosDB {
		estadosList[i] = *a.toEstadoEntity(&estadoDB)
	}

	// Guardar en cache
	estadosJSON, err := json.Marshal(estadosList)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(estadosJSON), 3600) // 1 hora
	}

	return estadosList, nil
}

// ObtenerEstadosXid implementa el puerto EstadosPort
func (a *EstadosAdapter) ObtenerEstadosXid(ctx context.Context, estadoData estados.EstadoDataXid) (*estados.Estado, error) {
	cacheKey := fmt.Sprintf("estado:%d", estadoData.ID)

	// Intentar obtener del cache
	cachedEstado, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedEstado != "" {
		var estadoCache estados.Estado
		if err := json.Unmarshal([]byte(cachedEstado), &estadoCache); err == nil {
			return &estadoCache, nil
		}
	}

	// Consultar base de datos usando models.Estado
	var estadoDB models.Estado
	err = a.db.WithContext(ctx).
		Where("id = ?", estadoData.ID).
		First(&estadoDB).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Estado no encontrado
		}
		return nil, a.handleQueryError(err, "consultando estado")
	}

	estadoEntity := a.toEstadoEntity(&estadoDB)

	// Guardar en cache
	estadoJSON, err := json.Marshal(estadoEntity)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(estadoJSON), 3600)
	}

	return estadoEntity, nil
}

// DelEstado implementa el puerto EstadosPort
func (a *EstadosAdapter) DelEstado(ctx context.Context, estadoData estados.EstadoDataXid) error {
	// Actualizar cache de lista
	cachedEstados, err := a.redisService.Get(ctx, "estados:lista")
	if err == nil && cachedEstados != "" {
		var estadosCache []estados.Estado
		if err := json.Unmarshal([]byte(cachedEstados), &estadosCache); err == nil {
			// Filtrar el estado eliminado
			filtered := make([]estados.Estado, 0)
			for _, e := range estadosCache {
				if e.ID != estadoData.ID {
					filtered = append(filtered, e)
				}
			}
			filteredJSON, _ := json.Marshal(filtered)
			a.redisService.Set(ctx, "estados:lista", string(filteredJSON), 3600)
		}
	}

	// Eliminar de base de datos usando models.Estado
	result := a.db.WithContext(ctx).Where("id = ?", estadoData.ID).Delete(&models.Estado{})
	if result.Error != nil {
		return a.handleDeleteError(result.Error, estadoData.ID)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("el estado con ID %d no existe", estadoData.ID)
	}

	// Limpiar cache individual
	a.redisService.Delete(ctx, fmt.Sprintf("estado:%d", estadoData.ID))

	return nil
}

// ActualizaEstado implementa el puerto EstadosPort
func (a *EstadosAdapter) ActualizaEstado(ctx context.Context, estadoData estados.EstadoDataUpdate) (*estados.Estado, error) {
	// Verificar si el estado existe
	var estadoExistente models.Estado
	err := a.db.WithContext(ctx).Where("id = ?", estadoData.ID).First(&estadoExistente).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("el estado solicitado no existe en la base de datos")
		}
		return nil, a.handleQueryError(err, "verificando estado existente")
	}

	// Preparar updates
	updates := map[string]interface{}{
		"nombre_estado": estadoData.Nombre,
		"descripcion":   estadoData.Descripcion,
	}

	// Actualizar en base de datos
	err = a.db.WithContext(ctx).Model(&models.Estado{}).
		Where("id = ?", estadoData.ID).
		Updates(updates).Error

	if err != nil {
		return nil, a.handleUpdateError(err, estadoData.Nombre)
	}

	// Obtener estado actualizado
	var estadoActualizado models.Estado
	err = a.db.WithContext(ctx).Where("id = ?", estadoData.ID).First(&estadoActualizado).Error
	if err != nil {
		return nil, a.handleQueryError(err, "obteniendo estado actualizado")
	}

	// Actualizar cache de lista
	cachedEstados, err := a.redisService.Get(ctx, "estados:lista")
	if err == nil && cachedEstados != "" {
		var estadosCache []estados.Estado
		if err := json.Unmarshal([]byte(cachedEstados), &estadosCache); err == nil {
			for i, e := range estadosCache {
				if e.ID == estadoData.ID {
					estadosCache[i] = *a.toEstadoEntity(&estadoActualizado)
					break
				}
			}
			updatedJSON, _ := json.Marshal(estadosCache)
			a.redisService.Set(ctx, "estados:lista", string(updatedJSON), 3600)
		}
	}

	// Actualizar cache individual
	estadoJSON, _ := json.Marshal(a.toEstadoEntity(&estadoActualizado))
	a.redisService.Set(ctx, fmt.Sprintf("estado:%d", estadoData.ID), string(estadoJSON), 3600)

	return a.toEstadoEntity(&estadoActualizado), nil
}

// Mapeo de DB a Entity
func (a *EstadosAdapter) toEstadoEntity(estadoDB *models.Estado) *estados.Estado {
	return &estados.Estado{
		ID:          estadoDB.ID,
		Nombre:      estadoDB.NombreEstado,
		Descripcion: estadoDB.Descripcion,
	}
}

// Manejo de errores (sin cambios)
func (a *EstadosAdapter) handleCreateError(err error, nombre string) error {
	errStr := err.Error()

	// Error de duplicado
	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "Duplicate") {
		validacion := utils.ValidarExistente("P2002", nombre)
		if !validacion.OK {
			return fmt.Errorf(validacion.Data)
		}
	}

	return fmt.Errorf("Ocurrió un error creando el estado")
}

func (a *EstadosAdapter) handleQueryError(err error, operation string) error {
	return fmt.Errorf("Ocurrió un error %s", operation)
}

func (a *EstadosAdapter) handleDeleteError(err error, id int) error {
	errStr := err.Error()

	// Error de referencia (foreign key constraint)
	if strings.Contains(errStr, "foreign") || strings.Contains(errStr, "constraint") {
		return fmt.Errorf("No se puede eliminar el estado porque tiene registros asociados")
	}

	return fmt.Errorf("Ocurrió un error eliminando el estado")
}

func (a *EstadosAdapter) handleUpdateError(err error, nombre string) error {
	errStr := err.Error()

	// Error de duplicado
	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "Duplicate") {
		validacion := utils.ValidarExistente("P2002", nombre)
		if !validacion.OK {
			return fmt.Errorf("status_cod:409, data:%s", validacion.Data)
		}
	}

	return fmt.Errorf("Ocurrió un error actualizando el estado")
}
