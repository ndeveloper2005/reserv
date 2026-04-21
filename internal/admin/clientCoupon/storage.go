package clientCoupon

import "context"

type Repository interface {
	Create(ctx context.Context, businessId int, clientCoupon AssignCouponReqDTO) (*GetCouponsByClient, error)
	GetCouponsForClient(ctx context.Context, clientId int, limit, offset string, baseURL string) (*GetBusinessesCoupons, error)
	GetCouponsForBusiness(ctx context.Context, businessId int, limit, offset string) (*GetCouponsByBusiness, error)
	Delete(ctx context.Context, assignId int) error
}
