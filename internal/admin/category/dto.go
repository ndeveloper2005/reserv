package category

type CategoryDTO struct {
	Id        int           `json:"id"`
	Name      DictionaryDTO `json:"name"`
	ImagePath string        `json:"image_path"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type CategoryAllDTO struct {
	Categories []CategoryDTO `json:"categories"`
	Count      int           `json:"count"`
}
