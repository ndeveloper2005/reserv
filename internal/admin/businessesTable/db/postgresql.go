package db

import (
	"context"
	"fmt"
	"restaurants/internal/admin/businessesTable"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strconv"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) businessesTable.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, businessId int, dto businessesTable.BusinessTableReqDTO) error {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error begin tx:", err)
		return appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var exists bool
	q := `
		SELECT EXISTS(SELECT 1 FROM businesses WHERE id = $1)
	`
	err = tx.QueryRow(ctx, q, businessId).Scan(&exists)
	if err != nil {
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(businessId, "business")
	}

	qInsert := `
		INSERT INTO businesses_tables (businesses_id, seats, table_count)
		VALUES ($1, $2, $3)
	`
	_, err = tx.Exec(ctx, qInsert, businessId, dto.Seat, dto.TableCount)
	if err != nil {
		fmt.Println("error:", err)
		return appresult.ErrInternalServer
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error commit transaction:", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) GetTables(ctx context.Context, businessId int, limit, offset string) (*businessesTable.GetTableByBusiness, error) {
	var (
		tables []businessesTable.BusinessTableDTO
		count  int
	)

	offsetInt, err := strconv.Atoi(offset)
	if err != nil || offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}
	offsetInt = (offsetInt - 1) * limitInt

	q := `
		SELECT id, seats, table_count
		FROM businesses_tables
		WHERE businesses_id = $1
		ORDER BY id
		LIMIT $2 OFFSET $3;
	`
	rows, err := r.client.Query(ctx, q, businessId, limitInt, offsetInt)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var t businessesTable.BusinessTableDTO
		err := rows.Scan(&t.Id, &t.Seat, &t.TableCount)
		if err != nil {
			fmt.Println("error scan:", err)
			return nil, appresult.ErrInternalServer
		}
		tables = append(tables, t)
	}

	q = `
		SELECT count(*)
		FROM businesses_tables
		WHERE businesses_id = $1;
	`
	err = r.client.QueryRow(ctx, q, businessId).Scan(&count)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	result := businessesTable.GetTableByBusiness{
		Count:          count,
		BusinessTables: tables,
	}
	return &result, nil
}

func (r *repository) Update(ctx context.Context, businessTableId int, dto businessesTable.BusinessTableReqDTO) error {
	var (
		businessId int
	)
	q := `
		UPDATE businesses_tables
		SET seats = $1, table_count = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
		RETURNING businesses_id
	`

	err := r.client.QueryRow(ctx, q, dto.Seat, dto.TableCount, businessTableId).
		Scan(&businessId)

	if err != nil {
		return appresult.ErrNotFoundType(businessTableId, "businesses table")
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, businessTableId int) error {
	q := `
		DELETE FROM businesses_tables
		WHERE id = $1
	`
	result, err := r.client.Exec(ctx, q, businessTableId)
	if err != nil {
		return appresult.ErrInternalServer
	}
	if result.RowsAffected() == 0 {
		return appresult.ErrNotFoundType(businessTableId, "businesses table")
	}
	return nil
}
