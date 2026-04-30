package itemCategory

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"restaurants/internal/appresult"
	"restaurants/internal/enum"
	"restaurants/internal/handlers"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	itemCategoryURL   = ""
	itemCategoryIdURL = "/:id"
)

type handler struct {
	logger          *logging.Logger
	repository      Repository
	utilsRepository utils.Repository
	client          *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, utilsRepository utils.Repository, client *pgxpool.Pool) handlers.Handler {
	return &handler{
		logger:          logger,
		repository:      repository,
		utilsRepository: utilsRepository,
		client:          client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(itemCategoryURL, h.create)
	router.GET(itemCategoryIdURL, h.getOne)
	router.GET(itemCategoryURL, h.getAll)
	router.PATCH(itemCategoryIdURL, h.update)
	router.DELETE(itemCategoryIdURL, h.delete)
}

func (h *handler) create(c *gin.Context) {
	var dto ItemCategoryCreateDTO

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &dto); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	uploadDir := filepath.Join("uploads/item_category")
	imagePath, err := utils.SaveUploadedFile(c, image, uploadDir)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.Create(context.TODO(), dto, imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")
	itemCategoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetOne(context.TODO(), itemCategoryId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	businessId := c.Query("businesses_id")
	limit := c.Query("limit")
	offset := c.Query("offset")
	search := c.Query("search")

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAll(context.TODO(), businessId, search, limit, offset, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		itemCategory ItemCategoryNameDTO
		imagePath    string
		uploadDir    string
	)

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemCategoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &itemCategory); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	uploadDir = filepath.Join("uploads/item_category")
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
	resp, err := h.repository.Update(context.TODO(), itemCategoryId, itemCategory, imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemCategoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), itemCategoryId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "success!!!")
}

func (h *handler) extractUserIdAndRole(c *gin.Context) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil && userId != -1{
		return nil, err
	}
	if userId != -1 {
		role, err := h.utilsRepository.UserRoleById(context.TODO(), userId, nil)
		if err != nil {
			return nil, err
		}
		return role, nil
	}
	
	return nil, nil
}
