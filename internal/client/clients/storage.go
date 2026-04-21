package clients

import "context"

type Repository interface {
	Register(ctx context.Context, dto RegisterDTO) (int, error)
	CheckOTP(ctx context.Context, dto CheckOTP) (*ResultsOTP, error)
	CreateProfile(ctx context.Context, clientID int, client ClientReqDTO, imagePath string, baseURL string) (*Profile, error)
	GetProfile(ctx context.Context, clientID int, baseURL string) (*Profile, error)
	UpdateProfile(ctx context.Context, clientID int, client ClientUpdateDTO, imagePath string, baseURL string) (*Profile, error)
	GetAllClient(ctx context.Context, search, offset, limit, baseURL string) (*CountAndClients, error)
	Logout(ctx context.Context, token string) error
}
