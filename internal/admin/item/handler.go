package item

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
	itemURL           = ""
	itemById          = "/:id"
	itemForUpdateById = "/forUpdate/:id"
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
	router.POST(itemURL, h.create)
	router.GET(itemById, h.getOne)
	router.GET(itemURL, h.getAll)
	router.PATCH(itemById, h.update)
	router.DELETE(itemById, h.delete)
	router.GET(itemForUpdateById, h.getForUpdate)
}

func (h *handler) create(c *gin.Context) {
	var item ItemReqDTO

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager &&  *role != enum.RoleEmployee{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	jsonData := c.PostForm("data")
	if err := json.Unmarshal([]byte(jsonData), &item); err != nil {
		appresult.HandleError(c, err)
		return
	}

	uploadDir := filepath.Join("uploads/item")
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		appresult.HandleError(c, err)
		return
	}

	image, err := c.FormFile("image")
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	imagePath, err := utils.SaveUploadedFile(c, image, uploadDir)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.Create(context.TODO(), item, imagePath, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *handler) getOne(c *gin.Context) {
	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetOne(context.TODO(), itemId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	search := c.Query("search")
	limit := c.Query("limit")
	offset := c.Query("offset")
	businessId := c.Query("businesses_id")
	itemCategoryId := c.Query("item_category_id")

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAll(context.TODO(), search, offset, limit, itemCategoryId, businessId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	var (
		imagePath string
		item      ItemReqDTO
	)

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager &&  *role != enum.RoleEmployee{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	jsonData := c.PostForm("data")
	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &item); err != nil {
			appresult.HandleError(c, err)
			return
		}
	}

	uploadDir := filepath.Join("uploads/item")
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

	resp, err := h.repository.Update(context.TODO(), itemId, item, imagePath, baseURL)
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

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager &&  *role != enum.RoleEmployee{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), itemId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "success!!!")
}

func (h *handler) getForUpdate(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager &&  *role != enum.RoleEmployee{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	id := c.Param("id")
	itemId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetForUpdate(context.TODO(), itemId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
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
