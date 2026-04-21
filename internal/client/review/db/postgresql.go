package db

import (
	"context"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/internal/client/review"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
	"strings"
	"time"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) review.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(ctx context.Context, Review review.ReviewReqDTO, clientId int) (*review.ReviewDTO, error) {
	var (
		id     int
		exists bool
	)

	check := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %ss WHERE id = $1)", Review.TypeName)

	err := r.client.QueryRow(ctx, check, Review.TypeId).Scan(&exists)
	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	} else if !exists {
		return nil, appresult.ErrNotFoundType(Review.TypeId, Review.TypeName)
	}

	if Review.TypeName == "restaurant" {
		var exists bool

		query := `
        SELECT EXISTS (
            SELECT 1
            FROM reservations
            WHERE restaurant_id = $1
              AND client_id = $2
              AND status IN ('confirmed', 'completed')
        )
    `
		err := r.client.
			QueryRow(ctx, query, Review.TypeId, clientId).
			Scan(&exists)
		if err != nil {
			fmt.Println("error:", err)
			return nil, appresult.ErrInternalServer
		}
		if !exists {
			return nil, appresult.ErrNotFoundType(
				Review.TypeId,
				"restaurant in reservation",
			)
		}
	}

	q := `
        INSERT INTO reviews (
            client_id, type_id, type_name, over_all, food, service, ambience, value, comment
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        RETURNING id
    `

	err = r.client.QueryRow(ctx, q,
		clientId, Review.TypeId, Review.TypeName, Review.OverAll, Review.Food, Review.Service,
		Review.Ambience, Review.Value, Review.Comment,
	).Scan(&id)

	if err != nil {
		fmt.Println("error:", err)
		return nil, appresult.ErrInternalServer
	}

	resp, err := r.GetClient(ctx, id)
	return resp, nil
}

func (r *repository) GetClient(ctx context.Context, reviewId int) (*review.ReviewDTO, error) {
	var (
		rev            review.ReviewDTO
		Time           time.Time
		name, lastName string
	)

	q := `
        SELECT 
            r.id, c.id, c.name, c.last_name, r.type_id, r.type_name, 
			r.over_all, r.food, r.service, r.ambience, r.value, r.comment,
            r.created_at
        FROM reviews r 
		JOIN clients c ON c.id = r.client_id
        WHERE r.id = $1
    `

	err := r.client.QueryRow(ctx, q, reviewId).Scan(
		&rev.Id,
		&rev.Client.Id,
		&name,
		&lastName,
		&rev.TypeId,
		&rev.TypeName,
		&rev.OverAll,
		&rev.Food,
		&rev.Service,
		&rev.Ambience,
		&rev.Value,
		&rev.Comment,
		&Time,
	)

	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrNotFoundType(reviewId, "review")
	}
	if lastName != "" {
		rev.Client.FullName = fmt.Sprintf("%s %s", lastName, name)
	} else {
		rev.Client.FullName = name
	}
	rev.CreatedAt = Time.Format("02.01.2006")

	return &rev, nil
}

func (r *repository) buildReviewQuery(filter review.ReviewFilter) (string, []interface{}, int) {
	baseQuery := `
		FROM reviews rv
		JOIN clients c ON c.id = rv.client_id
	`

	conditions := []string{"1=1"}
	args := []interface{}{}
	argID := 1

	if filter.ClientId > 0 {
		conditions = append(conditions, fmt.Sprintf("rv.client_id = $%d", argID))
		args = append(args, filter.ClientId)
		argID++
	}

	if filter.TypeId > 0 {
		conditions = append(conditions, fmt.Sprintf("rv.type_id = $%d", argID))
		args = append(args, filter.TypeId)
		argID++
	}

	if filter.TypeName != "" {
		conditions = append(conditions, fmt.Sprintf("rv.type_name = $%d", argID))
		args = append(args, filter.TypeName)
		argID++
	}

	if filter.Category != "" {
		switch filter.Category {
		case "recommended":
			conditions = append(conditions, "rv.over_all >= 4")
		case "average":
			conditions = append(conditions, "rv.over_all IN (2,3)")
		case "badly":
			conditions = append(conditions, "rv.over_all = 1")
		}
	}

	whereSQL := "WHERE " + strings.Join(conditions, " AND ")

	return baseQuery + " " + whereSQL, args, argID
}

func (r *repository) fetchReviews(ctx context.Context, query string, args ...interface{}) (*[]review.ReviewALLDTO, error) {
	rows, err := r.client.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []review.ReviewALLDTO

	for rows.Next() {
		var (
			rev       review.ReviewALLDTO
			createdAt time.Time
			name      string
			lastName  string
		)

		err := rows.Scan(
			&rev.Id,
			&rev.OverAll,
			&rev.Comment,
			&createdAt,
			&rev.Client.Id,
			&name,
			&lastName,
		)
		if err != nil {
			return nil, err
		}
		if lastName != "" {
			rev.Client.FullName = fmt.Sprintf("%s %s", lastName, name)
		} else {
			rev.Client.FullName = name
		}
		rev.CreatedAt = createdAt.Format("02.01.2006")
		reviews = append(reviews, rev)
	}

	return &reviews, nil
}

func (r *repository) GetAll(ctx context.Context, filter review.ReviewFilter) (*review.CountAndReviews, error) {
	limit := 10
	offset := 1
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	if filter.Offset > 0 {
		offset = filter.Offset
	}
	offset = (offset - 1) * limit

	baseQuery, args, argID := r.buildReviewQuery(filter)

	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int
	if err := r.client.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	selectQuery := `
		SELECT rv.id,
		       rv.over_all,
		       rv.comment,
		       rv.created_at,
		       c.id,
		       c.name,
			   c.last_name
		` + baseQuery + `
		ORDER BY rv.created_at DESC
		LIMIT $` + fmt.Sprint(argID) + ` OFFSET $` + fmt.Sprint(argID+1)

	args = append(args, limit, offset)

	reviews, err := r.fetchReviews(ctx, selectQuery, args...)
	if err != nil {
		return nil, err
	}

	return &review.CountAndReviews{
		Count:   total,
		Reviews: *reviews,
	}, nil
}

func (r *repository) FindByFilter(ctx context.Context, filter review.ReviewFilter) (*[]review.ReviewALLDTO, error) {
	limit := 10
	offset := 1
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	if filter.Offset > 0 {
		offset = filter.Offset
	}
	offset = (offset - 1) * limit

	baseQuery, args, argID := r.buildReviewQuery(filter)

	selectQuery := `
		SELECT rv.id,
		       rv.over_all,
		       rv.comment,
		       rv.created_at,
		       c.id,
		       c.name,
			   c.last_name
		` + baseQuery + `
		ORDER BY rv.created_at DESC
		LIMIT $` + fmt.Sprint(argID) + ` OFFSET $` + fmt.Sprint(argID+1)

	args = append(args, limit, offset)

	return r.fetchReviews(ctx, selectQuery, args...)
}

func (r *repository) Delete(ctx context.Context, reviewId int) error {
	var id int

	q := `
        SELECT id
        FROM reviews
        WHERE id = $1
    `
	err := r.client.QueryRow(ctx, q, reviewId).Scan(&id)
	if err != nil {
		return appresult.ErrNotFoundType(reviewId, "review")
	}

	q = `
        DELETE FROM reviews
        WHERE id = $1;
    `
	_, err = r.client.Exec(ctx, q, reviewId)
	if err != nil {
		fmt.Println("error: ", err)
		return appresult.ErrInternalServer
	}

	return nil
}

func (r *repository) GetAveragesByRestaurantID(ctx context.Context, typeId int, typeName string) (*review.RatingsAvg, error) {
	var avg review.RatingsAvg
	query := `
		SELECT 
			COALESCE(AVG(food), 0)::numeric(10,1),
			COALESCE(AVG(service), 0)::numeric(10,1),
			COALESCE(AVG(ambience), 0)::numeric(10,1),
			COALESCE(AVG(value), 0)::numeric(10,1),
			COALESCE(AVG(over_all), 0)::numeric(10,1)
		FROM reviews
		WHERE type_id = $1 AND type_name = $2
	`
	err := r.client.QueryRow(ctx, query, typeId, typeName).Scan(
		&avg.Food,
		&avg.Service,
		&avg.Ambience,
		&avg.Value,
		&avg.OverAll,
	)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	return &avg, nil
}
