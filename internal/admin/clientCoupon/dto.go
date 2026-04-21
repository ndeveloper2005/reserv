package clientCoupon

import "restaurants/internal/client/clients"

type AssignCouponReqDTO struct {
	ClientId          int   `json:"client_id"`
	BusinessCouponIds []int `json:"businesses_coupon_ids"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type GetCouponsByClient struct {
	Business Business `json:"businesses"`
	Coupons  []Coupon `json:"coupons"`
}

type GetBusinessesCoupons struct {
	Count           int                  `json:"count"`
	BusinessCoupons []GetCouponsByClient `json:"businesses_coupons"`
}

type GetCouponsForBusiness struct {
	Client  clients.Profile `json:"client"`
	Coupons []Coupon        `json:"coupons"`
}

type GetCouponsByBusiness struct {
	ClientCount   int                     `json:"client_count"`
	ClientCoupons []GetCouponsForBusiness `json:"client_coupons"`
}

type Business struct {
	Id          int           `json:"id"`
	Images      string        `json:"images"`
	Name        string        `json:"name"`
	Rating      float32       `json:"rating"`
	Price       int           `json:"price"`
	Address     DictionaryDTO `json:"address"`
	CountCoupon int           `json:"count_coupon"`
}

type Coupon struct {
	Id        int           `json:"id"`
	Coupon    DictionaryDTO `json:"name"`
	ExpireDay int           `json:"expire_day"`
	IsUsed    bool          `json:"is_used"`
}
