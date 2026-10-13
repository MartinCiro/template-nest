package repositories

import (
	"context"
	"fmt"

	"api_go/internal/core/auth"
	"api_go/pkg/logger"
)

type AuthAdapter struct {
	usuarioRepo *UsuarioRepository
	rolRepo     *RolRepository
	permisoRepo *PermisoRepository
}

func NewAuthAdapter(
	usuarioRepo *UsuarioRepository,
	rolRepo *RolRepository,
	permisoRepo *PermisoRepository,
) *AuthAdapter {
	return &AuthAdapter{
		usuarioRepo: usuarioRepo,
		rolRepo:     rolRepo,
		permisoRepo: permisoRepo,
	}
}

func (a *AuthAdapter) RetrieveUser(ctx context.Context, authData auth.AuthData) (*auth.User, error) {
	var searchField, searchValue string

	if authData.Email != "" {
		searchField = "email"
		searchValue = authData.Email
	} else if authData.Username != "" {
		searchField = "username"
		searchValue = authData.Username
	} else {
		return nil, fmt.Errorf("debe proporcionar email o username")
	}

	usuario, err := a.usuarioRepo.FindByEmailOrUsername(ctx, searchField, searchValue)
	if err != nil {
		return nil, fmt.Errorf("error recuperando usuario: %v", err)
	}

	if usuario == nil {
		fmt.Printf("❌ Usuario no encontrado: %s=%s\n", searchField, searchValue)
		return nil, nil
	}

	if usuario.IDRol != nil {
		permisos, err := a.permisoRepo.FindByRolID(ctx, *usuario.IDRol)
		if err != nil {
			logger.Error("⚠️ Error obteniendo permisos (continuando sin permisos)", "error", err)
			usuario.Permisos = []string{}
		} else {
			usuario.Permisos = permisos
		}
	}

	if usuario.IDRol != nil {
		rol, err := a.rolRepo.FindByID(ctx, *usuario.IDRol)
		if err != nil {
			logger.Warn("⚠️ Error obteniendo rol (continuando)", "error", err)
		} else if rol != nil {
			usuario.RolNombre = rol.NombreRol
			logger.Info("Rol del usuario", "rol", rol.NombreRol)
		}
	}

	return usuario, nil
}

func (a *AuthAdapter) RetrieveUserByID(ctx context.Context, userID int) (*auth.User, error) {
	usuario, err := a.usuarioRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if usuario == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}

	if usuario.IDRol != nil {
		permisos, err := a.permisoRepo.FindByRolID(ctx, *usuario.IDRol)
		if err != nil {
			fmt.Printf("⚠️ Error obteniendo permisos (continuando sin permisos): %v\n", err)
			usuario.Permisos = []string{}
		} else {
			usuario.Permisos = permisos
			fmt.Printf("Permisos asignados: %d permisos\n", len(permisos))
		}
	}

	if usuario.IDRol != nil {
		rol, err := a.rolRepo.FindByID(ctx, *usuario.IDRol)
		if err != nil {
			logger.Warn("⚠️ Error obteniendo rol (continuando)", "error", err)
		} else if rol != nil {
			usuario.RolNombre = rol.NombreRol
			logger.Info("Rol del usuario", "rol", rol.NombreRol)
		}
	}

	return usuario, nil
}

func (a *AuthAdapter) GetPermissionsByRoleID(ctx context.Context, roleID int) ([]string, error) {
	return a.permisoRepo.FindByRolID(ctx, roleID)
}
