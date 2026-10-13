package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"api_go/internal/core/roles"
	"api_go/internal/infrastructure/database/models"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/utils"

	"gorm.io/gorm"
)

type RolesAdapter struct {
	db           *gorm.DB
	redisService *redis.Cache
}

func NewRolesAdapter(db *gorm.DB, redisService *redis.Cache) *RolesAdapter {
	return &RolesAdapter{
		db:           db,
		redisService: redisService,
	}
}

// CrearRol implementa el puerto RolesPort
func (a *RolesAdapter) CrearRol(ctx context.Context, rolData roles.RolData) (*roles.Rol, error) {
	// Verificar si el rol ya existe
	var rolExistente models.Rol
	err := a.db.WithContext(ctx).Where("nombre_rol = ?", rolData.Nombre).First(&rolExistente).Error
	if err == nil {
		return nil, fmt.Errorf("status_cod:409, data:Ya existe un rol con el nombre '%s'", rolData.Nombre)
	} else if err != gorm.ErrRecordNotFound {
		return nil, a.handleQueryError(err, "verificando rol existente")
	}

	// Obtener permisos basados en la lista recibida
	permisosIDs, err := a.permisosIDs(ctx, rolData.Permisos)
	if err != nil {
		return nil, err
	}

	rolDB := models.Rol{
		NombreRol:   rolData.Nombre,
		Descripcion: rolData.Descripcion,
	}

	// Usar a.db.Create en lugar de tx.Create
	if err := a.db.WithContext(ctx).Create(&rolDB).Error; err != nil {
		return nil, a.handleCreateError(err, rolData.Nombre)
	}

	// El resto en transacción si es necesario
	tx := a.db.WithContext(ctx).Begin()

	// Asignar permisos al rol
	for _, permisoID := range permisosIDs {
		rolPermiso := models.RolXPermiso{
			IDRol:     rolDB.ID,
			IDPermiso: permisoID,
		}
		if err := tx.Create(&rolPermiso).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("Error asignando permisos")
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("Error al guardar los permisos del rol")
	}

	// Limpiar cache
	a.redisService.Delete(ctx, "roles:lista")
	a.redisService.Delete(ctx, "permisos:lista")

	// Obtener el rol creado con sus permisos
	rolCompleto, err := a.obtenerRolConPermisos(ctx, rolDB.ID)
	if err != nil {
		return nil, err
	}

	return rolCompleto, nil
}

// ObtenerRoles implementa el puerto RolesPort
func (a *RolesAdapter) ObtenerRoles(ctx context.Context) ([]roles.Rol, error) {
	cacheKey := "roles:lista"

	// Intentar obtener del cache
	cachedRoles, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedRoles != "" {
		var rolesCache []roles.Rol
		if err := json.Unmarshal([]byte(cachedRoles), &rolesCache); err == nil {
			return rolesCache, nil
		}
	}

	// Consultar base de datos usando el modelo existente
	var rolesDB []models.Rol
	err = a.db.WithContext(ctx).Find(&rolesDB).Error

	if err != nil {
		return nil, a.handleQueryError(err, "consultando roles")
	}

	if len(rolesDB) == 0 {
		return []roles.Rol{}, nil
	}

	// Obtener permisos para cada rol
	rolesList := make([]roles.Rol, len(rolesDB))
	for i, rolDB := range rolesDB {
		rolCompleto, err := a.obtenerRolConPermisos(ctx, rolDB.ID)
		if err != nil {
			return nil, err
		}
		rolesList[i] = *rolCompleto
	}

	// Guardar en cache
	rolesJSON, err := json.Marshal(rolesList)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(rolesJSON), 3600)
	}

	return rolesList, nil
}

// ObtenerRolXid implementa el puerto RolesPort
func (a *RolesAdapter) ObtenerRolXid(ctx context.Context, rolData roles.RolDataXid) (*roles.Rol, error) {
	cacheKey := fmt.Sprintf("rol:%d", rolData.ID)

	// Intentar obtener del cache
	cachedRol, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedRol != "" {
		var rolCache roles.Rol
		if err := json.Unmarshal([]byte(cachedRol), &rolCache); err == nil {
			return &rolCache, nil
		}
	}

	// Consultar base de datos usando el modelo existente
	var rolDB models.Rol
	err = a.db.WithContext(ctx).Where("id = ?", rolData.ID).First(&rolDB).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, a.handleQueryError(err, "consultando rol")
	}

	rolCompleto, err := a.obtenerRolConPermisos(ctx, rolDB.ID)
	if err != nil {
		return nil, err
	}

	// Guardar en cache
	rolJSON, err := json.Marshal(rolCompleto)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(rolJSON), 3600)
	}

	return rolCompleto, nil
}

// ActualizarRol implementa el puerto RolesPort
func (a *RolesAdapter) ActualizarRol(ctx context.Context, rolData roles.RolDataUpdate) (*roles.Rol, error) {
	// Verificar si el rol existe
	var rolExistente models.Rol
	err := a.db.WithContext(ctx).Where("id = ?", rolData.ID).First(&rolExistente).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("status_cod:404, data:El rol solicitado no existe")
		}
		return nil, a.handleQueryError(err, "verificando rol existente")
	}

	// Verificar nombre duplicado solo si se está actualizando el nombre
	if rolData.Nombre != nil {
		var rolConMismoNombre models.Rol
		err = a.db.WithContext(ctx).Where("nombre_rol = ? AND id != ?", *rolData.Nombre, rolData.ID).First(&rolConMismoNombre).Error
		if err == nil {
			return nil, fmt.Errorf("status_cod:409, data:Ya existe otro rol con el nombre '%s'", *rolData.Nombre)
		} else if err != gorm.ErrRecordNotFound {
			return nil, a.handleQueryError(err, "verificando nombre duplicado")
		}
	}

	// Transacción
	tx := a.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Actualizar solo los campos proporcionados
	updates := make(map[string]interface{})
	if rolData.Nombre != nil {
		updates["nombre_rol"] = *rolData.Nombre
	}
	if rolData.Descripcion != nil {
		updates["descripcion"] = *rolData.Descripcion
	}

	if len(updates) > 0 {
		if err := tx.Model(&models.Rol{}).Where("id = ?", rolData.ID).Updates(updates).Error; err != nil {
			tx.Rollback()
			return nil, a.handleUpdateError(err, "actualizando rol")
		}
	}

	// Actualizar permisos solo si se proporcionaron
	if rolData.Permisos != nil {
		// Eliminar permisos actuales
		if err := tx.Where("id_rol = ?", rolData.ID).Delete(&models.RolXPermiso{}).Error; err != nil {
			tx.Rollback()
			return nil, a.handleUpdateError(err, "eliminando permisos anteriores")
		}

		// Validar y asignar nuevos permisos
		permisosIDs, err := a.permisosIDs(ctx, rolData.Permisos)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		for _, permisoID := range permisosIDs {
			rolPermiso := models.RolXPermiso{
				IDRol:     rolData.ID,
				IDPermiso: permisoID,
			}
			if err := tx.Create(&rolPermiso).Error; err != nil {
				tx.Rollback()
				return nil, a.handleUpdateError(err, "asignando nuevos permisos")
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("Error al actualizar el rol: %v", err)
	}

	// Limpiar cache
	a.redisService.Delete(ctx, "roles:lista")
	a.redisService.Delete(ctx, fmt.Sprintf("rol:%d", rolData.ID))
	a.redisService.Delete(ctx, "permisos:lista")

	// Obtener el rol actualizado
	rolActualizado, err := a.obtenerRolConPermisos(ctx, rolData.ID)
	if err != nil {
		return nil, err
	}

	return rolActualizado, nil
}

// EliminarRol implementa el puerto RolesPort
func (a *RolesAdapter) EliminarRol(ctx context.Context, rolData roles.RolDataXid) error {
	// Verificar si el rol existe
	var rolExistente models.Rol
	err := a.db.WithContext(ctx).Where("id = ?", rolData.ID).First(&rolExistente).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("status_cod:404, data:El rol solicitado no existe")
		}
		return a.handleQueryError(err, "verificando rol existente")
	}

	// Verificar si hay usuarios usando este rol
	var countUsuarios int64
	err = a.db.WithContext(ctx).Model(&models.Usuario{}).Where("id_rol = ?", rolData.ID).Count(&countUsuarios).Error
	if err != nil {
		return a.handleDeleteError(err, rolData.ID)
	}

	if countUsuarios > 0 {
		return fmt.Errorf("No se puede eliminar el rol porque tiene usuarios asociados")
	}

	// Crear transacción
	tx := a.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Eliminar permisos del rol
	if err := tx.Where("id_rol = ?", rolData.ID).Delete(&models.RolXPermiso{}).Error; err != nil {
		tx.Rollback()
		return a.handleDeleteError(err, rolData.ID)
	}

	// Eliminar el rol
	result := tx.Where("id = ?", rolData.ID).Delete(&models.Rol{})
	if result.Error != nil {
		tx.Rollback()
		return a.handleDeleteError(result.Error, rolData.ID)
	}

	if result.RowsAffected == 0 {
		tx.Rollback()
		return fmt.Errorf("status_cod:404, data:El rol con ID %d no existe", rolData.ID)
	}

	// Commit de la transacción
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("Error al eliminar el rol: %v", err)
	}

	// Limpiar cache
	a.redisService.Delete(ctx, "roles:lista")
	a.redisService.Delete(ctx, fmt.Sprintf("rol:%d", rolData.ID))
	a.redisService.Delete(ctx, "permisos:lista")

	return nil
}

// ObtenerTodosLosPermisos implementa el puerto RolesPort
func (a *RolesAdapter) ObtenerTodosLosPermisos(ctx context.Context) ([]string, error) {
	cacheKey := "permisos:lista"

	// Intentar obtener del cache
	cachedPermisos, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedPermisos != "" {
		var permisosCache []string
		if err := json.Unmarshal([]byte(cachedPermisos), &permisosCache); err == nil {
			return permisosCache, nil
		}
	}

	// Consultar base de datos usando el modelo existente
	var permisosDB []models.Permiso
	err = a.db.WithContext(ctx).Find(&permisosDB).Error

	if err != nil {
		return nil, a.handleQueryError(err, "consultando permisos")
	}

	permisos := make([]string, len(permisosDB))
	for i, permisoDB := range permisosDB {
		permisos[i] = permisoDB.NombrePermiso
	}

	// Guardar en cache
	permisosJSON, err := json.Marshal(permisos)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(permisosJSON), 3600)
	}

	return permisos, nil
}

// obtenerPermisosIDs obtiene los IDs de permisos basados en los nombres
func (a *RolesAdapter) obtenerPermisosIDs(ctx context.Context, permisosNombres []string) ([]int, error) {
	if len(permisosNombres) == 0 {
		return []int{}, nil
	}

	// Si el primer permiso es "All", obtener todos los permisos
	if len(permisosNombres) == 1 && permisosNombres[0] == "All" {
		var todosPermisos []models.Permiso
		err := a.db.WithContext(ctx).Find(&todosPermisos).Error
		if err != nil {
			return nil, a.handleQueryError(err, "obteniendo todos los permisos")
		}

		permisosIDs := make([]int, len(todosPermisos))
		for i, permiso := range todosPermisos {
			permisosIDs[i] = permiso.ID
		}
		return permisosIDs, nil
	}

	// Obtener IDs de permisos específicos
	var permisosDB []models.Permiso
	err := a.db.WithContext(ctx).
		Where("nombre_permiso IN ?", permisosNombres).
		Find(&permisosDB).Error

	if err != nil {
		return nil, a.handleQueryError(err, "obteniendo IDs de permisos")
	}

	// Verificar que todos los permisos solicitados existen
	permisosEncontrados := make(map[string]bool)
	permisosIDs := make([]int, len(permisosDB))

	for i, permisoDB := range permisosDB {
		permisosEncontrados[permisoDB.NombrePermiso] = true
		permisosIDs[i] = permisoDB.ID
	}

	// Verificar permisos no encontrados
	var permisosNoEncontrados []string
	for _, permisoNombre := range permisosNombres {
		if !permisosEncontrados[permisoNombre] {
			permisosNoEncontrados = append(permisosNoEncontrados, permisoNombre)
		}
	}

	if len(permisosNoEncontrados) > 0 {
		return nil, fmt.Errorf("Los siguientes permisos no existen: %s", strings.Join(permisosNoEncontrados, ", "))
	}

	return permisosIDs, nil
}

// obtenerRolConPermisos obtiene un rol con sus permisos
func (a *RolesAdapter) obtenerRolConPermisos(ctx context.Context, rolID int) (*roles.Rol, error) {
	var rolDB models.Rol
	err := a.db.WithContext(ctx).Where("id = ?", rolID).First(&rolDB).Error
	if err != nil {
		return nil, a.handleQueryError(err, "obteniendo rol")
	}

	// Obtener permisos del rol
	var permisosDB []models.Permiso
	err = a.db.WithContext(ctx).
		Table("permisos p").
		Select("p.nombre_permiso").
		Joins("INNER JOIN rol_x_permisos rxp ON p.id = rxp.id_permiso").
		Where("rxp.id_rol = ?", rolID).
		Find(&permisosDB).Error

	if err != nil {
		return nil, a.handleQueryError(err, "obteniendo permisos del rol")
	}

	permisos := make([]string, len(permisosDB))
	for i, permisoDB := range permisosDB {
		permisos[i] = permisoDB.NombrePermiso
	}

	return &roles.Rol{
		ID:          rolDB.ID,
		Nombre:      rolDB.NombreRol,
		Descripcion: rolDB.Descripcion,
		Permisos:    permisos,
	}, nil
}

// Manejo de errores (sin cambios)
func (a *RolesAdapter) handleCreateError(err error, nombre string) error {
	errStr := err.Error()

	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "Duplicate") {
		validacion := utils.ValidarExistente("P2002", nombre)
		if !validacion.OK {
			return fmt.Errorf(validacion.Data)
		}
	}

	return fmt.Errorf("Ocurrió un error creando el rol")
}

func (a *RolesAdapter) handleQueryError(err error, operation string) error {
	return fmt.Errorf("Ocurrió un error %s", operation)
}

func (a *RolesAdapter) handleUpdateError(err error, nombre string) error {
	errStr := err.Error()

	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "Duplicate") {
		validacion := utils.ValidarExistente("P2002", nombre)
		if !validacion.OK {
			return fmt.Errorf("status_cod:409, data:%s", validacion.Data)
		}
	}

	return fmt.Errorf("Ocurrió un error actualizando el rol")
}

func (a *RolesAdapter) handleDeleteError(err error, id int) error {
	errStr := err.Error()

	if strings.Contains(errStr, "foreign") || strings.Contains(errStr, "constraint") {
		return fmt.Errorf("No se puede eliminar el rol porque tiene registros asociados")
	}

	return fmt.Errorf("Ocurrió un error eliminando el rol")
}

func (a *RolesAdapter) permisosIDs(ctx context.Context, permisosIDs []int) ([]int, error) {
	if len(permisosIDs) == 0 {
		return nil, fmt.Errorf("Se requiere al menos un permiso")
	}

	// Verificar que todos los IDs existen
	var count int64
	err := a.db.WithContext(ctx).Model(&models.Permiso{}).
		Where("id IN ?", permisosIDs).
		Count(&count).Error

	if err != nil {
		return nil, a.handleQueryError(err, "validando permisos")
	}

	if int(count) != len(permisosIDs) {
		return nil, fmt.Errorf("Uno o más permisos no existen")
	}

	return permisosIDs, nil
}
