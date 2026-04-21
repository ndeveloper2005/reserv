package searchHistory

import (
	"context"
	"fmt"
	"net/http"
	"restaurants/internal/appresult"
	"restaurants/internal/handlers"
	"restaurants/pkg/logging"
	"restaurants/pkg/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	searchHistoryURL  = ""
	searchHistoryById = "/:id"
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
	router.POST(searchHistoryURL, h.create)
	router.GET(searchHistoryURL, h.getAll)
	router.DELETE(searchHistoryById, h.delete)
}

func (h *handler) create(c *gin.Context) {
	var (
		search SearchHistoryReq
	)
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&search); err != nil {
		fmt.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.Create(context.TODO(), search, clientId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAll(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil{
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.GetAll(context.TODO(), clientId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil{
		appresult.HandleError(c, err)
		return
	}

	id := c.Param("id")
	searchHistoryId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), searchHistoryId, clientId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "Successful!!!")
}
