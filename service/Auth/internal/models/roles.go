package models

import (
	"errors"
	"fmt"
)

const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

var (
	ErrInvalidRole = errors.New("invalid role")
	ErrEmptyRole   = errors.New("role cannot be empty")
)


var ValidRoles = []string{RoleUser, RoleAdmin}

// ValidateRole checks if the provided role is valid
func ValidateRole(role string) error {
	if role == "" {
		return ErrEmptyRole
	}

	for _, validRole := range ValidRoles {
		if role == validRole {
			return nil
		}
	}

	return fmt.Errorf("%w: %s (must be one of: %v)", ErrInvalidRole, role, ValidRoles)
}


func IsValidRole(role string) bool {
	return ValidateRole(role) == nil
}


func IsAdmin(role string) bool {
	return role == RoleAdmin
}

// IsUser returns true if role is user
func IsUser(role string) bool {
	return role == RoleUser
}

// DefaultRole returns the default role for new users
func DefaultRole() string {
	return RoleUser
}
