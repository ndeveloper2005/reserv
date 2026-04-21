package db

import (
	"context"
	"errors"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/internal/client/clients"
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

func NewRepository(client postgresql.Client, logger *logging.Logger) clients.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Register(ctx context.Context, dto clients.RegisterDTO) (int, error) {
	var (
		id int
	)
	q := `
			SELECT id
			FROM clients
			WHERE phone_number = $1
		`
	err := r.client.QueryRow(ctx, q, dto.PhoneNumber).Scan(&id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}

	randomNumber := utils.UniqueNumberGenerator(10000, 99999)
	fmt.Println("RANDOM: ", randomNumber)

	if id == 0 {
		q = `
		INSERT INTO clients (phone_number, otp)
				VALUES ($1, $2)`

		_, err = r.client.Exec(ctx, q, dto.PhoneNumber, randomNumber)
		if err != nil {
			return 0, err
		}

		return randomNumber, nil
	}

	q = `update clients set otp = $1 where id = $2`

	_, err = r.client.Exec(ctx, q, randomNumber, id)
	if err != nil {
		return 0, err
	}

	return randomNumber, nil
}

func (r *repository) CheckOTP(ctx context.Context, dto clients.CheckOTP) (*clients.ResultsOTP, error) {
	var (
		client         clients.ResultsOTP
		otp            int
		name, lastName string
	)
	q := `
			SELECT id, otp, name, last_name
			FROM clients
			WHERE phone_number = $1
		`
	err := r.client.QueryRow(ctx, q, dto.PhoneNumber).Scan(&client.ClientId, &otp, &name, &lastName)

	if err != nil {
		return nil, appresult.ErrNotFoundTypeStr(dto.PhoneNumber)
	}

	if dto.Code != otp {
		return nil, appresult.ErrOTP
	}

	if name == "" && lastName == "" {
		client.IsFirst = true
	} else {
		client.IsFirst = false
	}

	return &client, nil
}

func (r *repository) CreateProfile(ctx context.Context, clientID int, client clients.ClientReqDTO, imagePath string, baseURL string) (*clients.Profile, error) {
	var (
		id int
	)
	q := `
			SELECT id
			FROM clients
			WHERE id = $1
		`
	err := r.client.QueryRow(ctx, q, clientID).Scan(&id)

	if err != nil {
		return nil, appresult.ErrNotFoundType(clientID, "client")
	}

	q = `
		UPDATE clients
		SET name = $1, last_name = $2, image_path = $3
		WHERE id = $4;
	`
	_, err = r.client.Exec(ctx, q, client.Name, client.LastName, imagePath, id)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	profile, err := r.GetProfile(ctx, clientID, baseURL)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (r *repository) GetProfile(ctx context.Context, clientID int, baseURL string) (*clients.Profile, error) {
	var (
		profile clients.Profile
	)
	q := `
			SELECT id, name, last_name, phone_number, image_path
			FROM clients
			WHERE id = $1
		`
	err := r.client.QueryRow(ctx, q, clientID).Scan(&profile.Id, &profile.Name, &profile.LastName, &profile.PhoneNumber, &profile.ImagePath)

	if err != nil {
		return nil, appresult.ErrNotFoundType(clientID, "client")
	}

	if profile.ImagePath != "" && baseURL != "" {
		cleanPath := strings.ReplaceAll(profile.ImagePath, "\\", "/")
		profile.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
	}

	return &profile, nil
}

func (r *repository) UpdateProfile(ctx context.Context, clientID int, client clients.ClientUpdateDTO, imagePath string, baseURL string) (*clients.Profile, error) {
	var (
		image string
	)
	q := `
			SELECT image_path
			FROM clients
			WHERE id = $1
		`
	err := r.client.QueryRow(ctx, q, clientID).Scan(&image)

	if err != nil {
		return nil, appresult.ErrNotFoundType(clientID, "client")
	}

	if image != "" {
		imagges := []string{
			image,
		}
		utils.DropFiles(&imagges)
	}

	q = `
		UPDATE clients
		SET name = $1, last_name = $2, image_path = $3, phone_number = $4
		WHERE id = $5;
	`
	_, err = r.client.Exec(ctx, q, client.Name, client.LastName, imagePath, client.PhoneNumber, clientID)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, appresult.ErrInternalServer
	}

	profile, err := r.GetProfile(ctx, clientID, baseURL)
	if err != nil {
		return nil, err
	}

	return profile, nil
}

func (r *repository) GetAllClient(ctx context.Context, search, offset, limit, baseURL string) (*clients.CountAndClients, error) {
	var (
		count   int
		Clients []clients.Profile
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
		SELECT COUNT(*) 
		FROM clients 
		WHERE name ILIKE '%' || $1 || '%' OR phone_number ILIKE '%' || $1 || '%' OR last_name ILIKE '%' || $1 || '%'
	`
	err = r.client.QueryRow(ctx, q, search).Scan(&count)
	if err != nil {
		fmt.Println("error: ", err)
		return nil, err
	}

	q = `
		SELECT id, name, last_name, phone_number, image_path
		FROM clients
		WHERE name ILIKE '%' || $1 || '%' OR phone_number ILIKE '%' || $1 || '%' OR last_name ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		OFFSET $2 LIMIT $3
	`

	rows, err := r.client.Query(ctx, q, search, offsetInt, limitInt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var profile clients.Profile
		err := rows.Scan(&profile.Id, &profile.Name, &profile.LastName, &profile.PhoneNumber, &profile.ImagePath)
		if err != nil {
			return nil, err
		}

		if profile.ImagePath != "" {
			cleanPath := strings.ReplaceAll(profile.ImagePath, "\\", "/")
			profile.ImagePath = fmt.Sprintf("%s/%s", baseURL, cleanPath)
		}

		Clients = append(Clients, profile)
	}

	return &clients.CountAndClients{
		Count:   count,
		Clients: Clients,
	}, nil
}

func (r *repository) Logout(ctx context.Context, token string) error {
		q := `
		INSERT INTO blacklist (token)
				VALUES ($1)`

		_, err := r.client.Exec(ctx, q, token)
		if err != nil {
			return err
		}

		return nil
}
