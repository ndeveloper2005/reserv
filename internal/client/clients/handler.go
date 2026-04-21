package clients

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/internal/handlers"
	"restaurants/pkg/logging"
	"restaurants/pkg/sms_sender"
	"restaurants/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	registerURL = "/registration"
	checkOTP    = "/checkOTP"
	profile     = "/profile"
	logout      = "/logout"
	clientURL   = ""
)

type handler struct {
	logger         *logging.Logger
	repository     Repository
	utilRepository utils.Repository
	smsSender      *sms_sender.Client
	client         *pgxpool.Pool
}

func NewHandler(client *pgxpool.Pool, logger *logging.Logger, repository Repository,
	utilRepository utils.Repository, smsSender *sms_sender.Client) handlers.Handler {
	return &handler{
		logger:         logger,
		repository:     repository,
		utilRepository: utilRepository,
		smsSender:      smsSender,
		client:         client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(registerURL, h.register)
	router.POST(checkOTP, h.checkOTP)
	router.POST(profile, h.createProfile)
	router.GET(profile, h.getProfile)
	router.PUT(clientURL, h.update)
	router.GET(clientURL, h.getAll)
	router.POST(logout, h.logout)
}

func (h *handler) register(c *gin.Context) {
	var (
		register RegisterDTO
	)
	if err := c.ShouldBindJSON(&register); err != nil {
		appresult.HandleError(c, err)
		return
	}

	randomNumber, err := h.repository.Register(c, register)
	if err != nil {
		log.Println("[ERROR]", "failed to register client:", err)
		appresult.HandleError(c, err)
		return
	}

	if err := h.smsSender.SendOtp(register.PhoneNumber, randomNumber); err != nil {
		h.logger.Errorln("failed to send otp:", err, "; phone:", register.PhoneNumber)
		c.JSON(http.StatusInternalServerError,
			appresult.NewAppError(err, "failed to send sms", "500"))
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"otp": randomNumber,
	})
}

func (h *handler) checkOTP(c *gin.Context) {
	var (
		otp CheckOTP
	)

	if err := c.ShouldBindJSON(&otp); err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.CheckOTP(context.TODO(), otp)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token, err := utils.GenerateTokenPair(resp.ClientId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp.Token = token
	c.JSON(http.StatusOK, resp)
}

func (h *handler) createProfile(c *gin.Context) {
	var (
		client    ClientReqDTO
		imagePath string
	)
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &client); err != nil {
		appresult.HandleError(c, err)
		return
	}

	uploadDir := filepath.Join("uploads/client")
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err == nil {
		imagePath, err = utils.SaveUploadedFile(c, image, uploadDir)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.CreateProfile(context.TODO(), clientId, client, imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getProfile(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetProfile(context.TODO(), clientId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		client    ClientUpdateDTO
		imagePath string
	)
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &client); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	uploadDir := filepath.Join("uploads/client")
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err == nil {
		imagePath, err = utils.SaveUploadedFile(c, image, uploadDir)
		if err != nil {
			appresult.HandleError(c, err)
			return
		}
	}
	if client.Name == "" || client.PhoneNumber == "" {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.UpdateProfile(context.TODO(), clientId, client, imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	
	if *role != enum.RoleAdmin &&  *role != enum.RoleManager &&  *role != enum.RoleEmployee{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	search := c.Query("search")
	offset := c.Query("offset")
	limit := c.Query("limit")
	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAllClient(context.TODO(), search, offset, limit, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) logout(c *gin.Context) {
	_, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	token := c.GetHeader("Authorization")
	token = token[7:]

	err = h.repository.Logout(context.TODO(), token)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "success!!!")
}

func (h *handler) extractUserIdAndRole(c *gin.Context) (*string, error) {
	userID, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		return nil, err
	}

	role, err := h.utilRepository.UserRoleById(context.TODO(), userID)
	if err != nil {
		return nil, err
	}

	return role, nil
}
