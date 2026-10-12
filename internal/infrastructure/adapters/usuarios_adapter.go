package adapters

import (
	"api_go/internal/core/usuarios"
	"api_go/internal/infrastructure/database/models"
	"api_go/internal/infrastructure/redis"
	"api_go/pkg/logger"
	"api_go/pkg/utils"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UsuariosAdapter struct {
	db           *gorm.DB
	redisService *redis.Cache
}

func NewUsuariosAdapter(db *gorm.DB, redisService *redis.Cache) *UsuariosAdapter {
	return &UsuariosAdapter{
		db:           db,
		redisService: redisService,
	}
}

func (a *UsuariosAdapter) toUsuarioEntityConRelaciones(usuarioDB *models.Usuario) *usuarios.Usuario {
	rolNombre := a.obtenerNombreRolDesdeRelacion(usuarioDB)
	estadoNombre := a.obtenerNombreEstadoDesdeRelacion(usuarioDB)
	return &usuarios.Usuario{
		Documento:          usuarioDB.Documento,
		Nombres:            usuarioDB.Nombres,
		Apellido:           usuarioDB.Apellido,
		NombreCompleto:     usuarioDB.Nombres + " " + usuarioDB.Apellido,
		Email:              usuarioDB.Email,
		Username:           usuarioDB.NomUser,
		RolID:              usuarioDB.IDRol,
		EstadoID:           usuarioDB.EstadoID,
		RolNombre:          rolNombre,
		EstadoNombre:       estadoNombre,
		FechaRegistro:      usuarioDB.FechaRegistro,
		FechaActualizacion: time.Now(),
	}
}

func (a *UsuariosAdapter) obtenerNombreRolDesdeRelacion(usuarioDB *models.Usuario) string {
	return usuarioDB.Rol.NombreRol
}

func (a *UsuariosAdapter) obtenerNombreEstadoDesdeRelacion(usuarioDB *models.Usuario) string {
	return usuarioDB.Estado.NombreEstado
}

func (a *UsuariosAdapter) ObtenerUsuarios(ctx context.Context) ([]usuarios.Usuario, error) {
	cacheKey := "usuarios:leer"
	cachedUsuarios, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedUsuarios != "" {
		var usuariosCache []usuarios.Usuario
		if err := json.Unmarshal([]byte(cachedUsuarios), &usuariosCache); err == nil {
			return usuariosCache, nil
		}
	}

	var usuariosDB []models.Usuario
	err = a.db.WithContext(ctx).
		Preload("Rol").
		Preload("Estado").
		Find(&usuariosDB).Error
	if err != nil {
		return nil, a.handleQueryError(err, "consultando usuarios")
	}

	if len(usuariosDB) == 0 {
		return []usuarios.Usuario{}, nil
	}

	usuariosList := make([]usuarios.Usuario, len(usuariosDB))
	for i, usuarioDB := range usuariosDB {
		usuariosList[i] = *a.toUsuarioEntityConRelaciones(&usuarioDB)
	}

	usuariosJSON, err := json.Marshal(usuariosList)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(usuariosJSON), 1800)
	}

	return usuariosList, nil
}

func (a *UsuariosAdapter) ObtenerUsuarioXid(ctx context.Context, usuarioData usuarios.UsuarioDataXid) (*usuarios.Usuario, error) {
	documento := fmt.Sprintf("%s", usuarioData.Documento)
	logger.Warn("Consultando usuario con documento", "documento", usuarioData.Documento)
	cacheKey := fmt.Sprintf("usuars:%s", usuarioData.Documento)

	cachedUsuario, err := a.redisService.Get(ctx, cacheKey)
	if err == nil && cachedUsuario != "" {
		var usuarioCache usuarios.Usuario
		if err := json.Unmarshal([]byte(cachedUsuario), &usuarioCache); err == nil {
			return &usuarioCache, nil
		}
	}

	var usuarioDB models.Usuario
	err = a.db.WithContext(ctx).
		Preload("Rol").
		Preload("Estado").
		Where("documento = ?", documento).
		Limit(1).
		Find(&usuarioDB).Error
	if err != nil {
		return nil, a.handleQueryError(err, "consultando usuario")
	}

	if usuarioDB.Documento == "" {
		return nil, nil
	}

	usuarioEntity := a.toUsuarioEntityConRelaciones(&usuarioDB)

	usuarioJSON, err := json.Marshal(usuarioEntity)
	if err == nil {
		a.redisService.Set(ctx, cacheKey, string(usuarioJSON), 1800)
	}

	return usuarioEntity, nil
}

func (a *UsuariosAdapter) ActualizarUsuario(ctx context.Context, usuarioData usuarios.UsuarioDataUpdate) (*usuarios.Usuario, error) {
	var usuarioExistente models.Usuario
	err := a.db.WithContext(ctx).Where("documento = ?", usuarioData.Documento).First(&usuarioExistente).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("el usuario solicitado no existe")
		}
		return nil, a.handleQueryError(err, "verificando usuario existente")
	}

	updates := make(map[string]interface{})

	if usuarioData.Username != nil {
		updates["NomUser"] = *usuarioData.Username
	}
	if usuarioData.Email != nil {
		updates["email"] = *usuarioData.Email
	}
	if usuarioData.Nombres != nil {
		updates["nombres"] = *usuarioData.Nombres
	}
	if usuarioData.Apellido != nil {
		updates["apellido"] = *usuarioData.Apellido
	}
	if usuarioData.InfoPerfil != nil {
		updates["info_perfil"] = *usuarioData.InfoPerfil
	}
	if usuarioData.NumContacto != nil {
		updates["num_contacto"] = *usuarioData.NumContacto
	}
	if usuarioData.FechaNacimiento != nil {
		updates["fecha_nacimiento"] = *usuarioData.FechaNacimiento
	}
	if usuarioData.RolID != nil {
		updates["IDRol"] = *usuarioData.RolID
	}
	if usuarioData.EstadoID != nil {
		updates["EstadoID"] = *usuarioData.EstadoID
	}

	if len(updates) == 0 {
		return a.toUsuarioEntityConRelaciones(&usuarioExistente), nil
	}

	err = a.db.WithContext(ctx).Model(&models.Usuario{}).
		Where("documento = ?", usuarioData.Documento).
		Updates(updates).Error
	if err != nil {
		return nil, a.handleUpdateError(err, usuarioData.Username, usuarioData.Email)
	}

	var usuarioActualizado models.Usuario
	err = a.db.WithContext(ctx).
		Preload("Rol").
		Preload("Estado").
		Where("documento = ?", usuarioData.Documento).
		First(&usuarioActualizado).Error
	if err != nil {
		return nil, a.handleQueryError(err, "obteniendo usuario actualizado")
	}

	a.actualizarCacheUsuario(ctx, usuarioData.Documento, &usuarioActualizado)

	return a.toUsuarioEntityConRelaciones(&usuarioActualizado), nil
}

func (a *UsuariosAdapter) EliminarUsuario(ctx context.Context, usuarioData usuarios.UsuarioDataXid) error {
	result := a.db.WithContext(ctx).Where("documento = ?", usuarioData.Documento).Delete(&models.Usuario{})
	if result.Error != nil {
		return a.handleDeleteError(result.Error, usuarioData.Documento)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("el usuario con ID %s no existe", usuarioData.Documento)
	}

	a.redisService.Delete(ctx, fmt.Sprintf("usuario:%s", usuarioData.Documento))

	return nil
}

func (a *UsuariosAdapter) CambiarPassword(ctx context.Context, cambiarPasswordData usuarios.CambiarPasswordData) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cambiarPasswordData.NuevoPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error procesando nueva contraseña")
	}

	err = a.db.WithContext(ctx).Model(&models.Usuario{}).
		Where("documento = ?", cambiarPasswordData.ID).
		Updates(map[string]interface{}{
			"password": string(hashedPassword),
		}).Error
	if err != nil {
		return a.handleUpdateError(err, nil, nil)
	}

	a.redisService.Delete(ctx, fmt.Sprintf("usuario:%d", cambiarPasswordData.ID))

	return nil
}

func (a *UsuariosAdapter) ObtenerUsuarioPorUsername(ctx context.Context, username string) (*usuarios.Usuario, error) {
	var usuarioDB models.Usuario
	err := a.db.WithContext(ctx).
		Where("username = ?", username).
		First(&usuarioDB).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, a.handleQueryError(err, "consultando usuario por username")
	}

	return a.toUsuarioEntityConRelaciones(&usuarioDB), nil
}

func (a *UsuariosAdapter) ObtenerUsuarioPorEmail(ctx context.Context, email string) (*usuarios.Usuario, error) {
	var usuarioDB models.Usuario
	err := a.db.WithContext(ctx).
		Where("email = ?", email).
		First(&usuarioDB).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, a.handleQueryError(err, "consultando usuario por email")
	}

	return a.toUsuarioEntityConRelaciones(&usuarioDB), nil
}

func (a *UsuariosAdapter) ObtenerUsuariosConRelaciones(ctx context.Context) ([]usuarios.UsuarioConRelaciones, error) {
	var usuariosDB []models.Usuario
	err := a.db.WithContext(ctx).
		Preload("Rol").
		Preload("Estado").
		Find(&usuariosDB).Error
	if err != nil {
		return nil, a.handleQueryError(err, "consultando usuarios con relaciones")
	}

	usuariosList := make([]usuarios.UsuarioConRelaciones, len(usuariosDB))
	for i, usuarioDB := range usuariosDB {
		usuariosList[i] = *a.toUsuarioConRelacionesEntity(&usuarioDB)
	}

	return usuariosList, nil
}

func (a *UsuariosAdapter) ObtenerUsuarioConRelaciones(ctx context.Context, usuarioData usuarios.UsuarioDataXid) (*usuarios.UsuarioConRelaciones, error) {
	var usuarioDB models.Usuario
	err := a.db.WithContext(ctx).
		Preload("Rol").
		Preload("Estado").
		Where("documento = ?", usuarioData.Documento).
		First(&usuarioDB).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, a.handleQueryError(err, "consultando usuario con relaciones")
	}

	return a.toUsuarioConRelacionesEntity(&usuarioDB), nil
}

func (a *UsuariosAdapter) toUsuarioConRelacionesEntity(usuarioDB *models.Usuario) *usuarios.UsuarioConRelaciones {
	rolNombre := ""
	if usuarioDB.IDRol != 0 {
		rolNombre = usuarioDB.Rol.NombreRol
	}

	estadoNombre := ""
	if usuarioDB.Estado.ID != 0 {
		estadoNombre = usuarioDB.Estado.NombreEstado
	}

	return &usuarios.UsuarioConRelaciones{
		Username:  usuarioDB.NomUser,
		Email:     usuarioDB.Email,
		RolID:     usuarioDB.IDRol,
		EstadoID:  usuarioDB.EstadoID,
		RolNombre: rolNombre,
		Estado:    estadoNombre,
	}
}

func (a *UsuariosAdapter) actualizarCacheUsuario(ctx context.Context, userID string, usuarioDB *models.Usuario) {
	usuarioEntity := a.toUsuarioEntityConRelaciones(usuarioDB)
	usuarioJSON, err := json.Marshal(usuarioEntity)
	if err == nil {
		a.redisService.Set(ctx, fmt.Sprintf("usuario:%s", userID), string(usuarioJSON), 1800)
	}

	a.actualizarCacheLista(ctx, userID, usuarioEntity)
}

func (a *UsuariosAdapter) actualizarCacheLista(ctx context.Context, userID string, usuarioEntity *usuarios.Usuario) {
	cachedUsuarios, err := a.redisService.Get(ctx, "usuarios:leer")
	if err == nil && cachedUsuarios != "" {
		var usuariosCache []usuarios.Usuario
		if err := json.Unmarshal([]byte(cachedUsuarios), &usuariosCache); err == nil {
			for i, u := range usuariosCache {
				if u.Documento == userID {
					usuariosCache[i] = *usuarioEntity
					break
				}
			}

			updatedJSON, _ := json.Marshal(usuariosCache)
			a.redisService.Set(ctx, "usuarios:leer", string(updatedJSON), 1800)
		}
	}
}

func (a *UsuariosAdapter) handleQueryError(err error, operation string) error {
	return fmt.Errorf("Ocurrió un error %s", operation)
}

func (a *UsuariosAdapter) handleDeleteError(err error, id string) error {
	errStr := err.Error()
	logger.Error("Error eliminando usuario", "id", id)
	if strings.Contains(errStr, "foreign") || strings.Contains(errStr, "constraint") {
		return fmt.Errorf("No se puede eliminar el usuario porque tiene registros asociados")
	}
	return fmt.Errorf("Ocurrió un error eliminando el usuario")
}

func (a *UsuariosAdapter) handleUpdateError(err error, username, email *string) error {
	errStr := err.Error()
	if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "Duplicate") {
		if username != nil && strings.Contains(errStr, "username") {
			validacion := utils.ValidarExistente("P2002", *username)
			if !validacion.OK {
				return fmt.Errorf("status_cod:409, data:%s", validacion.Data)
			}
		}
		if email != nil && strings.Contains(errStr, "email") {
			validacion := utils.ValidarExistente("P2002", *email)
			if !validacion.OK {
				return fmt.Errorf("status_cod:409, data:%s", validacion.Data)
			}
		}
	}
	return fmt.Errorf("Ocurrió un error actualizando el usuario")
}
