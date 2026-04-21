package types

import "context"

type Repository interface {
	Create(ctx context.Context, typ TypeDTO, imagePath string, baseURL string) (*TypeOneDTO, error)
	GetOne(ctx context.Context, typeID int, baseURL string) (*TypeOneDTO, error)
	GetAll(ctx context.Context, search string, page string, size string, categoryId string, baseURL string) (*TypeAllDTO, error)
	Update(ctx context.Context, typeID int, typ TypeDTO, imagePath string, baseURL string) (*TypeOneDTO, error)
	Delete(ctx context.Context, typeID int) error
}