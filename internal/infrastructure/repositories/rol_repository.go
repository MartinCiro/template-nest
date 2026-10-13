package repositories

import (
	"api_go/internal/core/auth"
	"api_go/internal/infrastructure/database"
	"api_go/internal/infrastructure/database/models"
	"api_go/pkg/logger"
	"context"
	"fmt"

	"gorm.io/gorm"
)

// Asegurar que RolRepository implemente el port
var _ auth.RolRepositoryPort = (*RolRepository)(nil)

type RolRepository struct {
	dbManager *database.DBManager
}

func NewRolRepository(dbManager *database.DBManager) *RolRepository {
	return &RolRepository{
		dbManager: dbManager,
	}
}

// FindByID busca un rol por ID
func (r *RolRepository) FindByID(ctx context.Context, id int) (*models.Rol, error) {
	var rol models.Rol

	err := r.dbManager.GetDB().WithContext(ctx).
		First(&rol, id).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error("❌ Rol no encontrado", "ID", id)
			return nil, nil
		}
		logger.Error("❌ Error buscando rol por ID", "error", err)
		return nil, fmt.Errorf("error buscando rol por ID: %v", err)
	}

	return &rol, nil
}

// FindRolIDByName busca un rol por nombre y retorna su ID
func (r *RolRepository) FindRolIDByName(ctx context.Context, nombre string) (int, error) {
	var rol models.Rol

	err := r.dbManager.GetDB().WithContext(ctx).
		Model(&models.Rol{}).
		Where("nombre_rol = ?", nombre).
		First(&rol).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error("❌ Rol no encontrado", "nombre", nombre)
			return 0, nil
		}
		logger.Error("❌ Error buscando rol por nombre", "error", err)
		return 0, fmt.Errorf("error buscando rol por nombre: %v", err)
	}

	return rol.ID, nil
}

// FindAll busca todos los roles
func (r *RolRepository) FindAll(ctx context.Context) ([]models.Rol, error) {
	var roles []models.Rol

	err := r.dbManager.GetDB().WithContext(ctx).
		Find(&roles).Error

	if err != nil {
		logger.Error("❌ Error buscando todos los roles", "error", err)
		return nil, fmt.Errorf("error buscando todos los roles: %v", err)
	}

	return roles, nil
}

// FindByNombre busca roles por nombre (búsqueda parcial)
func (r *RolRepository) FindByNombre(ctx context.Context, nombre string) ([]models.Rol, error) {
	var roles []models.Rol

	err := r.dbManager.GetDB().WithContext(ctx).
		Where("nombre_rol LIKE ?", "%"+nombre+"%").
		Find(&roles).Error

	if err != nil {
		logger.Error("❌ Error buscando roles por nombre", "error", err)
		return nil, fmt.Errorf("error buscando roles por nombre: %v", err)
	}

	return roles, nil
}

// FindByNombreExacto busca un rol por nombre exacto
func (r *RolRepository) FindByExactName(ctx context.Context, nombre string) (*models.Rol, error) {
	var rol models.Rol

	err := r.dbManager.GetDB().WithContext(ctx).
		Where("nombre_rol = ?", nombre).
		First(&rol).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Debug("🔍 Rol no encontrado", "nombre", nombre)
			return nil, nil
		}
		logger.Error("❌ Error buscando rol por nombre exacto", "error", err)
		return nil, fmt.Errorf("error buscando rol por nombre exacto: %v", err)
	}

	logger.Debug("Rol encontrado", "nombre", nombre, "id", rol.ID)
	return &rol, nil
}

// FindOrCreateInvitado busca el rol "invitado" y si no existe, lo crea
func (r *RolRepository) FindOrCreateGuest(ctx context.Context) (int, error) {
	const rolInvitado = "invitado"

	// Buscamos por nombre exacto
	rol, err := r.FindByExactName(ctx, rolInvitado)
	if err != nil {
		return 0, err
	}

	// Si existe, retornamos el ID
	if rol != nil {
		logger.Info("Rol invitado encontrado", "id", rol.ID)
		return rol.ID, nil
	}

	// Si no existe, lo creamos
	logger.Info("➕ Creando rol invitado ya que no existe")

	nuevoRol := &models.Rol{
		NombreRol:   rolInvitado,
		Descripcion: "Rol para usuarios invitados",
	}

	err = r.Create(ctx, nuevoRol)
	if err != nil {
		logger.Error("❌ Error creando rol invitado", "error", err)
		return 0, fmt.Errorf("error creando rol invitado: %v", err)
	}

	logger.Info("Rol invitado creado exitosamente", "id", nuevoRol.ID)
	return nuevoRol.ID, nil
}

// Create crea un nuevo rol
func (r *RolRepository) Create(ctx context.Context, rol *models.Rol) error {
	err := r.dbManager.GetDB().WithContext(ctx).
		Create(rol).Error

	if err != nil {
		logger.Error("❌ Error creando rol", "error", err)
		return fmt.Errorf("error creando rol: %v", err)
	}

	return nil
}

// Update actualiza un rol
func (r *RolRepository) Update(ctx context.Context, rol *models.Rol) error {
	err := r.dbManager.GetDB().WithContext(ctx).
		Save(rol).Error

	if err != nil {
		logger.Error("❌ Error actualizando rol", "error", err)
		return fmt.Errorf("error actualizando rol: %v", err)
	}

	return nil
}

// Delete elimina un rol por ID
func (r *RolRepository) Delete(ctx context.Context, id int) error {
	result := r.dbManager.GetDB().WithContext(ctx).
		Delete(&models.Rol{}, id)

	if result.Error != nil {
		logger.Error("❌ Error eliminando rol", "error", result.Error)
		return fmt.Errorf("error eliminando rol: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		logger.Error("❌ Rol no encontrado para eliminar", "ID", id)
		return fmt.Errorf("rol no encontrado")
	}

	return nil
}

// GetRolesConPermisos obtiene roles con sus permisos asociados
func (r *RolRepository) GetRolesConPermisos(ctx context.Context) ([]models.Rol, error) {
	var roles []models.Rol

	err := r.dbManager.GetDB().WithContext(ctx).
		Preload("Permisos").
		Find(&roles).Error

	if err != nil {
		logger.Error("❌ Error buscando roles con permisos", "error", err)
		return nil, fmt.Errorf("error buscando roles con permisos: %v", err)
	}

	return roles, nil
}
