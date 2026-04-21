package searchHistory

type SearchHistoryReq struct {
	Search string `json:"search"`
}

type SearchHistoryDTO struct {
	Id     int    `json:"id"`
	Search string `json:"search"`
}
