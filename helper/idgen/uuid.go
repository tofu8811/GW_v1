package idgen

import (
	"github.com/gofrs/uuid/v5"
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