package clients

type RegisterDTO struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

type OTP struct {
	Code int `json:"code"`
}

type CheckOTP struct {
	Code        int    `json:"code"`
	PhoneNumber string `json:"phone_number"`
}

type ResultsOTP struct {
	IsFirst  bool   `json:"is_first"`
	Token    string `json:"token"`
	ClientId int    `json:"client_id"`
}

type ClientReqDTO struct {
	Name     string `json:"name" binding:"required"`
	LastName string `json:"last_name"`
}

type Profile struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	LastName    string `json:"last_name"`
	PhoneNumber string `json:"phone_number"`
	ImagePath   string `json:"image_path"`
}

type CountAndClients struct {
	Count   int       `json:"count"`
	Clients []Profile `json:"clients"`
}

type Client struct {
	Id       int    `json:"id"`
	FullName string `json:"fullName"`
}

type ClientUpdateDTO struct {
	Name        string `json:"name" binding:"required"`
	LastName    string `json:"last_name"`
	PhoneNumber string `json:"phone_number" binding:"required"`
}
