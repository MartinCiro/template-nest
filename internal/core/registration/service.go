package registration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"api_go/internal/core/auth"
	"api_go/internal/infrastructure/cookies"
	"api_go/pkg/logger"
	"api_go/pkg/utils"
)

// RegisterRequest es el DTO de entrada para el registro
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	RolID    *int   `json:"rol_id,omitempty"`
}

// RegisterResult es el resultado del dominio (sin concerns HTTP)
type RegisterResult struct {
	Message      string
	SignedCookie string
	ExpiresAt    time.Time
}

// RegistrationService se encarga exclusivamente del registro de usuarios
type RegistrationService struct {
	authPort        auth.AuthPort 
	userRepo        auth.UserRepositoryPort
	rolRepo         auth.RolRepositoryPort
	estadoRepo      auth.EstadoRepositoryPort
	cookieSigner    *cookies.CookieSigner
	passwordService *utils.PasswordService
	jwtExpireTime   int
}

func NewRegistrationService(
	authPort auth.AuthPort,  
	userRepo auth.UserRepositoryPort,
	rolRepo auth.RolRepositoryPort,
	estadoRepo auth.EstadoRepositoryPort,
	cookieSigner *cookies.CookieSigner,
	passwordService *utils.PasswordService,
	jwtExpireTime int,
) *RegistrationService {
	return &RegistrationService{
		authPort:        authPort, 
		userRepo:        userRepo,
		rolRepo:         rolRepo,
		estadoRepo:      estadoRepo,
		cookieSigner:    cookieSigner,
		passwordService: passwordService,
		jwtExpireTime:   jwtExpireTime,
	}
}

// Register registra un nuevo usuario. Si currentUser es nil, es auto-registro (rol invitado).
// Si currentUser no es nil, debe tener permiso "usuario:crear".
func (s *RegistrationService) Register(
	ctx context.Context,
	req RegisterRequest,
	currentUser *auth.User,
) (*RegisterResult, error) {
	// 1. Validar entrada
	if err := s.validateRequest(req); err != nil {
		return nil, err
	}

	// 2. Determinar el rol
	rolNombre, err := s.resolveRoleName(ctx, req, currentUser)
	if err != nil {
		return nil, err
	}

	// 3. Resolver el ID del rol
	rolID, err := s.resolveRoleID(ctx, rolNombre)
	if err != nil {
		return nil, err
	}

	// 4. Resolver el estado (siempre "activo")
	estadoID, err := s.estadoRepo.FindOrCreateActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("error buscando/creando estado 'activo': %v", err)
	}

	// 5. Crear la entidad de usuario
	newUser, err := auth.NewUsuario(req.Username, req.Password, &rolID, &estadoID)
	if err != nil {
		return nil, err
	}

	// 6. Persistir en la base de datos
	userID, err := s.userRepo.CreateUser(ctx, newUser, req.Email)
	if err != nil {
		return nil, s.handleCreateError(err)
	}

	// 7. Generar cookie de sesión automática
	signedCookie, expiresAt, err := s.generateCookie(ctx, userID, req.Username, rolNombre, rolID)
	if err != nil {
		return nil, err
	}

	// 8. Retornar mensaje según el contexto
	message := "Usuario registrado con exito"
	if currentUser != nil {
		message = "Usuario creado correctamente"
	}

	return &RegisterResult{
		Message:      message,
		SignedCookie: signedCookie,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *RegistrationService) validateRequest(req RegisterRequest) error {
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return fmt.Errorf("Username, email y password son requeridos")
	}
	if len(req.Password) < 6 {
		return fmt.Errorf("El password debe tener al menos 6 caracteres")
	}
	return nil
}

func (s *RegistrationService) resolveRoleName(
	ctx context.Context,
	req RegisterRequest,
	currentUser *auth.User,
) (string, error) {
	if currentUser != nil {
		// Usuario autenticado creando otro usuario
		if !hasPermission(currentUser.Permisos, "usuario:crear") {
			return "", fmt.Errorf("No tiene permisos para crear usuarios")
		}
		if req.RolID != nil {
			rol, err := s.rolRepo.FindByID(ctx, *req.RolID)
			if err != nil {
				return "", fmt.Errorf("error buscando rol por ID: %v", err)
			}
			if rol == nil {
				return "", fmt.Errorf("rol con ID %d no encontrado", *req.RolID)
			}
			return rol.NombreRol, nil
		}
		return "usuario", nil
	}
	// Auto-registro → invitado
	return "invitado", nil
}

func (s *RegistrationService) resolveRoleID(ctx context.Context, rolNombre string) (int, error) {
	if rolNombre == "invitado" {
		return s.rolRepo.FindOrCreateGuest(ctx)
	}
	rol, err := s.rolRepo.FindByExactName(ctx, rolNombre)
	if err != nil {
		return 0, fmt.Errorf("error buscando rol '%s': %v", rolNombre, err)
	}
	if rol == nil {
		return 0, fmt.Errorf("rol '%s' no encontrado", rolNombre)
	}
	return rol.ID, nil
}

func (s *RegistrationService) generateCookie(
	ctx context.Context,
	userID int,
	username, role string,
	roleID int,
) (string, time.Time, error) {
	user, err := s.getUserByID(ctx, userID)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("error obteniendo usuario creado: %v", err)
	}

	expiresAt := time.Now().Add(time.Duration(s.jwtExpireTime) * time.Second)
	cookieData := &cookies.SignedCookieData{
		UserID:    user.ID,
		Username:  user.Username,
		Role:      role,
		RoleID:    roleID,
		ExpiresAt: expiresAt,
		IssuedAt:  time.Now(),
	}

	signedCookie, err := s.cookieSigner.Sign(cookieData)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("error generando cookie de autenticación: %v", err)
	}

	return signedCookie, expiresAt, nil
}

func (s *RegistrationService) getUserByID(ctx context.Context, userID int) (*auth.User, error) {
	return s.authPort.RetrieveUserByID(ctx, userID)
}

func (s *RegistrationService) handleCreateError(err error) error {
	if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
		return fmt.Errorf("El usuario ya existe, inicie sesión para continuar")
	}
	logger.Error("No se pudo crear el usuario", "error", err)
	return fmt.Errorf("Ha ocurrido un error en el servidor, contacte al administrador")
}

func hasPermission(permisos []string, permission string) bool {
	for _, perm := range permisos {
		if perm == permission {
			return true
		}
	}
	return false
}