package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"holo-site-backend/internal/config"
	"holo-site-backend/internal/database"
	"holo-site-backend/internal/repository"
	"holo-site-backend/internal/server"
	"holo-site-backend/internal/service"
)

type closer interface {
	Close() error
}

type storeBundle struct {
	catalog  repository.CatalogRepository
	promos   repository.PromoRepository
	orders   repository.OrderRepository
	settings repository.SiteSettingsRepository
	visits   repository.VisitAnalyticsRepository
}

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	stores, postgresPool, fileCloser, err := buildStores(cfg)
	if err != nil {
		log.Fatalf("repository error: %v", err)
	}
	if postgresPool != nil {
		defer postgresPool.Close()
	}
	if fileCloser != nil {
		defer func() {
			if err := fileCloser.Close(); err != nil {
				log.Printf("repository close error: %v", err)
			}
		}()
	}

	bootstrapCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	catalogService, err := service.NewCatalogService(bootstrapCtx, stores.catalog)
	if err != nil {
		log.Fatalf("catalog bootstrap error: %v", err)
	}

	promoService, err := service.NewPromoService(bootstrapCtx, stores.promos, cfg.PromoCodes)
	if err != nil {
		log.Fatalf("promo bootstrap error: %v", err)
	}

	settingsService, err := service.NewSiteSettingsService(bootstrapCtx, cfg, stores.settings)
	if err != nil {
		log.Fatalf("settings bootstrap error: %v", err)
	}
	visitAnalyticsService, err := service.NewVisitAnalyticsService(bootstrapCtx, stores.visits)
	if err != nil {
		log.Fatalf("visit analytics bootstrap error: %v", err)
	}

	orderService := service.NewOrderService(catalogService, promoService, stores.orders)
	adminAuthService := service.NewAdminAuthService(cfg)
	adminService := service.NewAdminService(catalogService, promoService, orderService, settingsService, visitAnalyticsService)
	metaService := service.NewMetaService(cfg, catalogService, settingsService)

	app, err := server.New(cfg, catalogService, promoService, orderService, adminService, adminAuthService, metaService, settingsService, visitAnalyticsService)
	if err != nil {
		log.Fatalf("server build error: %v", err)
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("backend started: %s (%s)", cfg.Branding.SiteURL, cfg.Address())
		log.Printf("storage mode: %s (%s)", cfg.StorageMode(), cfg.DatabaseProvider())
		if err := app.Listen(cfg.Address()); err != nil {
			serverErrors <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("server stopped with error: %v", err)
	case <-stop:
		log.Println("shutdown signal received")
		if err := app.ShutdownWithTimeout(cfg.HTTP.ShutdownTimeout); err != nil {
			log.Fatalf("shutdown error: %v", err)
		}
	}
}

func buildStores(cfg config.Config) (storeBundle, *pgxpool.Pool, closer, error) {
	settingsRepo, err := repository.NewFileSiteSettingsRepository(cfg.DataDir)
	if err != nil {
		return storeBundle{}, nil, nil, err
	}
	visitRepo, err := repository.NewFileVisitAnalyticsRepository(cfg.DataDir)
	if err != nil {
		return storeBundle{}, nil, nil, err
	}

	if !cfg.DatabaseEnabled() && !cfg.SupabaseRESTEnabled() {
		catalogRepo, err := repository.NewFileCatalogRepository(cfg.DataDir)
		if err != nil {
			return storeBundle{}, nil, nil, err
		}

		promoRepo, err := repository.NewFilePromoRepository(cfg.DataDir)
		if err != nil {
			return storeBundle{}, nil, nil, err
		}

		orderRepo, err := repository.NewFileOrderRepository(cfg.DataDir, cfg.Storage.FileSnapshotEvery)
		if err != nil {
			return storeBundle{}, nil, nil, err
		}

		return storeBundle{
			catalog:  catalogRepo,
			promos:   promoRepo,
			orders:   orderRepo,
			settings: settingsRepo,
			visits:   visitRepo,
		}, nil, orderRepo, nil
	}

	if cfg.SupabaseRESTEnabled() {
		client, err := repository.NewSupabaseRESTClient(cfg.Supabase)
		if err != nil {
			return storeBundle{}, nil, nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := client.Probe(ctx); err != nil {
			return storeBundle{}, nil, nil, err
		}

		return storeBundle{
			catalog:  repository.NewSupabaseCatalogRepository(client),
			promos:   repository.NewSupabasePromoRepository(client),
			orders:   repository.NewSupabaseOrderRepository(client),
			settings: settingsRepo,
			visits:   visitRepo,
		}, nil, nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		return storeBundle{}, nil, nil, err
	}

	if cfg.Database.AutoMigrate {
		if err := database.RunSchema(ctx, pool, cfg.Database.SchemaPath); err != nil {
			pool.Close()
			return storeBundle{}, nil, nil, err
		}
	}

	return storeBundle{
		catalog:  repository.NewPostgresCatalogRepository(pool),
		promos:   repository.NewPostgresPromoRepository(pool),
		orders:   repository.NewPostgresOrderRepository(pool),
		settings: settingsRepo,
		visits:   visitRepo,
	}, pool, nil, nil
}
