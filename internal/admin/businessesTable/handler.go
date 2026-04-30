package businessesTable

import (
	"context"
	"net/http"
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
	businessTableIdURL = "/:id"
)

type handler struct {
	logger         *logging.Logger
	repository     Repository
	utilsRepository utils.Repository
	client           *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, utilsRepository utils.Repository, client *pgxpool.Pool) handlers.Handler {
	return &handler{
		logger:         logger,
		repository:     repository,
		utilsRepository: utilsRepository,
		client:           client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(businessTableIdURL, h.create)
	router.GET(businessTableIdURL, h.get)
	router.PUT(businessTableIdURL, h.update)
	router.DELETE(businessTableIdURL, h.delete)
}

func (h *handler) create(c *gin.Context) {
	var (
		businessTable BusinessTableReqDTO
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

	businessIdStr := c.Param("id")
	businessId, err := strconv.Atoi(businessIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&businessTable); err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Create(context.TODO(), businessId, businessTable)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, "success!!!")
}

func (h *handler) get(c *gin.Context) {
	businessIdStr := c.Param("id")
	limit := c.Query("limit")
	offset := c.Query("offset")
	businessId, err := strconv.Atoi(businessIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	resp, err := h.repository.GetTables(context.TODO(), businessId, limit, offset)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	tableIdStr := c.Param("id")
	tableId, err := strconv.Atoi(tableIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	var dto BusinessTableReqDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Update(context.TODO(), tableId, dto)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "success!!!")
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

	tableIdStr := c.Param("id")
	tableId, err := strconv.Atoi(tableIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), tableId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, "Successful!!!")
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