package review

import "restaurants/internal/client/clients"

type ReviewFilter struct {
	ClientId int    `form:"client_id"`
	TypeId   int    `form:"type_id" binding:"required"`
	TypeName string `form:"type_name" binding:"required"`
	Category string `form:"category"` // recommended, average, badly
	Offset   int    `form:"offset"`
	Limit    int    `form:"limit"`
}

type ReviewDTO struct {
	Id        int            `json:"id"`
	Client    clients.Client `json:"client"`
	TypeId    int            `json:"type_id"`
	TypeName  string         `json:"type_name"`
	OverAll   int            `json:"over_all"`
	Food      int            `json:"food"`
	Service   int            `json:"service"`
	Ambience  int            `json:"ambience"`
	Value     int            `json:"value"`
	Comment   string         `json:"comment"`
	CreatedAt string         `json:"created_at"`
}

type ReviewReqDTO struct {
	TypeId   int    `json:"type_id" binding:"required"`
	TypeName string `json:"type_name" binding:"required,oneof=restaurant food"`
	OverAll  int    `json:"over_all" binding:"required,gte=1,lte=5"`
	Food     int    `json:"food" binding:"required,gte=1,lte=5"`
	Service  int    `json:"service" binding:"required,gte=1,lte=5"`
	Ambience int    `json:"ambience" binding:"required,gte=1,lte=5"`
	Value    int    `json:"value" binding:"required,gte=1,lte=5"`
	Comment  string `json:"comment"`
}

type ReviewALLDTO struct {
	Id        int            `json:"id"`
	Client    clients.Client `json:"client"`
	OverAll   int            `json:"over_all"`
	Comment   string         `json:"comment"`
	CreatedAt string         `json:"created_at"`
}

type CountAndReviews struct {
	Count   int            `json:"count"`
	Reviews []ReviewALLDTO `json:"reviews"`
}

type RatingsAvg struct {
	OverAll  float64 `json:"over_all"`
	Food     float64 `json:"food"`
	Service  float64 `json:"service"`
	Ambience float64 `json:"ambience"`
	Value    float64 `json:"value"`
}
