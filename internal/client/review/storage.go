package review

import "context"

type Repository interface {
	Create(ctx context.Context, review ReviewReqDTO, clientId int) (*ReviewDTO, error)
	GetClient(ctx context.Context, reviewId int) (*ReviewDTO, error)
	GetAll(ctx context.Context, filter ReviewFilter) (*CountAndReviews, error)
	Delete(ctx context.Context, reviewId int) error
	GetAveragesByRestaurantID(ctx context.Context, restaurantId int, typeName string) (*RatingsAvg, error)
	FindByFilter(ctx context.Context, filter ReviewFilter) (*[]ReviewALLDTO, error)
}
