package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"restaurants/internal/admin/businesses"
	"restaurants/internal/admin/subcategory"
	"restaurants/internal/appresult"
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

	query := `
		INSERT INTO businesses
			(name, province_id, district_dictionary_id, 
			phone, description_dictionary_id, opens_time, closes_time,
			expires)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	err = tx.QueryRow(ctx, query,
		dto.Name, dto.ProvinceId,
		districtId, dto.Phone, descriptionId,
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
				b.phone,
				p.id,
				p_name.tm, p_name.en, p_name.ru,
				COALESCE(d_district.tm, ''), COALESCE(d_district.en, ''), COALESCE(d_district.ru, ''),
				b.opens_time,
				b.closes_time,
				b.expires
			FROM businesses b
			JOIN provinces p            ON b.province_id = p.id
			JOIN dictionary p_name      ON p.name_dictionary_id = p_name.id
			JOIN dictionary d_district  ON b.district_dictionary_id = d_district.id
			JOIN dictionary d_desc      ON b.description_dictionary_id = d_desc.id
			WHERE b.id = $1;
		`
	err := r.client.QueryRow(ctx, qBusiness, businessId).Scan(
		&res.Id,
		&res.Name,
		&res.Address.Tm, &res.Address.En, &res.Address.Ru,
		&res.Phone,
		&res.Province.Id, &res.Province.Name.Tm, &res.Province.Name.En, &res.Province.Name.Ru,
		&res.Description.Tm, &res.Description.En, &res.Description.Ru,
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

	if len(filter.SubcategoryIds) > 0 {
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM businesses_subcategories bs WHERE bs.businesses_id = b.id AND bs.subcategory_id = ANY($%d))", idx))
		args = append(args, pq.Array(filter.SubcategoryIds))
		idx++
	}

	if filter.ProvinceID != 0 {
		filters = append(filters, fmt.Sprintf("b.province_id = $%d", idx))
		args = append(args, filter.ProvinceID)
		idx++
	}

	if filter.Type != 0 {
		filters = append(filters, fmt.Sprintf("EXISTS (SELECT 1 FROM businesses_types bc WHERE bc.businesses_id = b.id AND bc.type_id = $%d)", idx))
		args = append(args, filter.Type)
		idx++
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
			name, province_id,
			district_dictionary_id, phone, description_dictionary_id,
			opens_time, closes_time, expires
		FROM businesses
		WHERE id = $1
	`, businessId).Scan(
		&res.Name, &res.ProvinceId,
		&res.DistrictId, &res.Phone, &res.DescriptionId,
		&opensTime, &closesTime, &res.Expires,
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

	if dto.ProvinceId != 0 {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM provinces WHERE id = $1)`, dto.ProvinceId).Scan(&exists); err != nil || !exists {
			return appresult.ErrNotFoundType(dto.ProvinceId, "province")
		}
		res.ProvinceId = dto.ProvinceId
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
		SET province_id = $1, phone = $2, name = $3,
			opens_time = $4, closes_time = $5,
			expires = $6
		WHERE id = $7
	`, res.ProvinceId, res.Phone, res.Name,
		res.OpensTime, res.ClosesTime, res.Expires,
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
	var exist bool

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
	SELECT EXISTS (
		SELECT 1 FROM businesses WHERE id = $1
	)
`

	err = tx.QueryRow(ctx, q, businessId).Scan(&exist)
	if err != nil {
		return err
	}

	if !exist {
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

func (r *repository) UpdateStatus(ctx context.Context, businessId int,  status businesses.UpdateStatus)  error {

	var (
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

func (r *repository) Index(ctx context.Context, filter businesses.IndexFilter, baseURL string) (*businesses.Index, error) {
	var (
		index businesses.Index  
	)

	firstInterestingSubcategoryIds, secondInterestingSubcategoryIds, err := findInterestingSubcategory(ctx, r, filter)
	if err != nil {
		return nil, err
	}

	 q := `
   		SELECT DISTINCT ON (b.id) b.id, b.name, img.image_path, b.discount_percent
	FROM businesses b
	LEFT JOIN businesses_subcategories bs ON bs.businesses_id = b.id
	LEFT JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
	WHERE (bs.subcategory_id = ANY($1) OR $1::int[] IS NULL)
	ORDER BY b.id, b.created_at DESC
	LIMIT $2
	`
	newCreated, err := findBusinessesForIndex(ctx, r, q, firstInterestingSubcategoryIds, 5, baseURL)
	if err != nil {
		fmt.Println("1error", err)
		return nil, err
	}

	if len(newCreated) < 5 {
		fallback, err := findBusinessesForIndex(ctx, r, q, secondInterestingSubcategoryIds, 5-len(newCreated), baseURL)
		if err != nil {
			return nil, err
		}
		newCreated = append(newCreated, fallback...)
	}

	q = `
   		SELECT DISTINCT ON (b.id) b.id, b.name, img.image_path, b.discount_percent
	FROM businesses b
	LEFT JOIN businesses_subcategories bs ON bs.businesses_id = b.id
	LEFT JOIN image_businesses img ON img.businesses_id = b.id AND img.is_main = true
	WHERE (bs.subcategory_id = ANY($1) OR $1::int[] IS NULL)
	AND b.discount_percent IS NOT NULL
	ORDER BY b.id, b.created_at DESC
	LIMIT $2
	`

	withDiscounts, err := findBusinessesForIndex(ctx, r, q, firstInterestingSubcategoryIds, 5, baseURL)
	if err != nil {
		return nil, err
	}

	if len(withDiscounts) < 5 {
		fallback, err := findBusinessesForIndex(ctx, r, q, secondInterestingSubcategoryIds, 5-len(withDiscounts), baseURL)
		if err != nil {
			return nil, err
		}
		withDiscounts = append(withDiscounts, fallback...)
	}

	index.NewCreated = newCreated
	index.WithDiscounts = withDiscounts

	return &index, nil
}

func findInterestingSubcategory(ctx context.Context, r *repository, filter businesses.IndexFilter) ([]int, []int, error) {
	 q := `
		WITH first AS (
			SELECT s.id
			FROM subcategories s
			JOIN categories c ON c.id = s.category_id
			WHERE (s.id = ANY($1) OR $1::int[] IS NULL)
			   OR (c.id = ANY($2) OR $2::int[] IS NULL)
		),
		second AS (
			SELECT s.id
			FROM subcategories s
			WHERE (s.category_id = ANY($2) OR $2::int[] IS NULL)
			  AND s.id NOT IN (SELECT id FROM first)
		)
		SELECT id, 1 AS group_num FROM first
		UNION ALL
		SELECT id, 2 AS group_num FROM second
	`

	var subcategoryIds, categoryIds interface{}
	if len(filter.SubcategoryIds) > 0 {
		subcategoryIds = filter.SubcategoryIds
	}
	if len(filter.CategoryIds) > 0 {
		categoryIds = filter.CategoryIds
	}

	rows, err := r.client.Query(ctx, q, subcategoryIds, categoryIds)
	if err != nil {
		return nil, nil, fmt.Errorf("interesting subcategory: %w", err)
	}
	defer rows.Close()

	var firstIds, secondIds []int
	for rows.Next() {
		var id, group int
		if err := rows.Scan(&id, &group); err != nil {
			return nil, nil, fmt.Errorf("scan: %w", err)
		}
		if group == 1 {
			firstIds = append(firstIds, id)
		} else {
			secondIds = append(secondIds, id)
		}
	}

	return firstIds, secondIds, rows.Err()
}

func findBusinessesForIndex(
	ctx context.Context, 
	r *repository, 
	q string,
	interestingSubcategoryIds []int,
	limit int,
	baseURL string,
	)([]businesses.IndexBusinesses, error){

	var newCreated []businesses.IndexBusinesses

	rows, err := r.client.Query(ctx, q, interestingSubcategoryIds, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var data businesses.IndexBusinesses
		if err := rows.Scan(
			&data.Id, 
			&data.Name,
			&data.Image,
			&data.DiscountPercent,
			); err != nil {
			return nil, err
		}
		cleanPath := strings.ReplaceAll(data.Image, "\\", "/")
		data.Image = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		newCreated = append(newCreated, data)
	}

	return newCreated, nil
}
