package handlermanager

import (
	"restaurants/internal/admin/sms_socket"
	"restaurants/internal/appresult"
	"restaurants/pkg/sms_sender"

	"restaurants/internal/admin/auth"
	authdb "restaurants/internal/admin/auth/db"

	province "restaurants/internal/admin/province"
	provincedb "restaurants/internal/admin/province/db"

	subcategory "restaurants/internal/admin/subcategory"
	subcategorydb "restaurants/internal/admin/subcategory/db"

	businesses "restaurants/internal/admin/businesses"
	businessesdb "restaurants/internal/admin/businesses/db"

	user "restaurants/internal/admin/user"
	userdb "restaurants/internal/admin/user/db"

	types "restaurants/internal/admin/types"
	typesdb "restaurants/internal/admin/types/db"

	itemCategory "restaurants/internal/admin/itemCategory"
	itemCategorydb "restaurants/internal/admin/itemCategory/db"

	item "restaurants/internal/admin/item"
	itemdb "restaurants/internal/admin/item/db"

	userClient "restaurants/internal/client/user"
	userClientdb "restaurants/internal/client/user/db"

	review "restaurants/internal/client/review"
	reviewdb "restaurants/internal/client/review/db"

	searchHistory "restaurants/internal/client/searchHistory"
	searchHistorydb "restaurants/internal/client/searchHistory/db"

	businessesTable "restaurants/internal/admin/businessesTable"
	businessesTabledb "restaurants/internal/admin/businessesTable/db"

	businessesCoupon "restaurants/internal/admin/businessesCoupon"
	businessesCoupondb "restaurants/internal/admin/businessesCoupon/db"

	clientCoupon "restaurants/internal/admin/clientCoupon"
	clientCoupondb "restaurants/internal/admin/clientCoupon/db"

	reservation "restaurants/internal/client/reservation"
	reservationdb "restaurants/internal/client/reservation/db"
	reservationdbWS "restaurants/internal/client/reservation/dbWS"

	notification "restaurants/internal/client/notification"
	notificationdb "restaurants/internal/client/notification/db"

	deviceToken "restaurants/internal/client/deviceToken"
	deviceTokendb "restaurants/internal/client/deviceToken/db"

	basket "restaurants/internal/client/basket"
	basketdb "restaurants/internal/client/basket/db"

	orderClient "restaurants/internal/client/order"
	orderClientdb "restaurants/internal/client/order/db"
	orderClientWS "restaurants/internal/client/order/dbWS"

	category "restaurants/internal/admin/category"
	categorydb "restaurants/internal/admin/category/db"

	"restaurants/pkg/fcm"
	utilsdb "restaurants/pkg/utils/db"

	"restaurants/pkg/logging"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v4/pgxpool"
	_ "github.com/swaggo/swag"
)

const (
	authURL             = "/api/v1/auth"
	provinceURL         = "/api/v1/province"
	subcategoryURL      = "/api/v1/subcategory"
	businessesURL       = "/api/v1/businesses"
	userURL             = "/api/v1/user"
	smsSocketURL        = "/api/v1/sms_socket"
	typeURL             = "/api/v1/type"
	itemCategoryURL     = "/api/v1/itemCategory"
	itemURL             = "/api/v1/item"
	businessesTableURL  = "/api/v1/businessesTable"
	businessesCouponURL = "/api/v1/businessesCoupon"
	orderAdminURL       = "/api/v1/admin/order"
	categoryURL         = "/api/v1/category"

	reviewURL        = "/api/v1/review"
	searchHistoryURL = "/api/v1/searchHistory"
	reservationURL   = "/api/v1/reservation"
	clientCouponURL  = "/api/v1/clientCoupon"
	notificationURL  = "/api/v1/notification"
	deviceTokenURL   = "/api/v1/deviceToken"
	basketURL        = "/api/v1/basket"
	orderClientURL   = "/api/v1/order"
	organizationURL   = "/api/v1/organization"
)

func Manager(client *pgxpool.Pool, logger *logging.Logger, smsSender *sms_sender.Client, fcmClient *fcm.FCMClient) *gin.Engine {
	router := gin.Default()

	router.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "http://localhost:3000" || origin == "https://prechemical-bibi-pentomic.ngrok-free.dev" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, Authorization, accept, origin, Cache-Control, X-Requested-With, ngrok-skip-browser-warning")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	router.Use(appresult.BaseURLMiddleware())

	utilRepository := utilsdb.NewRepository(client, logger)

	authRouterManager := router.Group(authURL)
	authRepository := authdb.NewRepository(client, logger)
	authRouterHandler := auth.NewHandler(logger, authRepository)
	authRouterHandler.Register(authRouterManager)

	provinceRouterManager := router.Group(provinceURL)
	provinceRepository := provincedb.NewRepository(client, logger)
	provinceRouterHandler := province.NewHandler(logger, provinceRepository, utilRepository, client)
	provinceRouterHandler.Register(provinceRouterManager)

	subcategoryRouterManager := router.Group(subcategoryURL)
	subcategoryRepository := subcategorydb.NewRepository(client, logger)
	subcategoryRouterHandler := subcategory.NewHandler(logger, subcategoryRepository, utilRepository, client)
	subcategoryRouterHandler.Register(subcategoryRouterManager)

	userRouterManager := router.Group(userURL)
	userRepository := userdb.NewRepository(client, logger)
	userRouterHandler := user.NewHandler(logger, userRepository, utilRepository, client)
	userRouterHandler.Register(userRouterManager)

	smsSocketRouter := router.Group(smsSocketURL)
	smsSocket := sms_socket.NewHandler(logger, smsSender)
	smsSocket.Register(smsSocketRouter)

	typeRouterManager := router.Group(typeURL)
	typeRepository := typesdb.NewRepository(client, logger)
	typeRouterHandler := types.NewHandler(logger, typeRepository, utilRepository, client)
	typeRouterHandler.Register(typeRouterManager)

	itemCategoryRouterManager := router.Group(itemCategoryURL)
	itemCategoryRepository := itemCategorydb.NewRepository(client, logger)
	itemCategoryRouterHandler := itemCategory.NewHandler(logger, itemCategoryRepository, utilRepository, client)
	itemCategoryRouterHandler.Register(itemCategoryRouterManager)

	itemRouterManager := router.Group(itemURL)
	itemRepository := itemdb.NewRepository(client, logger)
	itemRouterHandler := item.NewHandler(logger, itemRepository, utilRepository, client)
	itemRouterHandler.Register(itemRouterManager)

	reviewRouterManager := router.Group(reviewURL)
	reviewRepository := reviewdb.NewRepository(client, logger)
	reviewRouterHandler := review.NewHandler(logger, reviewRepository, utilRepository, client)
	reviewRouterHandler.Register(reviewRouterManager)

	businessesRouterManager := router.Group(businessesURL)
	businessesRepository := businessesdb.NewRepository(client, logger)
	businessesRouterHandler := businesses.NewHandler(logger, businessesRepository, itemRepository, typeRepository, reviewRepository, utilRepository, client)
	businessesRouterHandler.Register(businessesRouterManager)

	userClientRouterManager := router.Group(userURL)
	userClientRepository := userClientdb.NewRepository(client, logger)
	userClientRouterHandler := userClient.NewHandler(client, logger, userClientRepository, utilRepository, smsSender)
	userClientRouterHandler.Register(userClientRouterManager)

	searchHistoryRouterManager := router.Group(searchHistoryURL)
	searchHistoryRepository := searchHistorydb.NewRepository(client, logger)
	searchHistoryRouterHandler := searchHistory.NewHandler(logger, searchHistoryRepository, utilRepository, client)
	searchHistoryRouterHandler.Register(searchHistoryRouterManager)

	businessesTableManager := router.Group(businessesTableURL)
	businessesTableRepository := businessesTabledb.NewRepository(client, logger)
	businessesTableRouterHandler := businessesTable.NewHandler(logger, businessesTableRepository, utilRepository, client)
	businessesTableRouterHandler.Register(businessesTableManager)

	businessesCouponManager := router.Group(businessesCouponURL)
	businessesCouponRepository := businessesCoupondb.NewRepository(client, logger)
	businessesCouponRouterHandler := businessesCoupon.NewHandler(logger, businessesCouponRepository, utilRepository, client)
	businessesCouponRouterHandler.Register(businessesCouponManager)

	clientCouponManager := router.Group(clientCouponURL)
	clientCouponRepository := clientCoupondb.NewRepository(client, logger)
	clientCouponRouterHandler := clientCoupon.NewHandler(logger, clientCouponRepository, utilRepository, client)
	clientCouponRouterHandler.Register(clientCouponManager)

	reservationManager := router.Group(reservationURL)
	reservationRepository := reservationdb.NewRepository(client, logger)
	reservationRepositoryWS := reservationdbWS.NewRepository(client, logger)
	reservationRouterHandler := reservation.NewHandler(logger, reservationRepository, reservationRepositoryWS, utilRepository, fcmClient, client)
	reservationRouterHandler.Register(reservationManager)

	notificationManager := router.Group(notificationURL)
	notificationRepository := notificationdb.NewRepository(client, logger)
	notificationRouterHandler := notification.NewHandler(logger, notificationRepository, fcmClient, utilRepository, client)
	notificationRouterHandler.Register(notificationManager)

	deviceTokenManager := router.Group(deviceTokenURL)
	deviceTokenRepository := deviceTokendb.NewRepository(client, logger)
	deviceTokenRouterHandler := deviceToken.NewHandler(logger, deviceTokenRepository)
	deviceTokenRouterHandler.Register(deviceTokenManager)

	basketManager := router.Group(basketURL)
	basketRepository := basketdb.NewRepository(client, logger)
	basketHandler := basket.NewHandler(logger, basketRepository, utilRepository, client)
	basketHandler.Register(basketManager)

	orderManagerClient := router.Group(orderClientURL)
	orderRepositoryClient := orderClientdb.NewRepository(client, logger, basketRepository)
	orderRepositoryWS := orderClientWS.NewRepository(client, logger)
	orderHandlerClient := orderClient.NewHandler(logger, orderRepositoryClient, orderRepositoryWS, utilRepository, client)
	orderHandlerClient.Register(orderManagerClient)

	categoryManagerClient := router.Group(categoryURL)
	categoryRepositoryClient := categorydb.NewRepository(client, logger)
	categoryHandlerClient := category.NewHandler(logger, categoryRepositoryClient, utilRepository, client)
	categoryHandlerClient.Register(categoryManagerClient)

	return router
}
