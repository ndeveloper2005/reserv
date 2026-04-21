package businessesCoupon

type BusinessCouponReqDTO struct {
	BusinessCoupon DictionaryDTO `json:"businesses_coupon"`
	Life           int           `json:"life,omitempty"`
}

type UpdateBusinessCouponReqDTO struct {
	BusinessCoupon UpdateDictionaryDTO `json:"businesses_coupon,omitempty"`
	Life           int                 `json:"life,omitempty"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type UpdateDictionaryDTO struct {
	Tm string `json:"tm,omitempty"`
	Ru string `json:"ru,omitempty"`
	En string `json:"en,omitempty"`
}

type CouponsDTO struct {
	Id     int           `json:"id"`
	Coupon DictionaryDTO `json:"coupon"`
	Life   int           `json:"life"`
}

type GetCouponsByBusiness struct {
	Count   int          `json:"count"`
	Coupons []CouponsDTO `json:"coupons"`
}
