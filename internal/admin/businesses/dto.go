package businesses

import (
	"restaurants/internal/admin/item"
	"restaurants/internal/admin/subcategory"
)

type BusinessesFilter struct {
	SubcategoryIdsRaw   string `form:"subcategory_ids"`
	ProvinceID      	int    `form:"province_id"`
	Type          		int    `form:"type"`
	Search        		string `form:"search"`
	Limit         		int    `form:"limit"`
	Offset       	    int    `form:"offset"`
	SubcategoryIds 		[]int  
}

type IndexFilter struct {
	SubcategoryIdsRaw   string `form:"subcategory_ids"`
	CategoryIdsRaw      string `form:"category_ids"`
	SubcategoryIds 		[]int  
	CategoryIds 		[]int  
}

type BusinessesReqDTO struct {
	Name           string         `json:"name"`
	ProvinceId     int            `json:"province_id"`
	District       DictionaryDTO  `json:"district"`
	Phone          string         `json:"phone"`
	Description    DictionaryDTO  `json:"description"`
	SubcategoryIds []int          `json:"subcategory_ids"`
	OpensTime      string         `json:"opens_time"`
	ClosesTime     string         `json:"closes_time"`
	Expires        int            `json:"expires"`
}

type BusinessesResDTO struct {
	Id          int                            `json:"id"`
	Name        string                         `json:"name"`
	Address     DictionaryDTO                  `json:"address"`
	Images      []string                       `json:"images"`
	Phone       string                         `json:"phone"`
	Province    ConnectTypes                   `json:"province"`
	Types       []ConnectTypes                 `json:"types"`
	Description DictionaryDTO                  `json:"description"`
	Subcategory []subcategory.SubcategoriesDTO `json:"subcategories"`
	Items       []item.ItemGetAllDTO           `json:"items"`
	OpensTime   string                         `json:"opens_time"`
	ClosesTime  string                         `json:"closes_time"`
	Expires     int                            `json:"expires"`
}

type BusinessesAllDTO struct {
	Id          int                            `json:"id"`
	Name        string                         `json:"name"`
	Image       string                         `json:"image"`
	Address     DictionaryDTO                  `json:"address"`
	Types       []ConnectTypes                 `json:"types"`
	Subcategory []subcategory.SubcategoriesDTO `json:"subcategories"`
}

type BusinessForUpdateDTO struct {
	Name           string
	ProvinceId     int
	DistrictId     int
	Phone          string
	DescriptionId  int
	SubcategoryIds []int
	OpensTime      string
	ClosesTime     string
	Expires        int
}

type Index struct {
	NewCreated []IndexBusinesses `json:"new_created"`
	WithDiscounts []IndexBusinesses `json:"with_discounts"`
}

type IndexBusinesses struct {
	Id   int             `json:"id"`
	Name string          `json:"name"`
	Image string         `json:"image"`
	DiscountPercent *int `json:"discount_percent"`
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

type UpdateStatus struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}
