package roles

import (
	"context"
	"fmt"
)

type RolService struct {
	rolPort RolesPort
}

func NewRolService(rolPort RolesPort) *RolService {
	return &RolService{
		rolPort: rolPort,
	}
}

func (s *RolService) CrearRol(ctx context.Context, rolData RolData) (*Rol, error) {
	// Solo delega al port - las validaciones ya se hicieron en el handler
	return s.rolPort.CrearRol(ctx, rolData)
}

func (s *RolService) ObtenerRoles(ctx context.Context) ([]Rol, error) {
	return s.rolPort.ObtenerRoles(ctx)
}

func (s *RolService) ObtenerRolXid(ctx context.Context, rolData RolDataXid) (*Rol, error) {
	// Solo validación básica de ID (opcional)
	if rolData.ID <= 0 {
		return nil, fmt.Errorf("status_cod:400, data:ID de rol inválido")
	}
	return s.rolPort.ObtenerRolXid(ctx, rolData)
}

func (s *RolService) ActualizarRol(ctx context.Context, rolData RolDataUpdate) (*Rol, error) {
	// Solo validación básica de ID (opcional)
	if rolData.ID <= 0 {
		return nil, fmt.Errorf("status_cod:400, data:ID de rol inválido")
	}
	return s.rolPort.ActualizarRol(ctx, rolData)
}

func (s *RolService) EliminarRol(ctx context.Context, rolData RolDataXid) error {
	// Solo validación básica de ID (opcional)
	if rolData.ID <= 0 {
		return fmt.Errorf("status_cod:400, data:ID de rol inválido")
	}
	return s.rolPort.EliminarRol(ctx, rolData)
}

func (s *RolService) ObtenerTodosLosPermisos(ctx context.Context) ([]string, error) {
	return s.rolPort.ObtenerTodosLosPermisos(ctx)
}
