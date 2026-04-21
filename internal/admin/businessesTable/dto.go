package businessesTable

type BusinessTableReqDTO struct {
	Seat       int `json:"seat"`
	TableCount int `json:"table_count"`
}

type BusinessTableDTO struct {
	Id         int `json:"id"`
	Seat       int `json:"seat"`
	TableCount int `json:"table_count"`
}

type GetTableByBusiness struct {
	Count          int                `json:"count"`
	BusinessTables []BusinessTableDTO `json:"business_tables"`
}