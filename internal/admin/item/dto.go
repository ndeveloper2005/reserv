package item

type ItemReqDTO struct {
	Name            DictionaryDTO   `json:"name" binding:"required"`
	Ingredient      []DictionaryDTO `json:"ingredients" binding:"required"`
	Value           float32         `json:"value" binding:"required"`
	BusinessId      int             `json:"businesses_id" binding:"required"`
	TypeId          int             `json:"type_id" binding:"required"`
	ItemCategoryIds []int           `json:"item_category_ids" binding:"required"`
	IsChosen        interface{}     `json:"is_chosen"`
}

type ItemGetOneDTO struct {
	Id             int           `json:"id"`
	Name           DictionaryDTO `json:"name"`
	Ingredient     DictionaryDTO `json:"ingredients"`
	Value          float32       `json:"value"`
	ItemCategories DictionaryDTO `json:"item_categories"`
	ImagePath      string        `json:"image_path"`
}

type ItemGetAllDTO struct {
	Id         int           `json:"id"`
	Name       DictionaryDTO `json:"name"`
	Ingredient DictionaryDTO `json:"ingredients"`
	Value      float32       `json:"value"`
	ImagePath  string        `json:"image_path"`
	IsChosen   bool          `json:"is_chosen"`
}

type GetAllWithCount struct {
	Count int             `json:"count"`
	Items []ItemGetAllDTO `json:"items"`
}

type ItemForUpdateDTO struct {
	NameId          int
	IngredientId    int
	Value           float32
	BusinessId      int
	TypeId          int
	ItemCategoryIds []int
	ImagePath       string
	IsChosen        bool
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type ItemResForUpdateDTO struct {
	Id             int             `json:"id"`
	Name           DictionaryDTO   `json:"name"`
	Ingredient     []DictionaryDTO `json:"ingredients"`
	ImagePath      string          `json:"image_path"`
	Value          float32         `json:"value"`
	Type           Name            `json:"type"`
	ItemCategories []Name          `json:"item_categories"`
	IsChosen       bool            `json:"is_chosen"`
}

type Name struct {
	Id   int           `json:"id"`
	Name DictionaryDTO `json:"name"`
}
