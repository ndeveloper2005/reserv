package businessesCoupon

import "context"

type Repository interface {
	Create(ctx context.Context, businessCoupon BusinessCouponReqDTO, businessId int) error
	GetCoupons(ctx context.Context, businessId int, limit, offset string) (*GetCouponsByBusiness, error)
	Update(ctx context.Context, businessCouponId int, coupon UpdateBusinessCouponReqDTO) error
	Delete(ctx context.Context, couponId int) error
}
