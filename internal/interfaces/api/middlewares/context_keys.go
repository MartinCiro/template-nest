package middlewares

type contextKey string

const (
	UserKey   contextKey = "auth_user"
	UserIDKey contextKey = "auth_user_id"
)