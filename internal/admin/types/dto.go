package types

type DictionaryDTO struct {
	Tm string `json:"tm"`
	En string `json:"en"`
	Ru string `json:"ru"`
}

type TypeDTO struct {
	Name       DictionaryDTO `json:"name"`
	CategoryID int           `json:"category_id"`
}

type TypeOneDTO struct {
	Id        int        `json:"id"`
	Name      DictionaryDTO `json:"name"`
	Category  TypesDTO   `json:"category"`
	ImagePath string     `json:"image_path"`
}

type TypesDTO struct {
	Id        int           `json:"id"`
	Name      DictionaryDTO `json:"name"`
	ImagePath string        `json:"image_path"`
}

type TypeAllDTO struct {
	Types []TypesDTO `json:"types"`
	Count int        `json:"count"`
}