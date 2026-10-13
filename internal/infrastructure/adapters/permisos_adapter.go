package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"api_go/internal/core/permisos"
	"api_go/internal/infrastructure/database/models"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/utils"

	"gorm.io/gorm"
)

type PermisosAdapter struct {
	db           *gorm.DB
	redisService *redis.Cache
}

func NewPermisosAdapter(db *gorm.DB, redisService *redis.Cache) *PermisosAdapter {
	return &PermisosAdapter{
		db:           db,
		redisService: redisService,
	}
}

// CrearPermisos implementa el puerto PermisosPort
func (a *PermisosAdapter) CrearPermisos(ctx context.Context, permisoData permisos.PermisoData) (*permisos.Permiso, error) {
	// VALIDAR QUE LA CONEXIÓN A BD NO SEA NIL
	if a.db == nil {
		return nil, fmt.Errorf("error de configuración: conexión a base de datos no disponible")
	}

	// VALIDAR QUE REDIS NO SEA NIL
	if a.redisService == nil {
		return nil, fmt.Errorf("error de configuración: servicio de cache no disponible")
	}

	descripcion := ""
	if permisoData.Descripcion != nil {
		descripcion = *permisoData.Descripcion
	}

	// Usar el modelo existente models.Permiso
	permisoDB := models.Permiso{
		NombrePermiso: permisoData.Nombre,
		Descripcion:   descripcion,
	}

	err := a.db.WithContext(ctx).Create(&permisoDB).Error
	if err != nil {
		return nil, a.handleCreateError(err, permisoData.Nombre)
	}

	// Limpiar cache de lista de permisos
	a.redisService.Delete(ctx, "permisos:lista")

	// Mapear a entidad del core
	return a.toPermisoEntity(&permisoDB), nil
}

// ObtenerPermisos implementa el puerto PermisosPort
func (a *PermisosAdapter) ObtenerPermisos(ctx context.Context) ([]permisos.Permiso, error) {
	cacheKey := "permisos:lista"

	// Intentar obtener del cache
	cachedPermisos, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedPermisos != "" {
		var permisosCache []permisos.Permiso
		if err := json.Unmarshal([]byte(cachedPermisos), &permisosCache); err == nil {
			return permisosCache, nil
		}
	}

	// Consultar base de datos usando models.Permiso
	var permisosDB []models.Permiso
	err = a.db.WithContext(ctx).Find(&permisosDB).Error

	if err != nil {
		return nil, a.handleQueryError(err, "consultando permisos")
	}

	if len(permisosDB) == 0 {
		return []permisos.Permiso{}, nil // Retornar slice vacío en lugar de error
	}

	// Mapear a entidades del core
	permisosList := make([]permisos.Permiso, len(permisosDB))
	for i, permisoDB := range permisosDB {
		permisosList[i] = *a.toPermisoEntity(&permisoDB)
	}

	// Guardar en cache
	permisosJSON, err := json.Marshal(permisosList)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(permisosJSON), 3600) // 1 hora
	}

	return permisosList, nil
}

// ObtenerPermisosXid implementa el puerto PermisosPort
func (a *PermisosAdapter) ObtenerPermisosXid(ctx context.Context, permisoData permisos.PermisoDataXid) (*permisos.Permiso, error) {
	cacheKey := fmt.Sprintf("permiso:%d", permisoData.ID)

	// Intentar obtener del cache
	cachedPermiso, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedPermiso != "" {
		var permisoCache permisos.Permiso
		if err := json.Unmarshal([]byte(cachedPermiso), &permisoCache); err == nil {
			return &permisoCache, nil
		}
	}

	// Consultar base de datos usando models.Permiso
	var permisoDB models.Permiso
	err = a.db.WithContext(ctx).
		Where("id = ?", permisoData.ID).
		First(&permisoDB).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Permiso no encontrado
		}
		return nil, a.handleQueryError(err, "consultando permiso")
	}

	permisoEntity := a.toPermisoEntity(&permisoDB)

	// Guardar en cache
	permisoJSON, err := json.Marshal(permisoEntity)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(permisoJSON), 3600)
	}

	return permisoEntity, nil
}

// DelPermiso implementa el puerto PermisosPort
func (a *PermisosAdapter) DelPermiso(ctx context.Context, permisoData permisos.PermisoDataXid) error {
	// Actualizar cache de lista
	cachedPermisos, err := a.redisService.Get(ctx, "permisos:lista")
	if err == nil && cachedPermisos != "" {
		var permisosCache []permisos.Permiso
		if err := json.Unmarshal([]byte(cachedPermisos), &permisosCache); err == nil {
			// Filtrar el permiso eliminado
			filtered := make([]permisos.Permiso, 0)
			for _, e := range permisosCache {
				if e.ID != permisoData.ID {
					filtered = append(filtered, e)
				}
			}
			filteredJSON, _ := json.Marshal(filtered)
			a.redisService.Set(ctx, "permisos:lista", string(filteredJSON), 3600)
		}
	}

	// Eliminar de base de datos usando models.Permiso
	result := a.db.WithContext(ctx).Where("id = ?", permisoData.ID).Delete(&models.Permiso{})
	if result.Error != nil {
		return a.handleDeleteError(result.Error, permisoData.ID)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("el permiso con ID %d no existe", permisoData.ID)
	}

	// Limpiar cache individual
	a.redisService.Delete(ctx, fmt.Sprintf("permiso:%d", permisoData.ID))

	return nil
}

// ActualizaPermiso implementa el puerto PermisosPort
func (a *PermisosAdapter) ActualizaPermiso(ctx context.Context, permisoData permisos.PermisoDataUpdate) (*permisos.Permiso, error) {
	// Verificar si el permiso existe
	var permisoExistente models.Permiso
	err := a.db.WithContext(ctx).Where("id = ?", permisoData.ID).First(&permisoExistente).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("el permiso solicitado no existe en la base de datos")
		}
		return nil, a.handleQueryError(err, "verificando permiso existente")
	}

	// Preparar updates
	updates := map[string]interface{}{
		"nombre_permiso": permisoData.Nombre,
		"descripcion":    permisoData.Descripcion,
	}

	// Actualizar en base de datos
	err = a.db.WithContext(ctx).Model(&models.Permiso{}).
		Where("id = ?", permisoData.ID).
		Updates(updates).Error

	if err != nil {
		return nil, a.handleUpdateError(err, permisoData.Nombre)
	}

	// Obtener permiso actualizado
	var permisoActualizado models.Permiso
	err = a.db.WithContext(ctx).Where("id = ?", permisoData.ID).First(&permisoActualizado).Error
	if err != nil {
		return nil, a.handleQueryError(err, "obteniendo permiso actualizado")
	}

	// Actualizar cache de lista
	cachedPermisos, err := a.redisService.Get(ctx, "permisos:lista")
	if err == nil && cachedPermisos != "" {
		var permisosCache []permisos.Permiso
		if err := json.Unmarshal([]byte(cachedPermisos), &permisosCache); err == nil {
			for i, e := range permisosCache {
				if e.ID == permisoData.ID {
					permisosCache[i] = *a.toPermisoEntity(&permisoActualizado)
					break
				}
			}
			updatedJSON, _ := json.Marshal(permisosCache)
			a.redisService.Set(ctx, "permisos:lista", string(updatedJSON), 3600)
		}
	}

	// Actualizar cache individual
	permisoJSON, _ := json.Marshal(a.toPermisoEntity(&permisoActualizado))
	a.redisService.Set(ctx, fmt.Sprintf("permiso:%d", permisoData.ID), string(permisoJSON), 3600)

	return a.toPermisoEntity(&permisoActualizado), nil
}

// Mapeo de DB a Entity
func (a *PermisosAdapter) toPermisoEntity(permisoDB *models.Permiso) *permisos.Permiso {
	return &permisos.Permiso{
		ID: permisoDB.ID,
		PermisoBase: permisos.PermisoBase{
			Nombre:      permisoDB.NombrePermiso,
			Descripcion: &permisoDB.Descripcion,
		},
	}
}

// Manejo de errores (sin cambios)
func (a *PermisosAdapter) handleCreateError(err error, nombre string) error {
	errStr := err.Error()

	// Error de duplicado
	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "Duplicate") {
		validacion := utils.ValidarExistente("P2002", nombre)
		if !validacion.OK {
			return fmt.Errorf(validacion.Data)
		}
	}

	return fmt.Errorf("Ocurrió un error creando el permiso")
}

func (a *PermisosAdapter) handleQueryError(err error, operation string) error {
	return fmt.Errorf("Ocurrió un error %s", operation)
}

func (a *PermisosAdapter) handleDeleteError(err error, id int) error {
	errStr := err.Error()

	// Error de referencia (foreign key constraint)
	if strings.Contains(errStr, "foreign") || strings.Contains(errStr, "constraint") {
		return fmt.Errorf("No se puede eliminar el permiso porque tiene registros asociados")
	}

	return fmt.Errorf("Ocurrió un error eliminando el permiso")
}

func (a *PermisosAdapter) handleUpdateError(err error, nombre string) error {
	errStr := err.Error()

	// Error de duplicado
	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "Duplicate") {
		validacion := utils.ValidarExistente("P2002", nombre)
		if !validacion.OK {
			return fmt.Errorf("status_cod:409, data:%s", validacion.Data)
		}
	}

	return fmt.Errorf("Ocurrió un error actualizando el permiso")
}
