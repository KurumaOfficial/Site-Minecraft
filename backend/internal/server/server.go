package server

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	timeoutmw "github.com/gofiber/fiber/v2/middleware/timeout"

	"holo-site-backend/internal/config"
	"holo-site-backend/internal/domain"
	"holo-site-backend/internal/httpapi"
	"holo-site-backend/internal/service"
)

func New(
	cfg config.Config,
	catalogService *service.CatalogService,
	promoService *service.PromoService,
	orderService *service.OrderService,
	adminService *service.AdminService,
	adminAuthService *service.AdminAuthService,
	metaService *service.MetaService,
	settingsService *service.SiteSettingsService,
	visitAnalyticsService *service.VisitAnalyticsService,
	integrationService *service.IntegrationService,
) (*fiber.App, error) {
	if err := ensureStaticFiles(cfg.StaticDir); err != nil {
		return nil, err
	}

	proxyHeader := ""
	if cfg.Security.EnableTrustedProxyCheck {
		proxyHeader = fiber.HeaderXForwardedFor
	}

	app := fiber.New(fiber.Config{
		AppName:                      cfg.AppName,
		ServerHeader:                 cfg.AppName,
		ReadTimeout:                  cfg.HTTP.ReadTimeout,
		WriteTimeout:                 cfg.HTTP.WriteTimeout,
		IdleTimeout:                  cfg.HTTP.IdleTimeout,
		BodyLimit:                    cfg.HTTP.BodyLimit,
		Concurrency:                  cfg.HTTP.Concurrency,
		ReadBufferSize:               cfg.HTTP.ReadBufferSize,
		WriteBufferSize:              cfg.HTTP.WriteBufferSize,
		StreamRequestBody:            true,
		DisablePreParseMultipartForm: true,
		ReduceMemoryUsage:            true,
		DisableStartupMessage:        cfg.HTTP.DisableStartupInfo,
		EnableTrustedProxyCheck:      cfg.Security.EnableTrustedProxyCheck,
		TrustedProxies:               cfg.Security.TrustedProxies,
		ProxyHeader:                  proxyHeader,
		ErrorHandler:                 newErrorHandler(),
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(helmet.New(helmet.Config{
		ContentSecurityPolicy: strings.Join([]string{
			"default-src 'self'",
			"base-uri 'self'",
			"object-src 'none'",
			"frame-ancestors 'self'",
			"script-src 'self' https://cdn.jsdelivr.net",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data: https:",
			"font-src 'self' data: https:",
			"connect-src 'self' https://cdn.jsdelivr.net https://*.supabase.co wss://*.supabase.co https://discord.com https://*.discord.com",
			"form-action 'self' https://*.supabase.co https://discord.com https://*.discord.com",
			"frame-src 'self' https://*.supabase.co https://discord.com https://*.discord.com",
		}, "; "),
		CrossOriginEmbedderPolicy: "unsafe-none",
		CrossOriginResourcePolicy: "same-site",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		XFrameOptions:             "DENY",
		HSTSMaxAge:                31536000,
		HSTSPreloadEnabled:        true,
		HSTSExcludeSubdomains:     false,
		PermissionPolicy:          "geolocation=(), camera=(), microphone=(), payment=(self)",
	}))
	app.Use(compress.New())
	app.Use(etag.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.Security.AllowOrigins,
		AllowMethods: strings.Join([]string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPatch, fiber.MethodHead, fiber.MethodOptions}, ","),
		AllowHeaders: strings.Join([]string{fiber.HeaderAccept, fiber.HeaderAuthorization, fiber.HeaderContentType, fiber.HeaderXRequestID}, ","),
		MaxAge:       300,
	}))

	if cfg.HTTP.EnableRequestLog {
		app.Use(logger.New(logger.Config{
			Format:        "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} | ${error}\n",
			TimeFormat:    "2006-01-02 15:04:05",
			DisableColors: true,
			Next: func(c *fiber.Ctx) bool {
				return c.Path() == "/api/v1/health" || strings.HasPrefix(c.Path(), "/assets/")
			},
		}))
	}

	catalogHandler := httpapi.NewCatalogHandler(catalogService)
	orderHandler := httpapi.NewOrderHandler(orderService)
	metaHandler := httpapi.NewMetaHandler(metaService)
	adminHandler := httpapi.NewAdminHandler(adminService, adminAuthService, catalogService, promoService, orderService, settingsService)
	analyticsHandler := httpapi.NewAnalyticsHandler(visitAnalyticsService)
	integrationHandler := httpapi.NewIntegrationHandler(integrationService, orderService)

	api := app.Group("/api/v1", timeoutmw.NewWithContext(func(c *fiber.Ctx) error {
		return c.Next()
	}, cfg.HTTP.RequestTimeout))
	api.Use(func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Next()
	})

	api.Use(limiter.New(limiter.Config{
		Max:        cfg.Security.APIRateLimitMax,
		Expiration: cfg.Security.APIRateLimitWindow,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/api/v1/health"
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":     "Слишком много запросов. Повторите позже.",
				"requestId": c.Get(fiber.HeaderXRequestID),
			})
		},
	}))

	orderLimiter := limiter.New(limiter.Config{
		Max:        cfg.Security.OrderRateLimitMax,
		Expiration: cfg.Security.OrderRateLimitWindow,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":     "Превышен лимит на оформление заказов. Повторите позже.",
				"requestId": c.Get(fiber.HeaderXRequestID),
			})
		},
	})

	adminLimiter := limiter.New(limiter.Config{
		Max:        cfg.Security.AdminRateLimitMax,
		Expiration: cfg.Security.AdminRateLimitWindow,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":     "Слишком много административных запросов. Повторите позже.",
				"requestId": c.Get(fiber.HeaderXRequestID),
			})
		},
	})

	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":           "ok",
			"timestamp":        time.Now().UTC(),
			"projectName":      cfg.Branding.ProjectName,
			"serverName":       cfg.Branding.ServerName,
			"storageMode":      cfg.StorageMode(),
			"databaseProvider": cfg.DatabaseProvider(),
		})
	})
	api.Get("/meta", metaHandler.Get)
	api.Get("/catalog", catalogHandler.List)
	api.Post("/analytics/visit", analyticsHandler.Track)
	api.Post("/orders/quote", orderLimiter, orderHandler.Quote)
	api.Post("/orders", orderLimiter, orderHandler.Create)
	api.Get("/orders/:id", orderHandler.Get)
	api.Post("/orders/:id/payment", orderLimiter, integrationHandler.InitPayment)
	api.Post("/payments/yookassa/webhook", integrationHandler.YooKassaWebhook)

	admin := api.Group("/admin", adminLimiter, adminHandler.RequireAdmin)
	admin.Get("/session", adminHandler.Session)
	admin.Get("/dashboard", adminHandler.Dashboard)
	admin.Get("/orders", orderHandler.List)
	admin.Patch("/orders/:id", orderHandler.Update)
	admin.Get("/catalog", adminHandler.ListCatalog)
	admin.Post("/catalog", adminHandler.SaveCatalog)
	admin.Get("/promos", adminHandler.ListPromos)
	admin.Post("/promos", adminHandler.SavePromo)
	admin.Get("/settings", adminHandler.GetSettings)
	admin.Post("/settings", adminHandler.SaveSettings)
	admin.Get("/integrations", integrationHandler.Get)
	admin.Post("/integrations", integrationHandler.SaveIntegrations)
	admin.Post("/integrations/test", integrationHandler.Test)
	admin.Post("/payments", integrationHandler.SavePayments)

	app.Static("/assets", filepath.Join(cfg.StaticDir, "assets"), fiber.Static{
		Compress:      false,
		ByteRange:     true,
		CacheDuration: cfg.HTTP.StaticCacheTTL,
		MaxAge:        cfg.HTTP.StaticMaxAge,
	})

	indexPath := filepath.Join(cfg.StaticDir, "index.html")
	offerPath := filepath.Join(cfg.StaticDir, "oferta.html")
	privacyPath := filepath.Join(cfg.StaticDir, "privacy.html")
	apiDocsPath := filepath.Join(cfg.StaticDir, "docs", "API.md")
	apiDocsHTMLPath := filepath.Join(cfg.StaticDir, "docs", "api-docs.html")
	app.Get("/admin/api-docs", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendFile(apiDocsHTMLPath)
	})
	app.Get("/api/v1/docs/api", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "public, max-age=300")
		c.Set(fiber.HeaderContentType, "text/markdown; charset=utf-8")
		return c.SendFile(apiDocsPath)
	})
	app.Get("/", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendFile(indexPath)
	})
	app.Get("/oferta", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendFile(offerPath)
	})
	app.Get("/privacy", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendFile(privacyPath)
	})
	app.Get("/oferta.html", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendFile(offerPath)
	})
	app.Get("/privacy.html", func(c *fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendFile(privacyPath)
	})

	app.Use(func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":     "Маршрут не найден.",
				"requestId": c.Get(fiber.HeaderXRequestID),
			})
		}

		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.SendFile(indexPath)
	})

	return app, nil
}

func ensureStaticFiles(staticDir string) error {
	requiredFiles := []string{
		filepath.Join(staticDir, "index.html"),
		filepath.Join(staticDir, "oferta.html"),
		filepath.Join(staticDir, "privacy.html"),
		filepath.Join(staticDir, "assets", "css", "styles.css"),
		filepath.Join(staticDir, "assets", "js", "app.js"),
		filepath.Join(staticDir, "assets", "js", "admin.js"),
	}

	for _, target := range requiredFiles {
		if _, err := os.Stat(target); err != nil {
			return err
		}
	}

	return nil
}

func newErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		status := fiber.StatusInternalServerError
		message := "Внутренняя ошибка сервера."

		var appError *domain.AppError
		if errors.As(err, &appError) {
			status = appError.Status
			message = appError.Message
		}

		var fiberError *fiber.Error
		if errors.As(err, &fiberError) {
			status = fiberError.Code
			if status < fiber.StatusInternalServerError {
				message = fiberError.Message
			}
		}

		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(status).JSON(fiber.Map{
				"error":     message,
				"requestId": c.Get(fiber.HeaderXRequestID),
			})
		}

		c.Set(fiber.HeaderContentType, fiber.MIMETextPlainCharsetUTF8)
		return c.Status(status).SendString(message)
	}
}
