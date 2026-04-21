package utils

import "context"

type Repository interface {
	UserRoleById(ctx context.Context, userId int) (*string, error)
}
