package repositories

import (
	"api_go/internal/core/auth"
	"api_go/internal/infrastructure/database"
	"api_go/internal/infrastructure/database/models"
	"context"
	"fmt"

	"api_go/pkg/logger"

	"gorm.io/gorm"
)

// Asegurar que EstadoRepository implemente el port
var _ auth.EstadoRepositoryPort = (*EstadoRepository)(nil)

type EstadoRepository struct {
	dbManager *database.DBManager
}

func NewEstadoRepository(dbManager *database.DBManager) *EstadoRepository {
	return &EstadoRepository{
		dbManager: dbManager,
	}
}

// FindEstadoIDByName busca un estado por nombre y retorna su ID
func (r *EstadoRepository) FindEstadoIDByName(ctx context.Context, nombre string) (int, error) {
	logger.Info("🔍 Buscando estado por nombre", "nombre", nombre)

	var estado models.Estado

	err := r.dbManager.GetDB().WithContext(ctx).
		Model(&models.Estado{}).
		Where("nombre_estado = ?", nombre).
		First(&estado).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error("❌ Estado no encontrado", "nombre", nombre)
			return 0, nil
		}
		logger.Error("❌ Error buscando estado por nombre", "error", err)
		return 0, fmt.Errorf("error buscando estado por nombre: %v", err)
	}

	logger.Info("Estado encontrado", "ID", estado.ID, "nombre", estado.NombreEstado)
	return estado.ID, nil
}

// FindByExactName busca un estado por nombre exacto
func (r *EstadoRepository) FindByExactName(ctx context.Context, nombre string) (*models.Estado, error) {
	logger.Info("🔍 Buscando estado por nombre exacto", "nombre", nombre)

	var estado models.Estado

	err := r.dbManager.GetDB().WithContext(ctx).
		Where("nombre_estado = ?", nombre).
		First(&estado).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Debug("🔍 Estado no encontrado", "nombre", nombre)
			return nil, nil
		}
		logger.Error("❌ Error buscando estado por nombre exacto", "error", err)
		return nil, fmt.Errorf("error buscando estado por nombre exacto: %v", err)
	}

	logger.Debug("Estado encontrado", "nombre", nombre, "id", estado.ID)
	return &estado, nil
}

// FindOrCreateActive busca el estado "activo" y si no existe, lo crea
func (r *EstadoRepository) FindOrCreateActive(ctx context.Context) (int, error) {
	const estadoActivo = "activo"

	// Buscamos por nombre exacto
	estado, err := r.FindByExactName(ctx, estadoActivo)
	if err != nil {
		return 0, err
	}

	// Si existe, retornamos el ID
	if estado != nil {
		logger.Info("Estado activo encontrado", "id", estado.ID)
		return estado.ID, nil
	}

	// Si no existe, lo creamos
	logger.Info("➕ Creando estado activo ya que no existe")

	nuevoEstado := &models.Estado{
		NombreEstado: estadoActivo,
	}

	err = r.Create(ctx, nuevoEstado)
	if err != nil {
		logger.Error("❌ Error creando estado activo", "error", err)
		return 0, fmt.Errorf("error creando estado activo: %v", err)
	}

	logger.Info("Estado activo creado exitosamente", "id", nuevoEstado.ID)
	return nuevoEstado.ID, nil
}

// FindByID busca un estado por ID
func (r *EstadoRepository) FindByID(ctx context.Context, id int) (*models.Estado, error) {
	logger.Info("🔍 Buscando estado por ID", "ID", id)

	var estado models.Estado

	err := r.dbManager.GetDB().WithContext(ctx).
		First(&estado, id).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error("❌ Estado no encontrado", "ID", id)
			return nil, nil
		}
		logger.Error("❌ Error buscando estado por ID", "error", err)
		return nil, fmt.Errorf("error buscando estado por ID: %v", err)
	}

	logger.Info("Estado encontrado", "ID", estado.ID, "nombre", estado.NombreEstado)
	return &estado, nil
}

// FindAll busca todos los estados
func (r *EstadoRepository) FindAll(ctx context.Context) ([]models.Estado, error) {
	logger.Info("🔍 Buscando todos los estados")
	var estados []models.Estado // Usar el modelo existente

	err := r.dbManager.GetDB().WithContext(ctx).
		Find(&estados).Error

	if err != nil {
		logger.Error("❌ Error buscando todos los estados", "error", err)
		return nil, fmt.Errorf("error buscando todos los estados: %v", err)
	}

	logger.Info("Estados encontrados", "count", len(estados))
	return estados, nil
}

// Create crea un nuevo estado
func (r *EstadoRepository) Create(ctx context.Context, estado *models.Estado) error {
	logger.Info("🔍 Creando nuevo estado", "nombre", estado.NombreEstado)

	err := r.dbManager.GetDB().WithContext(ctx).
		Create(estado).Error

	if err != nil {
		logger.Error("❌ Error creando estado", "error", err)
		return fmt.Errorf("error creando estado: %v", err)
	}
	logger.Info("Estado creado", "ID", estado.ID, "nombre", estado.NombreEstado)
	return nil
}

// Update actualiza un estado existente
func (r *EstadoRepository) Update(ctx context.Context, estado *models.Estado) error {
	logger.Info("🔍 Actualizando estado", "ID", estado.ID)
	err := r.dbManager.GetDB().WithContext(ctx).
		Save(estado).Error

	if err != nil {
		logger.Error("❌ Error actualizando estado", "error", err)
		return fmt.Errorf("error actualizando estado: %v", err)
	}

	logger.Info("Estado actualizado", "ID", estado.ID)
	return nil
}

// Delete elimina un estado por ID
func (r *EstadoRepository) Delete(ctx context.Context, id int) error {
	logger.Info("🔍 Eliminando estado", "ID", id)

	result := r.dbManager.GetDB().WithContext(ctx).
		Delete(&models.Estado{}, id)

	if result.Error != nil {
		logger.Error("❌ Error eliminando estado", "error", result.Error)
		return fmt.Errorf("error eliminando estado: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		logger.Error("❌ Estado no encontrado para eliminar", "ID", id)
		return fmt.Errorf("estado no encontrado")
	}

	logger.Info("Estado eliminado", "ID", id)
	return nil
}
