package businesses

import "context"

type Repository interface {
	Create(ctx context.Context, business BusinessesReqDTO) (*int, error)
	AddImages(ctx context.Context, businessId int, mainImage string, additionalImages []string, baseURL string) (*BusinessesResDTO, error)
	GetOne(ctx context.Context, businessId int, baseURL string) (*BusinessesResDTO, error)
	GetAll(ctx context.Context, filter BusinessesFilter, baseURL string) (*[]BusinessesAllDTO, *int, error)
	Update(ctx context.Context, businessId int, business BusinessesReqDTO) error
	Delete(ctx context.Context, businessId int) error
	GetAndDeleteImage(ctx context.Context, businessId int, isMain bool) (*[]string, error)
	ConnectType(ctx context.Context, businessId int, cuisines ConnectType) (*[]ConnectTypes, error)
	Available(ctx context.Context, baseURL string) (*[]BusinessesAllDTO, error)
	AvailableAll(ctx context.Context, filter BusinessesFilter, baseURL string) (*AllAndSum, error)
}