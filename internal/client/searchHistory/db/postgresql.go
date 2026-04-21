package db

import (
	"context"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/internal/client/searchHistory"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) searchHistory.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, search searchHistory.SearchHistoryReq, clientId int) (*[]searchHistory.SearchHistoryDTO, error) {
	var (
		count, searchHistoryId *int
		exists                 bool
	)

	q := `SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1) `

	err := r.client.QueryRow(ctx, q, clientId).Scan(&exists)
	if err != nil || !exists {
		fmt.Println("error: ", err)
		return nil, appresult.ErrNotFoundType(clientId, "clients")
	}
	q = `
         SELECT count(*), 
			(
		 		SELECT id 
				FROM search_histories 
				WHERE client_id = $1
				ORDER BY created_at ASC
				LIMIT 1
    		)
		FROM search_histories
		WHERE client_id = $1
    `

	err = r.client.QueryRow(ctx, q, clientId).Scan(&count, &searchHistoryId)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	}

	if *count > 4 {
		q = `
        DELETE FROM search_histories
        WHERE id = $1;
    `
		_, err = r.client.Exec(ctx, q, searchHistoryId)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
	}

	q = `
		INSERT INTO search_histories (client_id, search) 
			VALUES ($1, $2)
		`

	_, err = r.client.Exec(ctx, q, clientId, search.Search)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	resp, err := r.GetAll(ctx, clientId)
	return resp, nil
}

func (r *repository) GetAll(ctx context.Context, clientId int) (*[]searchHistory.SearchHistoryDTO, error) {
	var (
		searchHistories []searchHistory.SearchHistoryDTO
	)
	q := `
         SELECT id, search
		FROM search_histories
		WHERE client_id = $1
		ORDER BY created_at DESC
    `
	rows, err := r.client.Query(ctx, q, clientId)

	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()
	for rows.Next() {
		var search searchHistory.SearchHistoryDTO
		if err := rows.Scan(&search.Id, &search.Search); err != nil {
			fmt.Println("error:", err)
			return nil, appresult.ErrInternalServer
		}
		searchHistories = append(searchHistories, search)
	}

	return &searchHistories, nil
}

func (r *repository) Delete(ctx context.Context, searchHistoryId, clientId int) error {
	var (
		clientID int
	)

	q := `
        SELECT client_id
        FROM search_histories
        WHERE id = $1
    `
	err := r.client.QueryRow(ctx, q, searchHistoryId).Scan(&clientID)
	if err != nil {
		return appresult.ErrNotFoundType(searchHistoryId, "search_histories")
	}

	if clientID != clientId {
		return appresult.ErrInvalidCredentials
	}

	q = `
        DELETE FROM search_histories
        WHERE id = $1;
    `
	_, err = r.client.Exec(ctx, q, searchHistoryId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}
	return nil
}
