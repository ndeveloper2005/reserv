package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"restaurants/internal/admin/businesses"
	"restaurants/internal/admin/businessesTable"
	"restaurants/internal/admin/subcategory"
	"restaurants/internal/appresult"
	"restaurants/internal/client/reservation/dbWS"
	"restaurants/internal/enum"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/lib/pq"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) businesses.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

var (
	formatTime = "15:04"
)

func (r *repository) Create(ctx context.Context, userId int, dto businesses.BusinessesReqDTO) (*int, error) {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var (
		districtId, descriptionId, businessId, provinceId, subcategoryID int
		dressCodeId                                                      *int
	)
	queryDictionary := `INSERT INTO dictionary (tm, en, ru) VALUES ($1, $2, $3) RETURNING id`

	q := `SELECT id FROM provinces WHERE id = $1;`
	err = tx.QueryRow(ctx, q, dto.ProvinceId).Scan(&provinceId)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrNotFoundType(provinceId, "provinces")
	}

	for _, subcategoryId := range dto.SubcategoryIds {
		q := `SELECT id FROM subcategories
		WHERE id = $1;`
		err = tx.QueryRow(ctx, q, subcategoryId).Scan(&subcategoryID)
		if err != nil {
			fmt.Println("error:", err)
			return nil, appresult.ErrNotFoundType(subcategoryID, "subcategory")
		}
	}

	err = tx.QueryRow(ctx, queryDictionary, dto.District.Tm, dto.District.En, dto.District.Ru).Scan(&districtId)
	if err != nil {
		fmt.Println("error insert district dictionary:", err)
		return nil, appresult.ErrInternalServer
	}

	err = tx.QueryRow(ctx, queryDictionary, dto.Description.Tm, dto.Description.En, dto.Description.Ru).Scan(&descriptionId)
	if err != nil {
		fmt.Println("error insert description dictionary:", err)
		return nil, appresult.ErrInternalServer
	}

	if dto.DressCode != nil {
		err = tx.QueryRow(ctx, queryDictionary, dto.DressCode.Tm, dto.DressCode.En, dto.DressCode.Ru).Scan(&dressCodeId)
		if err != nil {
			fmt.Println("error insert dress code dictionary:", err)
			return nil, appresult.ErrInternalServer
		}
	}

	_, err = time.Parse(formatTime, dto.OpensTime)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrTime("opens")
	}

	_, err = time.Parse(formatTime, dto.ClosesTime)
	if err != nil {
		fmt.Println("error :", err)
		return nil, appresult.ErrTime("closes")
	}

	roundedRating := math.Round(float64(dto.Rating)*10) / 10

	query := `
		INSERT INTO businesses
			(name, province_id, rating, district_dictionary_id, 
			phone, description_dictionary_id, dress_code_dictionary_id, opens_time, closes_time,
			expires)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	err = tx.QueryRow(ctx, query,
		dto.Name, dto.ProvinceId, roundedRating,
		districtId, dto.Phone, descriptionId, dressCodeId,
		dto.OpensTime, dto.ClosesTime, dto.Expires).Scan(&businessId)
	if err != nil {
		fmt.Println("error insert business:", err)
		return nil, err
	}

	query = `INSERT INTO businesses_subcategories (businesses_id, subcategory_id) VALUES ($1, $2)`
	for _, subcategoryId := range dto.SubcategoryIds {
		if _, err := tx.Exec(ctx, query, businessId, subcategoryId); err != nil {
			fmt.Println("error insert businesses_subcategories:", err)
			return nil, err
		}
	}

	query = `
		INSERT INTO user_businesses
			(user_id, businesses_id, role)
		VALUES
			($1, $2, $3)
	`
	_,err = tx.Exec(ctx, query,
		userId, businessId, enum.RoleManager)
	if err != nil {
		fmt.Println("error insert user in businesses:", err)
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error commit transaction:", err)
		return nil, err
	}

	return &businessId, nil
}

func (r *repository) AddImages(
	ctx context.Context,
	businessId int,
	mainImage string,
	images []string,
	baseURL string,
) (*businesses.BusinessesResDTO, error) {

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	insertImage := func(path string, isMain bool) error {
		query := `INSERT INTO image_businesses (businesses_id, image_path, is_main) VALUES ($1, $2, $3)`
		_, err := tx.Exec(ctx, query, businessId, path, isMain)
		if err != nil {
			return fmt.Errorf("insert image %q: %w", path, err)
		}
		return nil
	}

	if mainImage != "" {
		if err := insertImage(mainImage, true); err != nil {
			return nil, err
		}
	}

	for _, path := range images {
		if err := insertImage(path, false); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	createdBusiness, err := r.GetOne(ctx, businessId, baseURL)
	if err != nil {
		return nil, fmt.Errorf("get business: %w", err)
	}

	return createdBusiness, nil
}

func (r *repository) GetOne(ctx context.Context, businessId int, baseURL string) (*businesses.BusinessesResDTO, error) {
	var (
		res                   businesses.BusinessesResDTO
		opensTime, closesTime time.Time
	)

	qBusiness := `
			SELECT 
				b.id,
				b.name, 
				(p_name.tm || ', ' || d_district.tm)  AS addr_tm,
				(p_name.en || ', ' || d_district.en)  AS addr_en,
				(p_name.ru || ', ' || d_district.ru)  AS addr_ru,
				b.rating, 
				b.phone,
				p.id,
				p_name.tm, p_name.en, p_name.ru,
				d_desc.tm, d_desc.en, d_desc.ru,
				COALESCE(d_dress.tm, ''), COALESCE(d_dress.en, ''), COALESCE(d_dress.ru, ''),
				b.opens_time,
				b.closes_time,
				b.expires
			FROM businesses b
			JOIN provinces p            ON b.province_id = p.id
			JOIN dictionary p_name      ON p.name_dictionary_id = p_name.id
			JOIN dictionary d_district  ON b.district_dictionary_id = d_district.id
			JOIN dictionary d_desc      ON b.description_dictionary_id = d_desc.id
			LEFT JOIN dictionary d_dress ON b.dress_code_dictionary_id = d_dress.id
			WHERE b.id = $1;
		`
	err := r.client.QueryRow(ctx, qBusiness, businessId).Scan(
		&res.Id,
		&res.Name,
		&res.Address.Tm, &res.Address.En, &res.Address.Ru,
		&res.Rating,
		&res.Phone,
		&res.Province.Id, &res.Province.Name.Tm, &res.Province.Name.En, &res.Province.Name.Ru,
		&res.Description.Tm, &res.Description.En, &res.Description.Ru,
		&res.DressCode.Tm, &res.DressCode.En, &res.DressCode.Ru,
		&opensTime,
		&closesTime,
		&res.Expires,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fmt.Println("error: ", err)
			return nil, appresult.ErrNotFoundType(businessId, "businesses")
		}
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	res.OpensTime = opensTime.Format(formatTime)
	res.ClosesTime = closesTime.Format(formatTime)

	res.Types, err = getTypesByBusinessId(r, ctx, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	res.Subcategory, err = FindSubcategories(r, ctx, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	res.Images, err = getImagesByBusinessId(r, ctx, businessId, baseURL)
	if err != nil {
		fmt.Println("error", err)
		return nil, err
	}

	return &res, nil
}

func getImagesByBusinessId(r *repository, ctx context.Context, businessId int, baseURL string) ([]string, error) {
	var images []string
	qImages := `
       		SELECT image_path
        FROM image_businesses
        WHERE businesses_id = $1
        ORDER BY is_main DESC
    `
	imgRows, err := r.client.Query(ctx, qImages, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer imgRows.Close()

	for imgRows.Next() {
		var path string
		if err := imgRows.Scan(&path); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		if baseURL != "" {
			cleanPath := strings.ReplaceAll(path, "\\", "/")
			path = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}
		images = append(images, path)
	}

	return images, nil
}

func getTypesByBusinessId(r *repository, ctx context.Context, businessId int) ([]businesses.ConnectTypes, error) {
	var types []businesses.ConnectTypes
	q := `
			SELECT c.id, d.tm, d.en, d.ru
		FROM businesses_types bt
		JOIN types c ON bt.type_id = c.id
		JOIN dictionary d ON c.name_dictionary_id = d.id
		WHERE bt.businesses_id = $1
	`
	rows, err := r.client.Query(ctx, q, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var typee businesses.ConnectTypes
		if err := rows.Scan(&typee.Id, &typee.Name.Tm, &typee.Name.En, &typee.Name.Ru); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		types = append(types, typee)
	}
	return types, nil
}

func FindSubcategories(r *repository, ctx context.Context, businessId int) ([]subcategory.SubcategoriesDTO, error) {
	var res []subcategory.SubcategoriesDTO

	q := `
        SELECT 
			s.id, ds.tm, ds.en, ds.ru, s.image_path
		FROM businesses_subcategories bs
		JOIN subcategories s ON s.id = bs.subcategory_id
		JOIN dictionary ds ON s.name_dictionary_id = ds.id
		WHERE bs.businesses_id = $1
    `
	rows, err := r.client.Query(ctx, q, businessId)
	if err != nil {
		return res, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var subc subcategory.SubcategoriesDTO
		if err := rows.Scan(
			&subc.Id,
			&subc.Name.Tm,
			&subc.Name.En,
			&subc.Name.Ru,
			&subc.ImagePath,
		); err != nil {
			return res, appresult.ErrInternalServer
		}

		res = append(res, subc)
	}
	return res, nil
}

func (r *repository) Available(ctx context.Context, baseURL string) (*[]businesses.BusinessesAllDTO, error) {
	var (
		businesses []businesses.BusinessesAllDTO
		count      int
	)

	query := `
	    	SELECT id
	    FROM businesses
	    ORDER BY id DESC
	`
	rows, err := r.client.Query(ctx, query)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var resId int
		if err := rows.Scan(&resId); err != nil {
			fmt.Println("error :", err)
			return nil, err
		}

		tables, err := findTablesByBusiness(resId, r, ctx)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		rWS := dbWS.NewRepository(r.client, r.logger)
		fmt.Println("table ", resId, " - ", tables)
		isEmpty, err := dbWS.AvailabilityTime(resId, time.Now(), time.Now(), tables, 1, rWS, ctx)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		if *isEmpty {
			count++
			res, err := r.fetchBusinessById(ctx, resId, baseURL)
			if err != nil {
				return nil, err
			}
			businesses = append(businesses, *res)
			if count == 5 {
				break
			}
		}
	}
	return &businesses, nil
}

func (r *repository) AvailableAll(ctx context.Context, filter businesses.BusinessesFilter, baseURL string) (*businesses.AllAndSum, error) {
	var (
		business     []businesses.BusinessesAllDTO
		count, needB int
	)
	args, queryAll, _ := makeFilter(filter)

	rows, err := r.client.Query(ctx, queryAll, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var bDTO businesses.BusinessesAllDTO
		var typeJSON []byte
		if err := rows.Scan(
			&bDTO.Id,
			&bDTO.Name,
			&bDTO.Image,
			&bDTO.Address.Tm, &bDTO.Address.En, &bDTO.Address.Ru,
			&bDTO.Rating,
			&typeJSON,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		tables, err := findTablesByBusiness(bDTO.Id, r, ctx)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		rWS := dbWS.NewRepository(r.client, r.logger)
		isEmpty, err := dbWS.AvailabilityTime(bDTO.Id, time.Now(), time.Now(), tables, 1, rWS, ctx)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		if len(typeJSON) > 0 {
			if err := json.Unmarshal(typeJSON, &bDTO.Types); err != nil {
				fmt.Println("error: ", err)
				return nil, err
			}
		} else {
			bDTO.Types = []businesses.ConnectTypes{}
		}

		Limit := 10
		Offset := 1
		if filter.Limit > 0 {
			Limit = filter.Limit
		}
		if filter.Offset > 0 {
			Offset = filter.Offset
		}
		Offset = (Offset - 1) * Limit

		if *isEmpty {
			count++
			if count >= Offset+1 && needB != Limit {
				needB++
				cleanPath := strings.ReplaceAll(bDTO.Image, "\\", "/")
				bDTO.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)
				business = append(business, bDTO)
			}
		}
	}

	respo := businesses.AllAndSum{
		Businesses: &business,
		Count:      count,
	}
	return &respo, nil
}

func findTablesByBusiness(businessId int, r *repository, ctx context.Context) ([]businessesTable.BusinessTableDTO, error) {
	var tables []businessesTable.BusinessTableDTO
	query := `
	    	SELECT id, seats, table_count
	    FROM businesses_types
	    WHERE businesses_id = $1
	`
	rows, err := r.client.Query(ctx, query, businessId)
	if err != nil {
		fmt.Println("error :", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var table businessesTable.BusinessTableDTO
		if err := rows.Scan(&table.Id, &table.Seat, &table.TableCount); err != nil {
			fmt.Println("error :", err)
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}

func (r *repository) fetchBusinessesByCategory(ctx context.Context, category string, limit int, baseURL string) ([]businesses.BusinessesAllDTO, error) {
	query := `
		SELECT b.id,
		       b.name,
		       dp.tm, dp.ru, dp.en, 
		       b.rating,
		       COALESCE(img.image_path, ''),
		       COALESCE(
				(
					SELECT json_agg(
						json_build_object(
							'id', c.id,
							'name', json_build_object('tm', d_c.tm, 'ru', d_c.ru, 'en', d_c.en)
						)
					)
					FROM businesses_types bt
					JOIN types c ON c.id = bt.type_id
					JOIN dictionary d_c ON d_c.id = c.name_dictionary_id
					WHERE bt.businesses_id = b.id
				), '[]'
			) AS types
		FROM types t
		         JOIN businesses_types bt ON bt.type_id = t.id
		         JOIN businesses b ON b.id = bt.businesses_id
		         JOIN provinces p ON p.id = b.province_id
		         JOIN dictionary dp ON dp.id = p.name_dictionary_id
		         LEFT JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
		         JOIN dictionary d_type ON d_type.id = t.name_dictionary_id
		WHERE d_type.en = $1
		ORDER BY b.created_at DESC
		LIMIT $2;
	`

	rows, err := r.client.Query(ctx, query, category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []businesses.BusinessesAllDTO
	for rows.Next() {
		var dto businesses.BusinessesAllDTO
		var province businesses.DictionaryDTO
		var typeJSON []byte

		err := rows.Scan(
			&dto.Id,
			&dto.Name,
			&province.Tm, &province.Ru, &province.En,
			&dto.Rating,
			&dto.Image,
			&typeJSON,
		)
		if err != nil {
			return nil, err
		}

		var typee []businesses.ConnectTypes
		if err := json.Unmarshal(typeJSON, &typee); err == nil {
			dto.Types = typee
		}

		if dto.Image != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(dto.Image, "\\", "/")
			dto.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		dto.Address = province

		businessTypes, err := FindSubcategories(r, ctx, dto.Id)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}
		dto.Subcategory = businessTypes

		result = append(result, dto)
	}

	return result, nil
}

func (r *repository) fetchBusinessById(ctx context.Context, businessId int, baseURL string) (*businesses.BusinessesAllDTO, error) {
	var (
		dto      businesses.BusinessesAllDTO
		province businesses.DictionaryDTO
		typeJSON []byte
		types    []businesses.ConnectTypes
	)
	query := `
		SELECT 
		b.id,
		b.name,
		dp.tm, dp.ru, dp.en,
		b.rating,
		COALESCE(img.image_path, '') AS image_path,
		COALESCE(
			(
				SELECT json_agg(
					json_build_object(
						'id', c.id,
						'name', json_build_object(
							'tm', d_c.tm,
							'ru', d_c.ru,
							'en', d_c.en
						)
					)
				)
				FROM businesses_types bc
				JOIN types c ON c.id = bc.type_id
				JOIN dictionary d_c ON d_c.id = c.name_dictionary_id
				WHERE bc.businesses_id = b.id
			), 
			'[]'
		) AS type
	FROM businesses b
	JOIN provinces p ON p.id = b.province_id
	JOIN dictionary dp ON dp.id = p.name_dictionary_id
	LEFT JOIN image_businesses img 
		ON img.businesses_id = b.id AND img.is_main = TRUE
	WHERE b.id = $1;
	`

	err := r.client.QueryRow(ctx, query, businessId).Scan(
		&dto.Id,
		&dto.Name,
		&province.Tm, &province.Ru, &province.En,
		&dto.Rating,
		&dto.Image,
		&typeJSON,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(typeJSON, &types); err == nil {
		dto.Types = types
	}

	if dto.Image != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(dto.Image, "\\", "/")
		dto.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}

	dto.Address = province

	subc, err := FindSubcategories(r, ctx, dto.Id)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}
	dto.Subcategory = subc

	return &dto, nil
}

func (r *repository) GetAll(ctx context.Context, filter businesses.BusinessesFilter, baseURL string) (*[]businesses.BusinessesAllDTO, *int, error) {
	var (
		business []businesses.BusinessesAllDTO
		total    int
	)
	args, queryAll, countQuery := makeFilter(filter)

	if err := r.client.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		fmt.Println("error: ", err)
		return nil, nil, err
	}

	Limit := 10
	Offset := 1
	if filter.Limit > 0 {
		Limit = filter.Limit
	}
	if filter.Offset > 0 {
		Offset = filter.Offset
	}
	Offset = (Offset - 1) * Limit

	queryAll += fmt.Sprintf(" LIMIT %d OFFSET %d", Limit, Offset)

	rows, err := r.client.Query(ctx, queryAll, args...)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			bDTO     businesses.BusinessesAllDTO
			typeJSON []byte
		)
		if err := rows.Scan(
			&bDTO.Id,
			&bDTO.Name,
			&bDTO.Image,
			&bDTO.Address.Tm, &bDTO.Address.En, &bDTO.Address.Ru,
			&bDTO.Rating,
			&typeJSON,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, nil, err
		}

		if len(typeJSON) > 0 {
			if err := json.Unmarshal(typeJSON, &bDTO.Types); err != nil {
				fmt.Println("error: ", err)
				return nil, nil, err
			}
		} else {
			bDTO.Types = []businesses.ConnectTypes{}
		}

		bDTO.Subcategory, err = FindSubcategories(r, ctx, bDTO.Id)

		if err != nil {
			fmt.Println("error: ", err)
			return nil, nil, err
		}

		cleanPath := strings.ReplaceAll(bDTO.Image, "\\", "/")
		bDTO.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		business = append(business, bDTO)
	}

	return &business, &total, nil
}

func makeFilter(filter businesses.BusinessesFilter) ([]interface{}, string, string) {
	var args []interface{}
	idx := 1

	filters := []string{"1=1"}

	if filter.SubcategoryId != 0 {
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM businesses_subcategories bt WHERE bt.businesses_id = b.id AND bt.subcategory_id = $%d)", idx))
		args = append(args, filter.SubcategoryId)
		idx++
	}

	if filter.ProvinceID != 0 {
		filters = append(filters, fmt.Sprintf("b.province_id = $%d", idx))
		args = append(args, filter.ProvinceID)
		idx++
	}

	if len(filter.Types) > 0 {
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM businesses_types bc WHERE bc.businesses_id = b.id AND bc.type_id = ANY($%d))", idx))
		args = append(args, pq.Array(filter.Types))
		idx++
	}

	if len(filter.Rating) > 0 {
		ratingConds := []string{}
		for _, rVal := range filter.Rating {
			switch rVal {
			case 1:
				ratingConds = append(ratingConds, "(b.rating BETWEEN 0 AND 1)")
			case 2:
				ratingConds = append(ratingConds, "(b.rating > 1 AND b.rating <= 2)")
			case 3:
				ratingConds = append(ratingConds, "(b.rating > 2 AND b.rating <= 3)")
			case 4:
				ratingConds = append(ratingConds, "(b.rating > 3 AND b.rating <= 4)")
			case 5:
				ratingConds = append(ratingConds, "(b.rating > 4 AND b.rating <= 5)")
			}
		}
		if len(ratingConds) > 0 {
			filters = append(filters, "("+strings.Join(ratingConds, " OR ")+")")
		}
	}

	if filter.Search != "" {
		filters = append(filters, fmt.Sprintf(`
		(LOWER(b.name) LIKE LOWER('%%' || $%d || '%%') 
		OR LOWER(d_province.tm || ', ' || COALESCE(d_district.tm,'')) LIKE LOWER('%%' || $%d || '%%')
		OR LOWER(d_province.en || ', ' || COALESCE(d_district.en,'')) LIKE LOWER('%%' || $%d || '%%')
		OR LOWER(d_province.ru || ', ' || COALESCE(d_district.ru,'')) LIKE LOWER('%%' || $%d || '%%')
		)`, idx, idx, idx, idx))
		args = append(args, filter.Search)
		idx++
	}

	query := fmt.Sprintf(`
		SELECT 
			b.id,
			b.name,
			img.image_path,
			(d_province.tm || ', ' || COALESCE(d_district.tm,'')) AS address_tm,
			(d_province.en || ', ' || COALESCE(d_district.en,'')) AS address_en,
			(d_province.ru || ', ' || COALESCE(d_district.ru,'')) AS address_ru,
			b.rating,
			COALESCE(
			(
				SELECT json_agg(
					json_build_object(
						'id', c.id,
						'name', json_build_object(
							'tm', d_c.tm,
							'en', d_c.en,
							'ru', d_c.ru
						)
					)
				)
				FROM businesses_types bc
				JOIN types c ON c.id = bc.type_id
				JOIN dictionary d_c ON d_c.id = c.name_dictionary_id
				WHERE bc.businesses_id = b.id
			), '[]'
		) AS types
		FROM businesses b
		JOIN provinces p ON p.id = b.province_id
		JOIN dictionary d_province ON d_province.id = p.name_dictionary_id
		JOIN dictionary d_district ON d_district.id = b.district_dictionary_id
		LEFT JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
		WHERE %s`, strings.Join(filters, " AND "))

	if len(filter.Sort) > 0 {
		orders := []string{}
		for _, s := range filter.Sort {
			switch s {
			case "rating":
				orders = append(orders, "b.rating DESC")
			case "new":
				orders = append(orders, "b.created_at DESC")
			}
		}
		query += " ORDER BY " + strings.Join(orders, ", ")
	} else {
		query += " ORDER BY b.id DESC"
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM businesses b
		JOIN provinces p ON p.id = b.province_id
		JOIN dictionary d_province ON d_province.id = p.name_dictionary_id
		JOIN dictionary d_district ON d_district.id = b.district_dictionary_id
		WHERE %s`, strings.Join(filters, " AND "))

	return args, query, countQuery
}

func (r *repository) Update(ctx context.Context, businessId int, dto businesses.BusinessesReqDTO) error {
	var (
		res                   businesses.BusinessForUpdateDTO
		opensTime, closesTime time.Time
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	err = tx.QueryRow(ctx, `
			SELECT 
			name, province_id, rating,
			district_dictionary_id, phone, description_dictionary_id,
			dress_code_dictionary_id, opens_time, closes_time, expires
		FROM businesses
		WHERE id = $1
	`, businessId).Scan(
		&res.Name, &res.ProvinceId, &res.Rating,
		&res.DistrictId, &res.Phone, &res.DescriptionId,
		&res.DressCodeId, &opensTime, &closesTime, &res.Expires,
	)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrNotFoundType(businessId, "businesses")
	}
	res.OpensTime = opensTime.Format(formatTime)
	res.ClosesTime = closesTime.Format(formatTime)

	updateOrCreateDictionary := func(idPtr *int, dict businesses.DictionaryDTO) (int, error) {
		if dict.Tm == "" && dict.En == "" && dict.Ru == "" {
			return 0, nil
		}

		var currentId int
		if idPtr != nil {
			currentId = *idPtr
		}

		if currentId != 0 {
			_, err := tx.Exec(ctx, `
            UPDATE dictionary
            SET tm = $1, en = $2, ru = $3
            WHERE id = $4
        `, dict.Tm, dict.En, dict.Ru, currentId)
			fmt.Println("error: ", err)
			return 0, err
		} else {
			var newId int
			err := tx.QueryRow(ctx, `
            INSERT INTO dictionary (tm, en, ru)
            VALUES ($1, $2, $3)
            RETURNING id
        `, dict.Tm, dict.En, dict.Ru).Scan(&newId)
			if err != nil {
				fmt.Println("error: ", err)
				return 0, err
			}
			return newId, nil
		}
	}

	if dto.Name != "" {
		res.Name = dto.Name
	}

	if newId, err := updateOrCreateDictionary(&res.DistrictId, dto.District); err != nil {
		return appresult.ErrInternalServer
	} else if newId != 0 {
		res.DistrictId = newId
	}

	if newId, err := updateOrCreateDictionary(&res.DescriptionId, dto.Description); err != nil {
		return appresult.ErrInternalServer
	} else if newId != 0 {
		res.DescriptionId = newId
	}

	if dto.DressCode != nil {
		if newId, err := updateOrCreateDictionary(res.DressCodeId, *dto.DressCode); err != nil {
			return appresult.ErrInternalServer
		} else if newId != 0 {
			res.DressCodeId = &newId
		}
	}

	if dto.ProvinceId != 0 {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provinces WHERE id = $1)`, dto.ProvinceId).Scan(&exists); err != nil || !exists {
			return appresult.ErrNotFoundType(dto.ProvinceId, "province")
		}
		res.ProvinceId = dto.ProvinceId
	}

	if dto.Rating != 0 {
		res.Rating = math.Round(float64(dto.Rating)*10) / 10
	}

	if dto.Phone != "" {
		res.Phone = dto.Phone
	}

	if dto.Expires != 0 {
		res.Expires = dto.Expires
	}

	if dto.ClosesTime != "" {
		_, err = time.Parse(formatTime, dto.ClosesTime)
		if err != nil {
			return appresult.ErrTime("closes")
		}
		res.ClosesTime = dto.ClosesTime
	}

	if dto.OpensTime != "" {
		_, err = time.Parse(formatTime, dto.OpensTime)
		if err != nil {
			return appresult.ErrTime("opens")
		}
		res.OpensTime = dto.OpensTime
	}

	if _, err = tx.Exec(ctx, `
		UPDATE businesses
		SET province_id = $1, rating = $2, phone = $3, name = $4,
			dress_code_dictionary_id = $5, opens_time = $6, closes_time = $7,
			expires = $8
		WHERE id = $9
	`, res.ProvinceId, res.Rating, res.Phone, res.Name,
		res.DressCodeId, res.OpensTime, res.ClosesTime, res.Expires,
		businessId); err != nil {
		return appresult.ErrInternalServer
	}

	if len(dto.SubcategoryIds) > 0 {
		if _, err = tx.Exec(ctx, `DELETE FROM businesses_subcategories WHERE businesses_id = $1`, businessId); err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}

		valueStrings := []string{}
		valueArgs := []interface{}{}
		argIndex := 1
		for _, subcategoruId := range dto.SubcategoryIds {
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d)", argIndex, argIndex+1))
			valueArgs = append(valueArgs, businessId, subcategoruId)
			argIndex += 2
		}
		query := `INSERT INTO businesses_subcategories (businesses_id, subcategory_id) VALUES ` + strings.Join(valueStrings, ", ")
		if _, err = tx.Exec(ctx, query, valueArgs...); err != nil {
			fmt.Println("error: ", err)
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}
func (r *repository) GetAndDeleteImage(ctx context.Context, businessId int, isMain bool) (*[]string, error) {
	var images []string
	query := `
			DELETE FROM image_businesses
		WHERE businesses_id = $1 AND is_main = $2
		RETURNING image_path
	`
	rows, err := r.client.Query(ctx, query, businessId, isMain)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}
		images = append(images, path)
	}

	return &images, nil
}

func (r *repository) Delete(ctx context.Context, businessId int) error {
	var res businesses.BusinessForUpdateDTO

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return appresult.ErrInternalServer
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	q := `
		SELECT 
			province_id, rating,
			district_dictionary_id, phone, description_dictionary_id
		FROM businesses
		WHERE id = $1
	`
	err = tx.QueryRow(ctx, q, businessId).Scan(
		&res.ProvinceId, &res.Rating,
		&res.DistrictId, &res.Phone, &res.DescriptionId,
	)
	if err != nil {
		return appresult.ErrNotFoundType(businessId, "businesses")
	}

	queries := []string{
	`DELETE FROM businesses_subcategories WHERE businesses_id = $1`,
	`DELETE FROM image_businesses WHERE businesses_id = $1`,
	`DELETE FROM businesses_types WHERE businesses_id = $1`,
	`DELETE FROM user_businesses WHERE businesses_id = $1`,
	`DELETE FROM businesses WHERE id = $1`,
}

	for _, q := range queries {
		if _, err = tx.Exec(ctx, q, businessId); err != nil {
			return err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (r *repository) ConnectType(ctx context.Context, businessId int, dto businesses.ConnectType) (*[]businesses.ConnectTypes, error) {
	var (
		typeID     int
		businessID int
		result     []businesses.ConnectTypes
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	q := `SELECT id
			FROM businesses
			WHERE id = $1;
			`
	err = r.client.QueryRow(ctx, q, businessId).Scan(&businessID)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrNotFoundType(businessId, "businesses")
	}

	q = `DELETE FROM businesses_types WHERE businesses_id = $1`
	_, err = tx.Exec(ctx, q, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	for _, typeId := range dto.TypeIds {
		q := `SELECT id
			FROM types
			WHERE id = $1;
			`
		err = r.client.QueryRow(ctx, q, typeId).Scan(&typeID)
		if err != nil {
			fmt.Println("error:", err)
			return nil, appresult.ErrNotFoundType(typeId, "type")
		}
	}
	query := `INSERT INTO businesses_types (businesses_id, type_id) VALUES ($1, $2)`
	for _, typeId := range dto.TypeIds {
		if _, err := tx.Exec(ctx, query, businessId, typeId); err != nil {
			fmt.Println("error insert businesses_types:", err)
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	q = `
		SELECT c.id, d.tm, d.ru, d.en
		FROM types c
		JOIN dictionary d ON c.name_dictionary_id = d.id
		JOIN businesses_types bc ON bc.type_id = c.id
		WHERE bc.businesses_id = $1
	`
	rows, err := r.client.Query(ctx, q, businessId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var typee businesses.ConnectTypes
		if err := rows.Scan(&typee.Id, &typee.Name.Tm, &typee.Name.Ru, &typee.Name.En); err != nil {
			return nil, err
		}
		result = append(result, typee)
	}

	return &result, nil
}

func (r *repository) UpdateStatus(ctx context.Context, businessId int,  userId int, status businesses.UpdateStatus)  error {

	var (
		exists bool
		existsBusinesses bool
	)

	query := `
		SELECT EXISTS (
			SELECT 1
			FROM businesses
			WHERE id = $1 AND status = 'PENDING'
		);
	`
	err := r.client.QueryRow(ctx, query, businessId).Scan(&existsBusinesses)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(businessId, "businesses")
		}
		return err
	}

	if !existsBusinesses {
		return appresult.ErrStatus
	}

	query = `
		SELECT EXISTS (
			SELECT 1
			FROM user_businesses ub
			WHERE (ub.user_id = $1 AND ub.businesses_id = $2 AND ub.role = $3)
			OR (ub.user_id = $1 AND ub.role = 'ADMIN')
		);
	`
	err = r.client.QueryRow(ctx, query, userId, businessId, enum.RoleManager).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return appresult.ErrForbidden
	}

	if strings.HasPrefix(status.Status, "CANCELED") && status.Reason == "" {
		return appresult.ErrReason
	}

	_, err = r.client.Exec(ctx, `
		UPDATE businesses
		SET status = $1, reason = $2, updated_at = now()
		WHERE id = $3
	`, status.Status, status.Reason, businessId)

	if err != nil {
		fmt.Println(err)
		return appresult.ErrInternalServer
	}

	return nil
}

