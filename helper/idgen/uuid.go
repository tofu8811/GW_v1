package idgen

import (
	"github.com/google/uuid"
)

func NewUUID() (uuid.UUID, error) {
	return uuid.NewV7()
}

func MustNewUUID() uuid.UUID {
	id, err := NewUUID()
	if err != nil {
		panic(err)
	}

	return id
}
