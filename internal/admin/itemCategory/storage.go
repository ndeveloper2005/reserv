package itemCategory

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, dto ItemCategoryCreateDTO, imagePath, baseURL string) (*ItemCategoryDTO, error)
	GetOne(ctx context.Context, itemCategoryId int, baseURL string) (*ItemCategoryDTO, error)
	GetAll(ctx context.Context, businessId, search, limit, offset, baseURL string) (*ItemCategoryAllDTO, error)
	Update(ctx context.Context, itemCategoryId int, itemCategory ItemCategoryNameDTO, imagePath, baseURL string) (*ItemCategoryDTO, error)
	Delete(ctx context.Context, itemCategoryId int) error
}
