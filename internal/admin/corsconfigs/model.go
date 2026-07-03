package corsconfigs

import "github.com/google/uuid"

type CORSConfig struct {
	ID               uuid.UUID
	RouteID          uuid.UUID
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}
