package main

import (
	"bufio"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"

	"scflow/internal/database"
	"scflow/internal/handlers"
	"scflow/internal/middleware"
	"scflow/internal/models"
	"scflow/internal/services"
)

// loadEnvFile reads a .env file and sets environment variables
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Println("[INFO] No .env file found, using system environment variables")
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	log.Println("[INFO] Loaded .env file")
}

func main() {
	// Load .env file
	loadEnvFile(".env")

	// Initialize Database
	database.Connect()
	
	// Initialize Services
	services.SeedAdmin()
	services.InitLineBot()

	// Initialize View Engine
	engine := html.New("./views", ".html")
	engine.AddFunc("toLower", func(s string) string {
		return strings.ToLower(s)
	})
	engine.AddFunc("add", func(a, b int) int {
		return a + b
	})
	engine.AddFunc("sub", func(a, b int) int {
		return a - b
	})
	engine.AddFunc("categoryKey", func(s string) string {
		s = strings.ToLower(s)
		s = strings.ReplaceAll(s, " ", "_")
		return s
	})

	// Create Fiber App
	app := fiber.New(fiber.Config{
		Views:       engine,
		AppName:     "SCFlow Internal Admin",
		BodyLimit:   50 * 1024 * 1024, // 50MB limit for uploads
		ReadTimeout: 120 * time.Second,
		WriteTimeout: 120 * time.Second,
	})

	// Global Middleware
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed, // Optimize for speed
	}))

	// Static Files
	app.Static("/", "./public")

	// Public Routes
	// Rate Limiting for Login: 5 requests per 1 minute
	loginLimiter := limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).SendString("Too many login attempts. Please try again later.")
		},
	})

	app.Get("/login", handlers.LoginPage)
	app.Post("/login", loginLimiter, handlers.LoginPost)
	app.Get("/logout", handlers.Logout)
	
	// Rate Limiting for Webhook: 60 requests per 1 minute
	webhookLimiter := limiter.New(limiter.Config{
		Max:        60,
		Expiration: 1 * time.Minute,
	})
	app.Post("/webhook/line", webhookLimiter, handlers.LineWebhook)

	// Protected Routes
	api := app.Group("/", middleware.Protected())

	api.Get("/", handlers.Dashboard)
	
	// Task Routes
	api.Get("/tasks", handlers.GetTasks)
	api.Get("/tasks/new", handlers.GetTaskForm)
	api.Get("/tasks/search", handlers.GetTasks) 
	api.Get("/tasks/filter", handlers.GetTasks)
	api.Get("/tasks/:id", handlers.GetTaskDetails)
	api.Post("/tasks", handlers.CreateTask)
	api.Post("/tasks/:id/status", handlers.UpdateTaskStatus)
	api.Post("/tasks/:id/assign", handlers.UpdateTaskAssignee)
	api.Post("/tasks/:id/logs", handlers.CreateTaskLog)
	api.Delete("/tasks/:id", handlers.DeleteTask)
	api.Post("/upload", handlers.UploadFile)

	// Calendar Routes
	api.Get("/calendar", handlers.GetCalendarPage)
	api.Get("/calendar/events", handlers.GetCalendarEvents)

	// Project Routes
	api.Get("/projects", handlers.GetProjects)
	api.Post("/projects", handlers.CreateProject)
	api.Delete("/projects/:id", handlers.DeleteProject)

	// User Routes (Master Only - simplified for now, or check in handler)
	// Ideally should be in RoleCheck middleware
	users := api.Group("/users", middleware.RoleCheck(models.RoleMaster))
	users.Get("/", handlers.GetUsers)
	users.Post("/", handlers.CreateUser)
	users.Delete("/:id", handlers.DeleteUser)

	// SQL Routes (Master Only)
	sql := api.Group("/sql", middleware.RoleCheck(models.RoleMaster))
	sql.Get("/", handlers.GetSQLScripts)
	sql.Post("/", handlers.CreateSQLScript)
	sql.Get("/:id/content", handlers.GetSQLScript)
	sql.Get("/:id/details", handlers.GetSQLScriptDetails)
	sql.Delete("/:id", handlers.DeleteSQLScript)

	// Knowledge Base Routes (All authenticated users can access)
	kb := api.Group("/knowledge", middleware.RoleCheck(models.RoleMaster, models.RoleProjectAdmin, models.RoleMember, models.RoleViewer))
	kb.Get("/", handlers.GetKnowledgeBase)
	kb.Post("/", handlers.CreateKnowledge)
	kb.Get("/:id/content", handlers.GetKnowledgeContent)
	kb.Delete("/:id", handlers.DeleteKnowledge)

	// Log Routes (Master and ProjectAdmin only)
	logs := api.Group("/logs", middleware.RoleCheck(models.RoleMaster, models.RoleProjectAdmin))
	logs.Get("/", handlers.GetLogs)
	logs.Post("/analyze", handlers.AnalyzeLogFile)

	// SQL Routes (continued)
	sql.Post("/run/custom", handlers.RunCustomSQL)
	sql.Post("/run/:id", handlers.RunSQLScript)

	// Start Background Services
	services.StartDeadlineChecker()

	// Determine Port
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// Graceful Shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("[INFO] Shutting down server...")
		if err := app.Shutdown(); err != nil {
			log.Println("[ERROR] Server shutdown error:", err)
		}
	}()

	// Start Server
	log.Printf("[INFO] SCFlow starting on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

