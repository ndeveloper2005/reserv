package businessesTable

import "context"

type Repository interface {
	Create(ctx context.Context, businessId int, businessTable BusinessTableReqDTO) error
	GetTables(ctx context.Context, businessId int, limit, offset string) (*GetTableByBusiness, error)
	Update(ctx context.Context, businessTableId int, tables BusinessTableReqDTO) error
	Delete(ctx context.Context, businessTableId int) error
}