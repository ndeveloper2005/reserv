package searchHistory

import "context"

type Repository interface {
	Create(ctx context.Context, search SearchHistoryReq, clientId int) (*[]SearchHistoryDTO, error)
	GetAll(ctx context.Context, clientId int) (*[]SearchHistoryDTO, error)
	Delete(ctx context.Context, searchHistory, clientId int) error
}
