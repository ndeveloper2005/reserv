package clientCoupon

import (
	"context"
	"errors"
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
	clientCouponByIdURL         = "/:id"
	clientCouponByClientURL     = "/byClient"
	clientCouponByBusinessIdURL = "/byBusinesses/:id"
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
	router.POST(clientCouponByIdURL, h.create)
	router.GET(clientCouponByClientURL, h.getForClient)
	router.GET(clientCouponByBusinessIdURL, h.getForBusiness)
	router.DELETE(clientCouponByIdURL, h.delete)
}

func (h *handler) create(c *gin.Context) {
	var clientCoupon AssignCouponReqDTO
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
	businessId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&clientCoupon); err != nil {
		appresult.HandleError(c, err)
		return
	}

	_, err = h.repository.Create(context.TODO(), businessId, clientCoupon)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, "success!!!")
}

func (h *handler) getForClient(c *gin.Context) {
	clientId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
	}

	limit := c.Query("limit")
	offset := c.Query("offset")
	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetCouponsForClient(context.TODO(), clientId, limit, offset, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) getForBusiness(c *gin.Context) {
	businessIdStr := c.Param("id")
	businessId, err := strconv.Atoi(businessIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	limit := c.Query("limit")
	offset := c.Query("offset")

	resp, err := h.repository.GetCouponsForBusiness(context.TODO(), businessId, limit, offset)
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
	assignId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), assignId)
	if err != nil {
		if errors.Is(err, appresult.ErrNotFoundBase) {
			c.JSON(http.StatusBadRequest, err)
			return
		}
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
