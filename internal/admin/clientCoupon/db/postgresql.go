package db

import (
	"context"
	"fmt"
	"restaurants/internal/admin/clientCoupon"
	"restaurants/internal/appresult"
	"restaurants/internal/client/user"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strconv"
	"strings"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) clientCoupon.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, businessId int, dto clientCoupon.AssignCouponReqDTO) (*clientCoupon.GetCouponsByClient, error) {
	var (
		checkBusinessId int
		exists          bool
	)
	q := `SELECT EXISTS(SELECT 1 FROM businesses WHERE id = $1)`
	err := r.client.QueryRow(ctx, q, businessId).Scan(&exists)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	}
	if !exists {
		return nil, appresult.ErrNotFoundType(businessId, "businesses")
	}

	q = `SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)`
	err = r.client.QueryRow(ctx, q, dto.ClientId).Scan(&exists)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	}
	if !exists {
		return nil, appresult.ErrNotFoundType(dto.ClientId, "client")
	}

	q = `SELECT businesses_id FROM businesses_coupons WHERE id = $1`
	for _, businessCouponId := range dto.BusinessCouponIds {
		err := r.client.QueryRow(ctx, q, businessCouponId).Scan(&checkBusinessId)
		if err != nil {
			fmt.Println("error:", err)
			return nil, appresult.ErrInternalServer
		}
		if checkBusinessId != businessId {
			return nil, appresult.ErrMissingParam
		}
	}

	for _, businessCouponId := range dto.BusinessCouponIds {
		q := `INSERT INTO client_coupons (client_id, businesses_coupon_id) VALUES ($1, $2) RETURNING id`
		_, err := r.client.Exec(ctx, q, dto.ClientId, businessCouponId)
		if err != nil {
			fmt.Println("error:", err)
			return nil, appresult.ErrInternalServer
		}
	}

	return nil, nil
}

func (r *repository) GetCouponsForClient(ctx context.Context, clientId int, limit, offset, baseURL string) (*clientCoupon.GetBusinessesCoupons, error) {
	var (
		results []clientCoupon.GetCouponsByClient
		total   int
	)

	offsetInt, _ := strconv.Atoi(offset)
	if offsetInt < 1 {
		offsetInt = 1
	}
	limitInt, _ := strconv.Atoi(limit)
	if limitInt < 1 {
		limitInt = 10
	}
	offsetInt = (offsetInt - 1) * limitInt

	countQuery := `
        SELECT COUNT(DISTINCT b.id)
        FROM client_coupons cc
        JOIN businesses_coupons bc ON bc.id = cc.businesses_coupon_id
        JOIN businesses b ON b.id = bc.businesses_id
        WHERE cc.client_id = $1
        AND (cc.created_at + bc.life * INTERVAL '1 day') >= NOW()
    `
	err := r.client.QueryRow(ctx, countQuery, clientId).Scan(&total)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	businessQuery := fmt.Sprintf(`
        SELECT DISTINCT 
            b.id,
            ir.image_path,
            b.rating,
            b.name,
            (d_province.tm || ', ' || COALESCE(d_district.tm,'')) AS address_tm,
            (d_province.en || ', ' || COALESCE(d_district.en,'')) AS address_en,
            (d_province.ru || ', ' || COALESCE(d_district.ru,'')) AS address_ru
        FROM client_coupons cc
        JOIN businesses_coupons bc ON bc.id = cc.businesses_coupon_id
        JOIN businesses b ON b.id = bc.businesses_id
        JOIN image_businesses ir ON ir.businesses_id = b.id AND ir.is_main = true
        JOIN provinces p ON p.id = b.province_id
        JOIN dictionary d_province ON d_province.id = p.name_dictionary_id
        JOIN dictionary d_district ON d_district.id = b.district_dictionary_id
        WHERE cc.client_id = $1
        AND (cc.created_at + bc.life * INTERVAL '1 day') >= NOW()
		LIMIT %d OFFSET %d
    `, limitInt, offsetInt)

	rows, err := r.client.Query(ctx, businessQuery, clientId)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var business clientCoupon.Business
		err = rows.Scan(
			&business.Id,
			&business.Images,
			&business.Rating,
			&business.Name,
			&business.Address.Tm,
			&business.Address.En,
			&business.Address.Ru,
		)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		if baseURL != "" {
			cleanPath := strings.ReplaceAll(business.Images, "\\", "/")
			business.Images = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		couponRows, err := r.client.Query(ctx, `
            SELECT 
                cc.id,
                d.tm, d.en, d.ru,
                DATE_PART('day', (cc.created_at + bc.life * INTERVAL '1 day') - NOW()) AS expire_day,
                (cc.reservation_id IS NOT NULL) AS is_used
            FROM client_coupons cc
            JOIN businesses_coupons bc ON bc.id = cc.businesses_coupon_id
            JOIN dictionary d ON d.id = bc.coupon_dictionary_id
            WHERE bc.businesses_id = $1
            AND cc.client_id = $2
            AND (cc.created_at + bc.life * INTERVAL '1 day') >= NOW()
            ORDER BY cc.created_at DESC
        `, business.Id, clientId)
		if err != nil {
			fmt.Println("error: ", err)
			return nil, err
		}

		var coupons []clientCoupon.Coupon
		for couponRows.Next() {
			var c clientCoupon.Coupon
			err := couponRows.Scan(
				&c.Id,
				&c.Coupon.Tm, &c.Coupon.En, &c.Coupon.Ru,
				&c.ExpireDay,
				&c.IsUsed,
			)
			if err != nil {
				fmt.Println("error: ", err)
				return nil, err
			}
			coupons = append(coupons, c)
		}

		business.CountCoupon = len(coupons)
		results = append(results, clientCoupon.GetCouponsByClient{
			Business: business,
			Coupons:  coupons,
		})
	}

	return &clientCoupon.GetBusinessesCoupons{
		Count:           total,
		BusinessCoupons: results,
	}, nil
}

func (r *repository) GetCouponsForBusiness(ctx context.Context, businessId int, limit, offset string) (*clientCoupon.GetCouponsByBusiness, error) {

	var (
		results []clientCoupon.GetCouponsForBusiness
		total   int
	)

	offsetInt, _ := strconv.Atoi(offset)
	if offsetInt < 1 {
		offsetInt = 1
	}

	limitInt, _ := strconv.Atoi(limit)
	if limitInt < 1 {
		limitInt = 10
	}

	offsetInt = (offsetInt - 1) * limitInt

	countQuery := `
		SELECT COUNT(DISTINCT c.id)
		FROM client_coupons cc
		JOIN businesses_coupons bc ON bc.id = cc.businesses_coupon_id
		JOIN clients c ON c.id = cc.client_id
		WHERE bc.businesses_id = $1
		AND (cc.created_at + bc.life * INTERVAL '1 day') >= NOW()
	`

	err := r.client.QueryRow(ctx, countQuery, businessId).Scan(&total)
	if err != nil {
		return nil, err
	}

	clientQuery := `
		SELECT DISTINCT
			c.id,
			c.name,
			c.last_name,
			c.phone_number,
			c.image_path
		FROM client_coupons cc
		JOIN businesses_coupons bc ON bc.id = cc.businesses_coupon_id
		JOIN clients c ON c.id = cc.client_id
		WHERE bc.businesses_id = $1
		AND (cc.created_at + bc.life * INTERVAL '1 day') >= NOW()
		LIMIT $2 OFFSET $3
	`

	rows, err := r.client.Query(ctx, clientQuery, businessId, limitInt, offsetInt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {

		var client clients.Profile

		err := rows.Scan(
			&client.Id,
			&client.Name,
			&client.LastName,
			&client.PhoneNumber,
			&client.ImagePath,
		)
		if err != nil {
			return nil, err
		}

		couponRows, err := r.client.Query(ctx, `
			SELECT 
				cc.id,
				d.tm, d.en, d.ru,
				DATE_PART('day', (cc.created_at + bc.life * INTERVAL '1 day') - NOW()) AS expire_day,
				(cc.reservation_id IS NOT NULL) AS is_used
			FROM client_coupons cc
			JOIN businesses_coupons bc ON bc.id = cc.businesses_coupon_id
			JOIN dictionary d ON d.id = bc.coupon_dictionary_id
			WHERE bc.businesses_id = $1
			AND cc.client_id = $2
			AND (cc.created_at + bc.life * INTERVAL '1 day') >= NOW()
			ORDER BY cc.created_at DESC
		`, businessId, client.Id)

		if err != nil {
			return nil, err
		}

		var coupons []clientCoupon.Coupon

		for couponRows.Next() {

			var c clientCoupon.Coupon

			err := couponRows.Scan(
				&c.Id,
				&c.Coupon.Tm,
				&c.Coupon.En,
				&c.Coupon.Ru,
				&c.ExpireDay,
				&c.IsUsed,
			)
			if err != nil {
				return nil, err
			}

			coupons = append(coupons, c)
		}

		results = append(results, clientCoupon.GetCouponsForBusiness{
			Client:  client,
			Coupons: coupons,
		})
	}

	return &clientCoupon.GetCouponsByBusiness{
		ClientCount:   total,
		ClientCoupons: results,
	}, nil
}

func (r *repository) Delete(ctx context.Context, assignId int) error {
	q := `DELETE FROM client_coupons WHERE id = $1`

	result, err := r.client.Exec(ctx, q, assignId)
	if err != nil {
		fmt.Println("error:", err)
		return appresult.ErrInternalServer
	}

	if result.RowsAffected() == 0 {
		return appresult.ErrNotFoundType(assignId, "client_coupon")
	}

	return nil
}
