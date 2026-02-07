package transport

import (
	"context"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/zhunismp/imagep-backend/services/image-apis/config"
)

type HttpServer struct {
	cfg    config.Config
	app    *fiber.App
	router fiber.Router
}

func NewHttpServer(cfg config.Config) *HttpServer {

	app := fiber.New(fiber.Config{
		AppName:   cfg.Name,
		BodyLimit: 50 * 1024 * 1024,
	})

	// middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "${time} ${status} - ${method} ${path}\n",
		TimeFormat: "2006/01/02 15:04:05",
		TimeZone:   "Asia/Bangkok",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		AllowHeaders:     "Origin,X-PINGOTHER,Accept,Authorization,Content-Type,X-CSRF-Token",
		ExposeHeaders:    "Link",
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// health check route
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"ping": "pong"})
	})

	// base api prefix
	apiGroup := app.Group(cfg.BaseApiPrefix)

	return &HttpServer{
		cfg:    cfg,
		app:    app,
		router: apiGroup,
	}
}

func (s *HttpServer) SetupRoute(processingHandler *ProcessingHandler) {
	s.router.Post("/upload", processingHandler.Upload)
	s.router.Post("/upload/:taskId", processingHandler.Upload)
	s.router.Get("/process/:taskId", processingHandler.Process)
	s.router.Get("/downloads/:taskId", processingHandler.Download)
	s.router.Delete("/delete/:taskId/:img", processingHandler.Delete)
}

func (s *HttpServer) Start() {
	serverAddr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	go func() {
		if err := s.app.Listen(serverAddr); err != nil && err.Error() != "http: Server closed" {
			log.Printf("Failed to start HTTP server: %v", err)
		}
	}()

}

func (s *HttpServer) Shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
}
