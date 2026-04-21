package db

import (
	"context"
	"errors"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) utils.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) UserRoleById(ctx context.Context, userID int) (*string, error) {
	var role string

	query := `
		SELECT role
		FROM users
		WHERE id = $1
	`
	err := r.client.QueryRow(ctx, query, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(userID, "user")
		}
		return nil, err
	}

	return &role, nil
}
