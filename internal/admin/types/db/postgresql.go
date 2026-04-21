package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/admin/types"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v4"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) types.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, dto types.TypeDTO, imagePath string, baseURL string) (*types.TypeOneDTO, error) {

	var (
		id     int
		typeId int
	)

	q := `
		SELECT d.id
		FROM types t
		JOIN dictionary d ON t.name_dictionary_id = d.id
		WHERE ( d.tm = $1 OR d.en = $2 OR d.ru = $3 ) AND t.category_id = $4
	`

	err := r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru, dto.CategoryID).Scan(&id)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {

			q = `
				INSERT INTO dictionary (tm, en, ru)
				VALUES ($1,$2,$3)
				RETURNING id
			`

			err = r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&id)

			if err != nil {
				fmt.Println("error: ", err)
				return nil, err
			}

			q = `
				INSERT INTO types (name_dictionary_id, category_id, image_path)
				VALUES ($1, $2, $3)
				RETURNING id
			`

			err = r.client.QueryRow(ctx, q, id, dto.CategoryID, imagePath).Scan(&typeId)

			if err != nil {
				fmt.Println("error: ", err)
				return nil, appresult.ErrInternalServer
			}

			return r.GetOne(ctx, typeId, baseURL)

		}

		fmt.Println("error: ", err)
		return nil, err
	}

	return nil, appresult.ErrAlreadyData("type")
}

func (r *repository) GetOne(ctx context.Context, typeId int, baseURL string) (*types.TypeOneDTO, error) {

	var resp types.TypeOneDTO

	q := `
		SELECT 
			t.id, ds.tm, ds.en, ds.ru, t.image_path,
			c.id, dc.tm, dc.en, dc.ru, c.image_path
		FROM types t
		JOIN categories c ON t.category_id = c.id
		JOIN dictionary ds ON t.name_dictionary_id = ds.id
		JOIN dictionary dc ON c.name_dictionary_id = dc.id
		WHERE t.id = $1
	`

	err := r.client.QueryRow(ctx, q, typeId).Scan(
		&resp.Id,
		&resp.Name.Tm,
		&resp.Name.En,
		&resp.Name.Ru,
		&resp.ImagePath,
		&resp.Category.Id,
		&resp.Category.Name.Tm,
		&resp.Category.Name.En,
		&resp.Category.Name.Ru,
		&resp.Category.ImagePath,
	)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(typeId, "type")
		}

		return nil, err
	}

	if baseURL != "" {

		cleanPath := strings.ReplaceAll(resp.ImagePath, "\\", "/")
		resp.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)

		cleanPathC := strings.ReplaceAll(resp.Category.ImagePath, "\\", "/")
		resp.Category.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPathC)

	}

	return &resp, nil
}

func (r *repository) GetAll(ctx context.Context, search string, limit string, offset string, categoryID string, baseURL string) (*types.TypeAllDTO, error) {

	var (
		resp  types.TypeAllDTO
		count int
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

	categoryFilter := ""

	if categoryID != "" {
		categoryFilter = fmt.Sprintf("AND t.category_id = %s", categoryID)
	}

	q := fmt.Sprintf(`
		SELECT t.id, d.tm, d.en, d.ru, t.image_path
		FROM types t
		JOIN dictionary d ON t.name_dictionary_id = d.id
		WHERE ($1 = '' 
		OR d.tm ILIKE '%%' || $1 || '%%' 
		OR d.en ILIKE '%%' || $1 || '%%' 
		OR d.ru ILIKE '%%' || $1 || '%%')
		%s
		ORDER BY t.created_at DESC
		LIMIT $2 OFFSET $3
	`, categoryFilter)

	rows, err := r.client.Query(ctx, q, search, limitInt, offsetInt)

	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	defer rows.Close()

	for rows.Next() {

		var t types.TypesDTO

		if err := rows.Scan(
			&t.Id,
			&t.Name.Tm,
			&t.Name.En,
			&t.Name.Ru,
			&t.ImagePath,
		); err != nil {
			return nil, appresult.ErrInternalServer
		}

		cleanPath := strings.ReplaceAll(t.ImagePath, "\\", "/")
		t.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)

		resp.Types = append(resp.Types, t)

	}

	qCount := fmt.Sprintf(`
		SELECT count(*)
		FROM types t
		JOIN dictionary d ON t.name_dictionary_id = d.id
		WHERE ($1 = '' 
		OR d.tm ILIKE '%%' || $1 || '%%' 
		OR d.en ILIKE '%%' || $1 || '%%' 
		OR d.ru ILIKE '%%' || $1 || '%%')
		%s
	`, categoryFilter)

	err = r.client.QueryRow(ctx, qCount, search).Scan(&count)

	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	resp.Count = count

	return &resp, nil
}

func (r *repository) Update(ctx context.Context, typeId int, dto types.TypeDTO, imagePath string, baseURL string) (*types.TypeOneDTO, error) {

	typeResp, err := r.GetOne(ctx, typeId, "")

	if err != nil {
		return nil, err
	}

	if dto.CategoryID != 0 && dto.CategoryID != typeResp.Category.Id {

		var catExists int

		err := r.client.QueryRow(ctx, `SELECT id FROM categories WHERE id=$1`, dto.CategoryID).Scan(&catExists)

		if err != nil {

			if errors.Is(err, pgx.ErrNoRows) {
				return nil, appresult.ErrNotFoundType(dto.CategoryID, "category")
			}

			return nil, appresult.ErrInternalServer
		}

		typeResp.Category.Id = dto.CategoryID
	}

	if dto.Name.Tm != "" || dto.Name.En != "" || dto.Name.Ru != "" {

		var id int

		q := `
			SELECT t.id
			FROM dictionary d
			JOIN types t ON t.name_dictionary_id = d.id
			WHERE (d.tm=$1 OR d.en=$2 OR d.ru=$3)
			AND t.category_id = $4
		`

		err := r.client.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru, typeResp.Category.Id).Scan(&id)

		if err == nil && id != typeId {
			return nil, appresult.ErrAlreadyData("name")
		}

		_, err = r.client.Exec(ctx, `
			UPDATE dictionary d
			SET tm=$1,en=$2,ru=$3
			FROM types t
			WHERE t.name_dictionary_id=d.id AND t.id=$4
		`, dto.Name.Tm, dto.Name.En, dto.Name.Ru, typeId)

		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if imagePath != "" {

		var img []string
		img = append(img, typeResp.ImagePath)

		utils.DropFiles(&img)

		typeResp.ImagePath = imagePath
	}

	q := `
		UPDATE types 
		SET 
		category_id = $1,
		image_path = $2 
		WHERE id = $3
	`

	_, err = r.client.Exec(ctx, q, dto.CategoryID, typeResp.ImagePath, typeId)

	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return r.GetOne(ctx, typeId, baseURL)
}

func (r *repository) Delete(ctx context.Context, typeId int) error {

	var (
		nameDictionaryId int
		imagePath        string
	)

	q := `
		SELECT name_dictionary_id, image_path
		FROM types
		WHERE id=$1
	`

	err := r.client.QueryRow(ctx, q, typeId).Scan(&nameDictionaryId, &imagePath)

	if err != nil {

		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(typeId, "type")
		}

		return appresult.ErrInternalServer
	}

	if imagePath != "" {

		var img []string
		img = append(img, imagePath)

		utils.DropFiles(&img)
	}

	_, err = r.client.Exec(ctx, `DELETE FROM types WHERE id=$1`, typeId)

	if err != nil {
		return appresult.ErrInternalServer
	}

	_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id=$1`, nameDictionaryId)

	if err != nil {
		return appresult.ErrInternalServer
	}

	return nil
}
