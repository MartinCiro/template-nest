// internal/infrastructure/repositories/usuario_repository.go
package repositories

import (
	"api_go/internal/core/auth"
	"api_go/internal/infrastructure/database"
	"api_go/internal/infrastructure/database/models"
	"api_go/pkg/logger"
	"context"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// Asegurar que UsuarioRepository implemente el port
var _ auth.UserRepositoryPort = (*UsuarioRepository)(nil)

type UsuarioRepository struct {
	dbManager *database.DBManager
}

func NewUsuarioRepository(dbManager *database.DBManager) *UsuarioRepository {
	return &UsuarioRepository{
		dbManager: dbManager,
	}
}

func (r *UsuarioRepository) FindByEmailOrUsername(ctx context.Context, field, value string) (*auth.User, error) {
	var usuario models.Usuario

	query := r.dbManager.GetDB().WithContext(ctx).Model(&models.Usuario{})

	if field == "email" {
		query = query.Where("email = ?", value)
	} else if field == "username" {
		query = query.Where("nom_user = ?", value)
	} else {
		return nil, fmt.Errorf("campo de búsqueda inválido: %s", field)
	}

	err := query.First(&usuario).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("error buscando usuario: %v", err)
	}

	return &auth.User{
		ID:           usuario.Documento,
		Username:     usuario.NomUser,
		Email:        usuario.Email,
		PasswordHash: usuario.Pass,
		IDRol:        &usuario.IDRol,
		IDEstado:     &usuario.EstadoID,
	}, nil
}

// FindByEmail busca usuario por email
func (r *UsuarioRepository) FindByEmail(ctx context.Context, email string) (*auth.User, error) {
	var usuario models.Usuario

	err := r.dbManager.GetDB().WithContext(ctx).
		Model(&models.Usuario{}).
		Where("email = ?", email).
		First(&usuario).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		logger.Error("❌ Error buscando usuario por email", "error", err, "email", email)
		return nil, fmt.Errorf("error buscando usuario: %v", err)
	}

	return &auth.User{
		ID:           usuario.Documento,
		Username:     usuario.NomUser,
		Email:        usuario.Email,
		PasswordHash: usuario.Pass,
		IDRol:        &usuario.IDRol,
		IDEstado:     &usuario.EstadoID,
	}, nil
}

// FindByID busca un usuario por documento (que es el ID)
func (r *UsuarioRepository) FindByID(ctx context.Context, userID int) (*auth.User, error) {
	var usuario models.Usuario
	documentoStr := strconv.FormatInt(int64(userID), 10)

	err := r.dbManager.GetDB().WithContext(ctx).
		Model(&models.Usuario{}).
		Where("documento = ?", documentoStr).
		First(&usuario).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("usuario no encontrado")
		}
		logger.Error("❌ Error buscando usuario por ID", "error", err, "user_id", userID)
		return nil, fmt.Errorf("error buscando usuario por ID: %v", err)
	}

	return &auth.User{
		ID:           usuario.Documento,
		Username:     usuario.NomUser,
		Email:        usuario.Email,
		PasswordHash: usuario.Pass,
		IDRol:        &usuario.IDRol,
		IDEstado:     &usuario.EstadoID,
	}, nil
}

// CreateUser crea un nuevo usuario
func (r *UsuarioRepository) CreateUser(ctx context.Context, usuario *auth.Usuario, email string) (int, error) {
	// Generar documento único
	documento := int(time.Now().UnixNano() / 1000000)
	documentoStr := strconv.Itoa(documento)

	// Usar mapa para evitar problemas de tipo
	userData := map[string]interface{}{
		"documento":      documentoStr,
		"nombres":        usuario.Username,
		"apellido":       "Usuario",
		"email":          email,
		"nom_user":       usuario.Username,
		"pass":           usuario.GetEncryptedPassword(),
		"id_rol":         *usuario.IDRol,
		"estado_id":      *usuario.IDEstado,
		"fecha_registro": time.Now(),
	}

	result := r.dbManager.GetDB().WithContext(ctx).
		Table("usuarios").
		Create(userData)

	if result.Error != nil {
		logger.Error("❌ Error creando usuario", "error", result.Error, "email", email)
		return 0, result.Error
	}

	return documento, nil
}

// UpdateUser actualiza un usuario existente
func (r *UsuarioRepository) UpdateUser(ctx context.Context, userID int, updates map[string]interface{}) error {
	result := r.dbManager.GetDB().WithContext(ctx).
		Model(&models.Usuario{}).
		Where("documento = ?", userID).
		Updates(updates)

	if result.Error != nil {
		logger.Error("❌ Error actualizando usuario", "error", result.Error, "user_id", userID)
		return fmt.Errorf("error actualizando usuario: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("usuario no encontrado")
	}

	return nil
}

// DeleteUser elimina un usuario por ID
func (r *UsuarioRepository) DeleteUser(ctx context.Context, userID int) error {
	result := r.dbManager.GetDB().WithContext(ctx).
		Where("documento = ?", userID).
		Delete(&models.Usuario{})

	if result.Error != nil {
		logger.Error("❌ Error eliminando usuario", "error", result.Error, "user_id", userID)
		return fmt.Errorf("error eliminando usuario: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("usuario no encontrado")
	}

	return nil
}

// FindAll busca todos los usuarios
func (r *UsuarioRepository) FindAll(ctx context.Context) ([]models.Usuario, error) {
	var usuarios []models.Usuario

	err := r.dbManager.GetDB().WithContext(ctx).
		Find(&usuarios).Error

	if err != nil {
		logger.Error("❌ Error buscando todos los usuarios", "error", err)
		return nil, fmt.Errorf("error buscando todos los usuarios: %v", err)
	}

	return usuarios, nil
}

// FindByRol busca usuarios por rol
func (r *UsuarioRepository) FindByRol(ctx context.Context, rolID int) ([]models.Usuario, error) {
	var usuarios []models.Usuario

	err := r.dbManager.GetDB().WithContext(ctx).
		Where("id_rol = ?", rolID).
		Find(&usuarios).Error

	if err != nil {
		logger.Error("❌ Error buscando usuarios por rol", "error", err, "rol_id", rolID)
		return nil, fmt.Errorf("error buscando usuarios por rol: %v", err)
	}

	return usuarios, nil
}
