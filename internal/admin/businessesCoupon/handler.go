package businessesCoupon

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
	businessCouponIdURL = "/:id"
)

type handler struct {
	logger         *logging.Logger
	repository     Repository
	utilsRepository utils.Repository
	client         *pgxpool.Pool
}

func NewHandler(logger *logging.Logger, repository Repository, utilsRepository utils.Repository, client *pgxpool.Pool) handlers.Handler {
	return &handler{
		logger:         logger,
		repository:     repository,
		utilsRepository: utilsRepository,
		client:         client,
	}
}

func (h *handler) Register(router *gin.RouterGroup) {
	router.POST(businessCouponIdURL, h.create)
	router.GET(businessCouponIdURL, h.get)
	router.PATCH(businessCouponIdURL, h.update)
	router.DELETE(businessCouponIdURL, h.delete)
}

func (h *handler) create(c *gin.Context) {
	var businessCoupon BusinessCouponReqDTO

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

	if err := c.ShouldBindJSON(&businessCoupon); err != nil {
		fmt.Println("error binding JSON:", err)
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Create(context.TODO(), businessCoupon, businessId)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, "success!!!")
}

func (h *handler) get(c *gin.Context) {
	businessIdStr := c.Param("id")
	businessId, err := strconv.Atoi(businessIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	limit := c.Query("limit")
	offset := c.Query("offset")

	resp, err := h.repository.GetCoupons(context.TODO(), businessId, limit, offset)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *handler) update(c *gin.Context) {
	var coupon UpdateBusinessCouponReqDTO

	role, err := h.extractUserIdAndRole(c)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if *role != enum.RoleAdmin &&  *role != enum.RoleManager{
		appresult.HandleError(c, appresult.ErrForbidden)
		return
	}

	businessCouponIdStr := c.Param("id")
	businessCouponId, err := strconv.Atoi(businessCouponIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	if err := c.ShouldBindJSON(&coupon); err != nil {
		fmt.Println("error:", err)
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Update(context.TODO(), businessCouponId, coupon)
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

	businessCouponIdStr := c.Param("id")
	businessCouponId, err := strconv.Atoi(businessCouponIdStr)
	if err != nil {
		appresult.HandleError(c, err)
		return
	}

	err = h.repository.Delete(context.TODO(), businessCouponId)
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
		role, err := h.utilsRepository.UserRoleById(context.TODO(), userId)
		if err != nil {
			return nil, err
		}
		return role, nil
	}
	
	return nil, nil
}
