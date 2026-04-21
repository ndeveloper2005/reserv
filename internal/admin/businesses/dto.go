package businesses

import (
	"restaurants/internal/admin/item"

	"restaurants/internal/admin/subcategory"
	"restaurants/internal/client/review"
)

type BusinessesFilter struct {
	SubcategoryId int    `form:"subcategory_id"`
	ProvinceID    int    `form:"province_id"`
	TypesRaw      string `form:"types"`
	RatingRaw     string `form:"ratings"`
	Search        string `form:"search"`
	Limit         int    `form:"limit"`
	Offset        int    `form:"offset"`
	SortRaw       string `form:"sort"`
	Types         []int
	Rating        []int
	Sort          []string
}

type BusinessesReqDTO struct {
	Name           string         `json:"name"`
	ProvinceId     int            `json:"province_id"`
	Rating         float32        `json:"rating"`
	District       DictionaryDTO  `json:"district"`
	Phone          string         `json:"phone"`
	Description    DictionaryDTO  `json:"description"`
	SubcategoryIds []int          `json:"subcategory_ids"`
	DressCode      *DictionaryDTO `json:"dress_code"`
	OpensTime      string         `json:"opens_time"`
	ClosesTime     string         `json:"closes_time"`
	Expires        int            `json:"expires"`
}

type BusinessesResDTO struct {
	Id          int                            `json:"id"`
	Name        string                         `json:"name"`
	Address     DictionaryDTO                  `json:"address"`
	Rating      float32                        `json:"rating"`
	Images      []string                       `json:"images"`
	Phone       string                         `json:"phone"`
	Province    ConnectTypes                   `json:"province"`
	Types       []ConnectTypes                 `json:"types"`
	Description DictionaryDTO                  `json:"description"`
	DressCode   DictionaryDTO                  `json:"dress_code"`
	Subcategory []subcategory.SubcategoriesDTO `json:"subcategories"`
	Items       []item.ItemGetAllDTO           `json:"items"`
	RatingsAvg  review.RatingsAvg              `json:"ratings_avg"`
	Reviews     []review.ReviewALLDTO          `json:"reviews"`
	OpensTime   string                         `json:"opens_time"`
	ClosesTime  string                         `json:"closes_time"`
	Expires     int                            `json:"expires"`
}

type BusinessesAllDTO struct {
	Id          int                            `json:"id"`
	Name        string                         `json:"name"`
	Image       string                         `json:"image"`
	Address     DictionaryDTO                  `json:"address"`
	Rating      float32                        `json:"rating"`
	Types       []ConnectTypes                 `json:"types"`
	Subcategory []subcategory.SubcategoriesDTO `json:"subcategories"`
}

type BusinessForUpdateDTO struct {
	Name           string
	ProvinceId     int
	Rating         float64
	DistrictId     int
	Phone          string
	DescriptionId  int
	SubcategoryIds []int
	DressCodeId    *int
	OpensTime      string
	ClosesTime     string
	Expires        int
}

type ConnectTypes struct {
	Id   int           `json:"id"`
	Name DictionaryDTO `json:"name"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type AllAndSum struct {
	Businesses *[]BusinessesAllDTO `json:"businesses"`
	Count      int                 `json:"count"`
}

type ConnectType struct {
	TypeIds []int `json:"type_ids"`
}
