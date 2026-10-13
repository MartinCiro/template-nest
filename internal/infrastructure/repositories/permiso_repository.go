package repositories

import (
	"api_go/internal/infrastructure/database"
	"api_go/internal/infrastructure/database/models"
	"context"
	"fmt"

	"api_go/pkg/logger"

	"gorm.io/gorm"
)

type PermisoRepository struct {
	dbManager *database.DBManager
}

func NewPermisoRepository(dbManager *database.DBManager) *PermisoRepository {
	return &PermisoRepository{
		dbManager: dbManager,
	}
}

func (r *PermisoRepository) FindByRolID(ctx context.Context, rolID int) ([]string, error) {
	var permisos []models.Permiso // Usar el modelo existente

	err := r.dbManager.GetDB().WithContext(ctx).
		Model(&models.Permiso{}).
		Select("permisos.nombre_permiso").
		Joins("INNER JOIN rol_x_permisos ON permisos.id = rol_x_permisos.id_permiso").
		Where("rol_x_permisos.id_rol = ?", rolID).
		Find(&permisos).Error

	if err != nil {
		return nil, fmt.Errorf("error buscando permisos: %v", err)
	}

	// Extraer solo los nombres de los permisos
	permisoNombres := make([]string, len(permisos))
	for i, permiso := range permisos {
		permisoNombres[i] = permiso.NombrePermiso
	}
	return permisoNombres, nil
}

// FindByID busca un permiso por ID
func (r *PermisoRepository) FindByID(ctx context.Context, id int) (*models.Permiso, error) {
	logger.Info("🔍 Buscando permiso por ID", "ID", id)

	var permiso models.Permiso

	err := r.dbManager.GetDB().WithContext(ctx).
		First(&permiso, id).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error("❌ Permiso no encontrado", "ID", id)
			return nil, nil
		}
		logger.Error("❌ Error buscando permiso por ID", "error", err)
		return nil, fmt.Errorf("error buscando permiso por ID: %v", err)
	}

	logger.Info("Permiso encontrado", "ID", permiso.ID, "nombre", permiso.NombrePermiso)
	return &permiso, nil
}

// FindAll busca todos los permisos
func (r *PermisoRepository) FindAll(ctx context.Context) ([]models.Permiso, error) {
	logger.Info("🔍 Buscando todos los permisos")

	var permisos []models.Permiso // Usar el modelo existente

	err := r.dbManager.GetDB().WithContext(ctx).
		Find(&permisos).Error

	if err != nil {
		logger.Error("❌ Error buscando todos los permisos", "error", err)
		return nil, fmt.Errorf("error buscando todos los permisos: %v", err)
	}

	logger.Info("Permisos encontrados", "count", len(permisos))
	return permisos, nil
}

// FindByNombre busca permisos por nombre
func (r *PermisoRepository) FindByNombre(ctx context.Context, nombre string) ([]models.Permiso, error) {
	logger.Info("🔍 Buscando permisos por nombre", "nombre", nombre)

	var permisos []models.Permiso // Usar el modelo existente

	err := r.dbManager.GetDB().WithContext(ctx).
		Where("nombre_permiso LIKE ?", "%"+nombre+"%").
		Find(&permisos).Error

	if err != nil {
		logger.Error("❌ Error buscando permisos por nombre", "error", err)
		return nil, fmt.Errorf("error buscando permisos por nombre: %v", err)
	}

	logger.Info("Permisos encontrados por nombre", "count", len(permisos))
	return permisos, nil
}

// Create crea un nuevo permiso
func (r *PermisoRepository) Create(ctx context.Context, permiso *models.Permiso) error {
	logger.Info("🔍 Creando nuevo permiso", "nombre", permiso.NombrePermiso)

	err := r.dbManager.GetDB().WithContext(ctx).
		Create(permiso).Error

	if err != nil {
		logger.Error("❌ Error creando permiso", "error", err)
		return fmt.Errorf("error creando permiso: %v", err)
	}

	logger.Info("Permiso creado", "ID", permiso.ID, "nombre", permiso.NombrePermiso)
	return nil
}

// AsignarPermisoARol asigna un permiso a un rol
func (r *PermisoRepository) AsignarPermisoARol(ctx context.Context, rolID int, permisoID int) error {
	logger.Info("Asignando permiso a rol", "rolID", rolID, "permisoID", permisoID)

	rolXPermiso := models.RolXPermiso{
		IDRol:     rolID,
		IDPermiso: permisoID,
	}

	err := r.dbManager.GetDB().WithContext(ctx).
		Create(&rolXPermiso).Error

	if err != nil {
		logger.Error("❌ Error asignando permiso al rol", "error", err)
		return fmt.Errorf("error asignando permiso al rol: %v", err)
	}

	logger.Info("Permiso asignado a rol", "rolID", rolID, "permisoID", permisoID)
	return nil
}

// RemoverPermisoDeRol remueve un permiso de un rol
func (r *PermisoRepository) RemoverPermisoDeRol(ctx context.Context, rolID int, permisoID int) error {
	logger.Info("Removiendo permiso de rol", "rolID", rolID, "permisoID", permisoID)

	result := r.dbManager.GetDB().WithContext(ctx).
		Where("id_rol = ? AND id_permiso = ?", rolID, permisoID).
		Delete(&models.RolXPermiso{})

	if result.Error != nil {
		logger.Error("❌ Error removiendo permiso del rol", "error", result.Error)
		return fmt.Errorf("error removiendo permiso del rol: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		logger.Error("❌ Asignación no encontrada", "rolID", rolID, "permisoID", permisoID)
		return fmt.Errorf("asignación no encontrada")
	}
	logger.Info("Permiso removido del rol", "rolID", rolID, "permisoID", permisoID)
	return nil
}

func (r *PermisoRepository) Update(ctx context.Context, permiso *models.Permiso) error {
	logger.Info("🔍 Actualizando permiso", "ID", permiso.ID)
	err := r.dbManager.GetDB().WithContext(ctx).
		Save(permiso).Error

	if err != nil {
		logger.Error("❌ Error actualizando permiso", "error", err)
		return fmt.Errorf("error actualizando permiso: %v", err)
	}

	logger.Info("Permiso actualizado", "ID", permiso.ID)
	return nil
}

// Delete elimina un permiso por ID
func (r *PermisoRepository) Delete(ctx context.Context, id int) error {
	logger.Info("🔍 Eliminando permiso", "ID", id)

	result := r.dbManager.GetDB().WithContext(ctx).
		Delete(&models.Permiso{}, id)

	if result.Error != nil {
		logger.Error("❌ Error eliminando permiso", "error", result.Error)
		return fmt.Errorf("error eliminando permiso: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		logger.Error("❌ Permiso no encontrado para eliminar", "ID", id)
		return fmt.Errorf("permiso no encontrado")
	}

	logger.Info("Permiso eliminado", "ID", id)
	return nil
}
