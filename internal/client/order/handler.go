package order

import (
	"context"
	"fmt"
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
	orderURL             = ""
	orderClientURL       = "/client"
	orderById            = "/:id"
	orderForBusinessesId = "/businesses/:id"
	orderForClientId     = "/client/:id"

	orderBusinessesWSURL = "/ws/orderBusinesses/:businesses_id"
	orderClientWSURL     = "/ws/orderClient"
	businessesWSURL      = "/ws/businesses/:id"
	clientWSURL          = "/ws/client"
	orderOneWSURL        = "/ws/:id"
	OneWSURL             = "/ws/orderOne/:id"
)

type handler struct {
	logger          *logging.Logger
	repository      Repository
	utilsRepository utils.Repository
	WSRepository    RepositoryWS
	client          *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, WSRepository RepositoryWS, utilsRepository utils.Repository, client *pgxpool.Pool,
) handlers.Handler {
	return &handler{
		logger:          logger,
		repository:      repository,
		utilsRepository: utilsRepository,
		WSRepository:    WSRepository,
		client:          client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(orderURL, h.create)
	router.GET(orderById, h.getOne)
	router.GET(orderClientURL, h.getAllForClient)
	router.GET(orderForBusinessesId, h.getAllForBusinesses)
	router.PUT(orderById, h.update)
	router.DELETE(orderById, h.delete)
	router.PUT(orderForClientId, h.updateStatusByClient)
	router.PUT(orderForBusinessesId, h.updateStatusByBusinesses)

	router.GET(businessesWSURL, h.checkBusinesses)
	router.GET(clientWSURL, h.checkClient)
	router.GET(orderBusinessesWSURL, h.wsHandlerBusinesses)
	router.GET(orderClientWSURL, h.wsHandlerClient)
	router.GET(OneWSURL, h.checkOrder)
	router.GET(orderOneWSURL, h.wsHandlerOrderOne)
}

func (h *handler) create(c *gin.Context) {
	var (
		req CreateOrderReq
	)
	clientID, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		fmt.Println("error: ", err)
		appresult.HandleError(c, err)
		return
	}

	orderId, err := h.repository.Create(context.TODO(), clientID, req)
	if err != nil {
		appresult.HandleError(c, err)
			return
	}

	NotifyOrderUpdate(req.BusinessesId, clientID, *orderId, h.repository)

	c.JSON(http.StatusCreated, "succsess!!!")
}

func (h *handler) getOne(c *gin.Context) {
	_, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	id := c.Param("id")
	orderId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)

	req, err := h.repository.GetOne(context.TODO(), orderId, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, req)
}

func (h *handler) getAllForClient(c *gin.Context) {
	clientID, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	limit := c.Query("limit")
	offset := c.Query("offset")
	status := c.Query("status")
	search := c.Query("search")
	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAllForClient(context.TODO(), clientID, limit, offset, status, search, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) getAllForBusinesses(c *gin.Context) {
	id := c.Param("id")
	businessesId, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin &&  *role != enum.RoleManager{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil && userId != -1{
		appresult.HandleError(c, err)
		return
	}

	limit := c.Query("limit")
	offset := c.Query("offset")
	status := c.Query("status")
	baseURL := c.MustGet("baseURL").(string)

	resp, err := h.repository.GetAllForBusinesses(context.TODO(), businessesId, userId, limit, offset, status, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	clientID, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	id := c.Param("id")
	orderID, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	var req UpdateOrderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		appresult.HandleError(c, err)
		return
	}

	baseURL := c.MustGet("baseURL").(string)
	resp, err := h.repository.Update(context.TODO(), clientID, orderID, req, baseURL)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyOrderUpdate(resp.BusinessesId, clientID, orderID, h.repository)

	c.JSON(http.StatusOK, resp)
}

func (h *handler) delete(c *gin.Context) {
	clientID, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	id := c.Param("id")
	orderID, err := strconv.Atoi(id)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	businessesId, clientId, err := h.repository.Delete(c.Request.Context(), clientID, orderID)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyOrderUpdate(businessesId, clientId, orderID, h.repository)

	c.JSON(http.StatusOK, "sucessfull!!!")
}

func (h *handler) updateStatusByClient(c *gin.Context) {
	clientID, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	var req UpdateOrderStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		appresult.HandleError(c, err)
		return
	}

	businessesId, clientId, err := h.repository.UpdateStatusByClient(c.Request.Context(), clientID, orderID, req)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyOrderUpdate(businessesId, clientId, orderID, h.repository)

	c.JSON(http.StatusOK, "sucessfull!!!")
}

func (h *handler) updateStatusByBusinesses(c *gin.Context) {
	var req UpdateOrderStatusReq

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	if *role != enum.RoleAdmin &&  *role != enum.RoleManager{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		appresult.HandleError(c, err)
		return
	}

	businessesId, clientId, err := h.repository.UpdateStatusByBusinesses(c.Request.Context(), userId, orderID, req)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	NotifyOrderUpdate(businessesId, clientId, orderID, h.repository)

	c.JSON(http.StatusOK, "sucessfull!!!")
}

func (h *handler) extractUserIdAndRole(c *gin.Context) (*string, error) {
	userId, err := utils.ExtractUserIdFromToken(c, h.client)
	if err != nil {
		return nil, err
	}
		role, err := h.utilsRepository.UserRoleById(context.TODO(), userId, nil)
		if err != nil {
			return nil, err
		}
		return role, nil
}
