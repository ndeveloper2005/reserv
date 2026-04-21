package db

import (
	"context"
	"fmt"
	"restaurants/internal/admin/businessesCoupon"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strconv"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) businessesCoupon.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, businessCoupon businessesCoupon.BusinessCouponReqDTO, businessId int) error {
	tx, err := r.client.Begin(ctx)
	if err != nil {
		fmt.Println("error begin tx:", err)
		return appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	var (
		exists       bool
		dictionaryId int
	)

	q := `	
		SELECT EXISTS(SELECT 1 FROM businesses WHERE id=$1)
	`
	err = tx.QueryRow(ctx, q, businessId).Scan(&exists)
	if err != nil {
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(businessId, "business")
	}

	q = `
	SELECT EXISTS(
		SELECT 1
		FROM businesses_coupons bc
		JOIN dictionary d ON d.id = bc.coupon_dictionary_id
		WHERE bc.businesses_id=$1
		AND d.tm=$2
		AND d.en=$3
		AND d.ru=$4
	)
`

	err = tx.QueryRow(
		ctx,
		q,
		businessId,
		businessCoupon.BusinessCoupon.Tm,
		businessCoupon.BusinessCoupon.En,
		businessCoupon.BusinessCoupon.Ru,
	).Scan(&exists)

	if err != nil {
		return appresult.ErrInternalServer
	}

	if exists {
		return appresult.ErrAlreadyData("businesses coupon")
	}

	qDictionary := `
		INSERT INTO dictionary (tm, en, ru) 
		VALUES ($1, $2, $3) RETURNING id
	`
	err = tx.QueryRow(ctx, qDictionary, businessCoupon.BusinessCoupon.Tm, businessCoupon.BusinessCoupon.En, businessCoupon.BusinessCoupon.Ru).Scan(&dictionaryId)
	if err != nil {
		fmt.Println("error:", err)
		return appresult.ErrInternalServer
	}

	q = `
		INSERT INTO businesses_coupons (businesses_id, coupon_dictionary_id, life) 
		VALUES ($1, $2, $3)
	`
	if _, err := tx.Exec(ctx, q, businessId, dictionaryId, businessCoupon.Life); err != nil {
		fmt.Println("error:", err)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error commit transaction:", err)
		return err
	}

	return nil
}

func (r *repository) GetCoupons(ctx context.Context, businessId int, limit, offset string) (*businessesCoupon.GetCouponsByBusiness, error) {
	var (
		coupons []businessesCoupon.CouponsDTO
		count   int
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
        SELECT
            bc.id, d.tm ,d.en, d.ru, bc.life
        FROM businesses_coupons bc
		JOIN dictionary d ON d.id = bc.coupon_dictionary_id
        WHERE bc.businesses_id = $1
		LIMIT $2 OFFSET $3;
    `
	rows, err := r.client.Query(ctx, q, businessId, limitInt, offsetInt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var coupon businessesCoupon.CouponsDTO

		err := rows.Scan(
			&coupon.Id,
			&coupon.Coupon.Tm,
			&coupon.Coupon.En,
			&coupon.Coupon.Ru,
			&coupon.Life,
		)
		if err != nil {
			fmt.Println("error:", err)
			return nil, err
		}

		coupons = append(coupons, coupon)
	}

	q = `
        SELECT
            count(*)
        FROM businesses_coupons bc
		JOIN dictionary d ON d.id = bc.coupon_dictionary_id
        WHERE bc.businesses_id = $1;
    `
	err = r.client.QueryRow(ctx, q, businessId).Scan(&count)
	if err != nil {
		return nil, err
	}

	businessCoupons := businessesCoupon.GetCouponsByBusiness{
		Count:   count,
		Coupons: coupons,
	}
	return &businessCoupons, nil
}

func (r *repository) Update(ctx context.Context, businessCouponId int, dto businessesCoupon.UpdateBusinessCouponReqDTO) error {
	var (
		dictionaryId, businessId, life int
	)

	tx, err := r.client.Begin(ctx)
	if err != nil {
		return appresult.ErrInternalServer
	}
	defer tx.Rollback(ctx)

	q := `
		SELECT businesses_id, coupon_dictionary_id, life
		FROM businesses_coupons
		WHERE id = $1
	`
	err = tx.QueryRow(ctx, q, businessCouponId).Scan(&businessId, &dictionaryId, &life)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrNotFoundType(businessCouponId, "business coupon")
	}

	if dto.BusinessCoupon.En != "" && dto.BusinessCoupon.Tm != "" && dto.BusinessCoupon.Ru != "" {
		var exists bool
		q := `
		SELECT EXISTS(
			SELECT 1
			FROM businesses_coupons bc
			JOIN dictionary d ON d.id = bc.coupon_dictionary_id
			WHERE bc.businesses_id=$1
			AND d.tm=$2
			AND d.en=$3
			AND d.ru=$4
			AND bc.id!=$5
		)
	`
		err := tx.QueryRow(
			ctx,
			q,
			businessId,
			dto.BusinessCoupon.Tm,
			dto.BusinessCoupon.En,
			dto.BusinessCoupon.Ru,
			businessCouponId,
		).Scan(&exists)

		if err != nil {
			return appresult.ErrInternalServer
		}

		if exists {
			return appresult.ErrAlreadyData("businesses coupon")
		}

		q = `
		UPDATE dictionary
		SET tm = $1, en = $2, ru = $3
		WHERE id = $4
	`
		_, err = tx.Exec(ctx, q, dto.BusinessCoupon.Tm, dto.BusinessCoupon.En, dto.BusinessCoupon.Ru, dictionaryId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}

	if dto.Life != 0 {
		q = `
		UPDATE businesses_coupons
		SET life = $1
		WHERE id = $2
	`
		_, err = tx.Exec(ctx, q, dto.Life, businessCouponId)
		if err != nil {
			fmt.Println("error: ", err)
			return appresult.ErrInternalServer
		}
	}
	if err := tx.Commit(ctx); err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, businessCouponId int) error {
	var dictionaryId int

	q := `
		SELECT coupon_dictionary_id
		FROM businesses_coupons
		WHERE id = $1
    `
	err := r.client.QueryRow(ctx, q, businessCouponId).Scan(&dictionaryId)
	if err != nil {
		return appresult.ErrNotFoundType(businessCouponId, "business coupon id")
	}

	q = `
        	DELETE FROM businesses_coupons
        WHERE id = $1;
    `
	_, err = r.client.Exec(ctx, q, businessCouponId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	q = `
        	DELETE FROM dictionary
        WHERE id = $1;
    `
	_, err = r.client.Exec(ctx, q, dictionaryId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}
