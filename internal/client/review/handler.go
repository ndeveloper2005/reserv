package review

import (
	"context"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/internal/handlers"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	reviewURL     = ""
	reviewByIdURL = "/:id"
)

type handler struct {
	logger         *logging.Logger
	repository     Repository
	utilRepository utils.Repository
	client           *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, utilRepository utils.Repository, client *pgxpool.Pool) handlers.Handler {
	return &handler{
		logger:         logger,
		repository:     repository,
		utilRepository: utilRepository,
		client:           client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(reviewURL, h.create)
	router.GET(reviewByIdURL, h.get)
	router.GET(reviewURL, h.getAll)
	router.DELETE(reviewByIdURL, h.delete)
}

func (h *handler) create(c *gin.Context) {
	var (
		review ReviewReqDTO
	)
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&review); err != nil {
		appresult.HandleError(c, err)
		return
	}

	actualLength := utf8.RuneCountInString(review.Comment)
	if actualLength > 720 {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.Create(context.TODO(), review, clientId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) get(c *gin.Context) {
	id := c.Param("id")
	reviewId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	resp, err := h.repository.GetClient(context.TODO(), reviewId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	var filter ReviewFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.GetAll(context.TODO(), filter)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	id := c.Param("id")
	reviewId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), reviewId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "Successful!!!")
}
