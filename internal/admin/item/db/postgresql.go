package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"restaurants/internal/admin/item"
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

func NewRepository(client postgresql.Client, logger *logging.Logger) item.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, dto item.ItemReqDTO, imagePath string, baseURL string) (*item.ItemGetOneDTO, error) {
	var (
		businessExists, typeExists, businessTypeExists, isChosen bool
		itemId, nameDictId, ingredientDictId, isChosenCount      int
		ingrTm, ingrEn, ingrRu                                   string
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, fmt.Errorf("begin tx: %w", err)
	}

	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	q := `SELECT EXISTS(SELECT 1 FROM businesses WHERE id=$1)`
	err = tx.QueryRow(ctx, q, dto.BusinessId).Scan(&businessExists)
	if err != nil || !businessExists {
		return nil, appresult.ErrNotFoundType(dto.BusinessId, "business")
	}

	q = `SELECT EXISTS(SELECT 1 FROM types WHERE id=$1)`
	err = tx.QueryRow(ctx, q, dto.TypeId).Scan(&typeExists)
	if err != nil || !typeExists {
		return nil, appresult.ErrNotFoundType(dto.TypeId, "type")
	}

	q = `SELECT EXISTS(SELECT 1 FROM businesses_types WHERE businesses_id=$1 AND type_id=$2)`
	err = tx.QueryRow(ctx, q, dto.BusinessId, dto.TypeId).Scan(&businessTypeExists)
	if err != nil || !businessTypeExists {
		error := fmt.Sprintf("connection business id: %d and type", dto.BusinessId)
		return nil, appresult.ErrNotFoundType(dto.TypeId, error)
	}

	q = `INSERT INTO dictionary (tm, en, ru) VALUES ($1,$2,$3) RETURNING id`
	err = tx.QueryRow(ctx, q, dto.Name.Tm, dto.Name.En, dto.Name.Ru).Scan(&nameDictId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	ingrTm, ingrRu, ingrEn = toString(dto.Ingredient)

	q = `INSERT INTO dictionary (tm, en, ru) VALUES ($1,$2,$3) RETURNING id`
	err = tx.QueryRow(ctx, q, ingrTm, ingrEn, ingrRu).Scan(&ingredientDictId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	if dto.IsChosen == true {
		isChosen = true
		q = `SELECT count(*) FROM items WHERE businesses_id = $1 AND is_chosen = true AND id != $2`
		err = tx.QueryRow(ctx, q, dto.BusinessId, itemId).Scan(&isChosenCount)
		if isChosenCount > 3 {
			return nil, appresult.ErrOverLimit(4, "is chosen")
		}
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}
	}

	dto.Value = float32(math.Round(float64(dto.Value)*10) / 10)

	q = `
		INSERT INTO items (name_dictionary_id, ingredient_dictionary_id, 
							image_path, value, businesses_id, type_id,
							is_chosen)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id
	`
	err = tx.QueryRow(ctx, q,
		nameDictId, ingredientDictId, imagePath, dto.Value, dto.BusinessId, dto.TypeId, isChosen,
	).Scan(&itemId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	if len(dto.ItemCategoryIds) > 0 {

		var count int

		checkQuery := `
		SELECT COUNT(*)
		FROM item_categories
		WHERE id = ANY($1)
		AND businesses_id = $2
	`

		err = tx.QueryRow(
			ctx,
			checkQuery,
			dto.ItemCategoryIds,
			dto.BusinessId,
		).Scan(&count)

		if err != nil {
			return nil, appresult.ErrInternalServer
		}

		if count != len(dto.ItemCategoryIds) {
			return nil, appresult.ErrNotFoundTypeStr("item_category in this business")
		}

		q = `INSERT INTO items_item_categories (item_id, item_category_id) VALUES ($1,$2)`
		for _, catId := range dto.ItemCategoryIds {
			_, err = tx.Exec(ctx, q, itemId, catId)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, fmt.Errorf("failed to link category: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	resp, err := r.GetOne(ctx, itemId, baseURL)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	return resp, nil
}

func (r *repository) GetOne(ctx context.Context, itemId int, baseURL string) (*item.ItemGetOneDTO, error) {
	var (
		dto        item.ItemGetOneDTO
		Tm, En, Ru string
	)

	q := `
		SELECT 
			i.id,
			dn.tm, dn.ru, dn.en,
			di.tm, di.ru, di.en,
			i.value,
			i.image_path
		FROM items i
		JOIN dictionary dn ON i.name_dictionary_id = dn.id
		JOIN dictionary di ON i.ingredient_dictionary_id = di.id
		WHERE i.id = $1
	`
	err := r.client.QueryRow(ctx, q, itemId).Scan(
		&dto.Id,
		&dto.Name.Tm, &dto.Name.Ru, &dto.Name.En,
		&dto.Ingredient.Tm, &dto.Ingredient.Ru, &dto.Ingredient.En,
		&dto.Value,
		&dto.ImagePath,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(itemId, "item")
		}
		return nil, err
	}

	q = `
        SELECT d.tm, d.ru, d.en
        FROM items_item_categories iic
		JOIN item_categories ic ON ic.id = iic.item_category_id
        JOIN dictionary d ON ic.name_dictionary_id = d.id
        WHERE iic.item_id = $1
    `
	rows, err := r.client.Query(ctx, q, itemId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var category item.DictionaryDTO
		if err := rows.Scan(&category.Tm, &category.Ru, &category.En); err != nil {
			return nil, err
		}

		if En == "" {
			En = category.En
		} else {
			En += " / " + category.En
		}

		if Ru == "" {
			Ru = category.Ru
		} else {
			Ru += " / " + category.Ru
		}

		if Tm == "" {
			Tm = category.Tm
		} else {
			Tm += " / " + category.Tm
		}
	}

	dto.ItemCategories.En = En
	dto.ItemCategories.Tm = Tm
	dto.ItemCategories.Ru = Ru

	if dto.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(dto.ImagePath, "\\", "/")
		dto.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}
	return &dto, nil
}

func (r *repository) GetAll(ctx context.Context, search, offset, limit string, itemCategoryId string, businessId string, baseURL string) (*item.GetAllWithCount, error) {
	var result item.GetAllWithCount

	offsetInt, err := strconv.Atoi(offset)
	if err != nil || offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, err := strconv.Atoi(limit)
	if err != nil || limitInt < 1 {
		limitInt = 10
	}
	offsetInt = (offsetInt - 1) * limitInt

	businessIdInt, err := strconv.Atoi(businessId)
	if err != nil {
		businessIdInt = 0
	}
	itemCategoryIdInt, err := strconv.Atoi(itemCategoryId)
	if err != nil {
		itemCategoryIdInt = 0
	}

	q := `
		SELECT 
			i.id,
			dn.tm, dn.ru, dn.en,
			di.tm, di.ru, di.en,
			i.value,
			i.image_path,
			i.is_chosen
		FROM items i
		JOIN dictionary dn ON i.name_dictionary_id = dn.id
		JOIN dictionary di ON i.ingredient_dictionary_id = di.id
		WHERE 
			($1 = '' OR dn.tm ILIKE '%' || $1 || '%' OR dn.en ILIKE '%' || $1 || '%' OR dn.ru ILIKE '%' || $1 || '%')
			AND (
				$2 = 0 OR EXISTS (
					SELECT 1
					FROM items_item_categories iic
					WHERE iic.item_id = i.id
					AND iic.item_category_id = $2
				)
			)
			AND ($3 = 0 OR i.businesses_id = $3)
		ORDER BY i.created_at DESC
		LIMIT $4 OFFSET $5;
	`

	rows, err := r.client.Query(ctx, q, search, itemCategoryIdInt, businessIdInt, limitInt, offsetInt)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var it item.ItemGetAllDTO
		if err := rows.Scan(
			&it.Id,
			&it.Name.Tm, &it.Name.Ru, &it.Name.En,
			&it.Ingredient.Tm, &it.Ingredient.Ru, &it.Ingredient.En,
			&it.Value,
			&it.ImagePath,
			&it.IsChosen,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if it.ImagePath != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(it.ImagePath, "\\", "/")
			it.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		result.Items = append(result.Items, it)
	}

	qCount := `
		SELECT COUNT(DISTINCT i.id)
		FROM items i
		JOIN dictionary dn ON i.name_dictionary_id = dn.id
		WHERE 
			($1 = '' OR dn.tm ILIKE '%' || $1 || '%' OR dn.en ILIKE '%' || $1 || '%' OR dn.ru ILIKE '%' || $1 || '%')
			AND (
				$2 = 0 OR EXISTS (
					SELECT 1
					FROM items_item_categories iic
					WHERE iic.item_id = i.id
					AND iic.item_category_id = $2
				)
			)
			AND ($3 = 0 OR i.businesses_id = $3);
	`
	if err := r.client.QueryRow(ctx, qCount, search, itemCategoryIdInt, businessIdInt).Scan(&result.Count); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	return &result, nil
}

func (r *repository) Update(ctx context.Context, itemId int, dto item.ItemReqDTO, imagePath string, baseURL string) (*item.ItemResForUpdateDTO, error) {
	var itm item.ItemForUpdateDTO

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			fmt.Println("rollback error:", rbErr)
		}
	}()

	q := `SELECT 
		name_dictionary_id, ingredient_dictionary_id, 
		image_path, value, businesses_id, type_id,
		is_chosen
		FROM items WHERE id = $1`
	err = tx.QueryRow(ctx, q, itemId).Scan(
		&itm.NameId, &itm.IngredientId, &itm.ImagePath,
		&itm.Value, &itm.BusinessId, &itm.TypeId,
		&itm.IsChosen,
	)
	if err != nil {
		return nil, appresult.ErrNotFoundType(itemId, "item")
	}

	if dto.Name.En != "" && dto.Name.Ru != "" && dto.Name.Tm != "" {
		_, err = tx.Exec(ctx, `UPDATE dictionary SET tm=$1, en=$2, ru=$3 WHERE id=$4`,
			dto.Name.Tm, dto.Name.En, dto.Name.Ru, itm.NameId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if len(dto.Ingredient) > 0 {
		ingrTm, ingrRu, ingrEn := toString(dto.Ingredient)
		_, err = tx.Exec(ctx, `UPDATE dictionary SET tm=$1, en=$2, ru=$3 WHERE id=$4`,
			ingrTm, ingrEn, ingrRu, itm.IngredientId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if dto.BusinessId != 0 {
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM businesses WHERE id=$1)`, dto.BusinessId).Scan(&exists)
		if err != nil || !exists {
			return nil, appresult.ErrNotFoundType(dto.BusinessId, "business")
		}
		itm.BusinessId = dto.BusinessId
	}

	if dto.TypeId != 0 {
		var exists bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM types WHERE id=$1)`, dto.TypeId).Scan(&exists)
		if err != nil || !exists {
			return nil, appresult.ErrNotFoundType(dto.TypeId, "type")
		}
		itm.TypeId = dto.TypeId
	}

	if dto.BusinessId != 0 && dto.TypeId != 0 {
		var ok bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM businesses_types WHERE businesses_id=$1 AND type_id=$2)`, dto.BusinessId, dto.TypeId).Scan(&ok)
		if err != nil || !ok {
			return nil, appresult.ErrNotFoundType(dto.TypeId, fmt.Sprintf("business id %d and type", dto.BusinessId))
		}
	}

	if dto.IsChosen == true {
		var isChosenCount int
		q = `SELECT count(*) FROM items WHERE businesses_id = $1 AND is_chosen = true AND id != $2`
		err = tx.QueryRow(ctx, q, itm.BusinessId, itemId).Scan(&isChosenCount)
		if isChosenCount > 3 {
			return nil, appresult.ErrOverLimit(4, "is chosen")
		}
		itm.IsChosen = true
	} else if dto.IsChosen == false {
		itm.IsChosen = false
	}

	newImage := itm.ImagePath
	if imagePath != "" {
		utils.DropFiles(&[]string{itm.ImagePath})
		newImage = imagePath
	}

	if dto.Value != 0 || imagePath != "" || dto.IsChosen != nil {
		_, err = tx.Exec(ctx, `UPDATE items SET value=$1, image_path=$2, is_chosen=$3 WHERE id=$4`,
			dto.Value, newImage, itm.IsChosen, itemId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
	}

	if len(dto.ItemCategoryIds) > 0 {
		var count int
		checkQuery := `SELECT COUNT(*) FROM item_categories WHERE id = ANY($1) AND businesses_id = $2`
		err = tx.QueryRow(ctx, checkQuery, dto.ItemCategoryIds, dto.BusinessId).Scan(&count)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
		if count != len(dto.ItemCategoryIds) {
			return nil, appresult.ErrNotFoundTypeStr("item_category in this business")
		}
		_, err = tx.Exec(ctx, `DELETE FROM items_item_categories WHERE item_id=$1`, itemId)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}
		for _, catId := range dto.ItemCategoryIds {
			_, err = tx.Exec(ctx, `INSERT INTO items_item_categories (item_id, item_category_id) VALUES ($1, $2)`, itemId, catId)
			if err != nil {
				return nil, appresult.ErrInternalServer
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	updated, err := r.GetForUpdate(ctx, itemId, baseURL)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return updated, nil
}

func (r *repository) Delete(ctx context.Context, itemId int) error {
	var (
		nameDictionaryId       int
		ingredientDictionaryId int
		imagePath              string
	)

	q := `
		SELECT name_dictionary_id, ingredient_dictionary_id, image_path
		FROM items
		WHERE id = $1
	`

	err := r.client.QueryRow(ctx, q, itemId).Scan(&nameDictionaryId, &ingredientDictionaryId, &imagePath)
	if err != nil {
		fmt.Println("error: ", err)
		if errors.Is(err, pgx.ErrNoRows) {
			return appresult.ErrNotFoundType(itemId, "item")
		}
		return appresult.ErrInternalServer
	}

	if imagePath != "" {
		utils.DropFiles(&[]string{imagePath})
	}

	_, err = r.client.Exec(ctx, `DELETE FROM items_item_categories WHERE item_id=$1`, itemId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	_, err = r.client.Exec(ctx, `DELETE FROM items WHERE id=$1`, itemId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	if nameDictionaryId != 0 {
		_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id=$1`, nameDictionaryId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}

	if ingredientDictionaryId != 0 {
		_, err = r.client.Exec(ctx, `DELETE FROM dictionary WHERE id=$1`, ingredientDictionaryId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}

	return nil
}

func (r *repository) GetForUpdate(ctx context.Context, itemId int, baseURL string) (*item.ItemResForUpdateDTO, error) {
	var (
		dto  item.ItemResForUpdateDTO
		ingr item.DictionaryDTO
	)

	q := `
		SELECT 
			i.id,
			dn.tm, dn.ru, dn.en,
			di.tm, di.ru, di.en,
			i.value,
			i.image_path,
			t.id, dc.tm, dc.ru, dc.en,
			i.is_chosen
		FROM items i
		JOIN dictionary dn ON i.name_dictionary_id = dn.id
		JOIN dictionary di ON i.ingredient_dictionary_id = di.id
		JOIN types t ON i.type_id = t.id
		JOIN dictionary dc ON dc.id = t.name_dictionary_id
		WHERE i.id = $1
	`
	err := r.client.QueryRow(ctx, q, itemId).Scan(
		&dto.Id,
		&dto.Name.Tm, &dto.Name.Ru, &dto.Name.En,
		&ingr.Tm, &ingr.Ru, &ingr.En,
		&dto.Value,
		&dto.ImagePath,
		&dto.Type.Id, &dto.Type.Name.Tm, &dto.Type.Name.Ru, &dto.Type.Name.En,
		&dto.IsChosen,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, appresult.ErrNotFoundType(itemId, "item")
		}
		return nil, err
	}

	dto.Ingredient = SplitDictionary(ingr)

	q = `
        SELECT d.id, d.tm, d.ru, d.en
        FROM items_item_categories iic
        JOIN item_categories ic ON iic.item_category_id = ic.id
        JOIN dictionary d ON ic.name_dictionary_id = d.id
        WHERE iic.item_id = $1
    `
	rows, err := r.client.Query(ctx, q, itemId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var cat item.Name
		if err := rows.Scan(&cat.Id, &cat.Name.Tm, &cat.Name.Ru, &cat.Name.En); err != nil {
			return nil, err
		}
		dto.ItemCategories = append(dto.ItemCategories, cat)
	}

	if dto.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(dto.ImagePath, "\\", "/")
		dto.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}

	return &dto, nil
}

func (r *repository) GetItemsByBusiness(ctx context.Context, businessId int, baseURL string) (*[]item.ItemGetAllDTO, error) {
	var result []item.ItemGetAllDTO

	q := `
		(
			SELECT 
				i.id,
				dn.tm, dn.ru, dn.en,
				di.tm, di.ru, di.en,
				i.value,
				i.image_path,
				i.is_chosen
			FROM items i
			JOIN dictionary dn ON i.name_dictionary_id = dn.id
			JOIN dictionary di ON i.ingredient_dictionary_id = di.id
			WHERE i.businesses_id = $1 AND i.is_chosen = true
			ORDER BY i.created_at DESC
			LIMIT 4
		)
		UNION ALL
		(
			SELECT 
				i.id,
				dn.tm, dn.ru, dn.en,
				di.tm, di.ru, di.en,
				i.value,
				i.image_path,
				i.is_chosen
			FROM items i
			JOIN dictionary dn ON i.name_dictionary_id = dn.id
			JOIN dictionary di ON i.ingredient_dictionary_id = di.id
			WHERE i.businesses_id = $1 AND i.is_chosen = false
			ORDER BY i.created_at DESC
			LIMIT 4
		)
		LIMIT 4;
	`

	rows, err := r.client.Query(ctx, q, businessId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var it item.ItemGetAllDTO
		if err := rows.Scan(
			&it.Id,
			&it.Name.Tm, &it.Name.Ru, &it.Name.En,
			&it.Ingredient.Tm, &it.Ingredient.Ru, &it.Ingredient.En,
			&it.Value,
			&it.ImagePath,
			&it.IsChosen,
		); err != nil {
			fmt.Println("error: ", err)
			return nil, appresult.ErrInternalServer
		}

		if it.ImagePath != "" && baseURL != "" {
			cleanPath := strings.ReplaceAll(it.ImagePath, "\\", "/")
			it.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		result = append(result, it)
	}

	return &result, nil
}

func SplitDictionary(dict item.DictionaryDTO) []item.DictionaryDTO {
	var result []item.DictionaryDTO
	tmParts := strings.Split(dict.Tm, "/")
	ruParts := strings.Split(dict.Ru, "/")
	enParts := strings.Split(dict.En, "/")

	for i, _ := range tmParts {
		var item item.DictionaryDTO

		item.Tm = strings.TrimSpace(tmParts[i])
		item.Ru = strings.TrimSpace(ruParts[i])
		item.En = strings.TrimSpace(enParts[i])

		result = append(result, item)
	}
	return result
}

func toString(dto []item.DictionaryDTO) (string, string, string) {
	var ingrTm, ingrEn, ingrRu string
	for indext, ing := range dto {
		if indext != 0 {
			ingrEn = ingrEn + " / " + ing.En
			ingrRu = ingrRu + " / " + ing.Ru
			ingrTm = ingrTm + " / " + ing.Tm
		} else {
			ingrEn = ingrEn + ing.En
			ingrRu = ingrRu + ing.Ru
			ingrTm = ingrTm + ing.Tm
		}
	}

	return ingrTm, ingrRu, ingrEn
}
