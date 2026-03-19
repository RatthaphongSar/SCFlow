package database

import (
	"fmt"
	"log"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"scflow/internal/models"
)

var DB *gorm.DB

func Connect() {
	var err error

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "scflow.db"
	}

	// Use SQLite for low resource usage as requested.
	DB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}

	log.Println("Database connected successfully")

	// Auto Migrate
	log.Println("Running migrations...")
	err = DB.AutoMigrate(
		&models.User{},
		&models.Project{},
		&models.Task{},
		&models.TaskLog{},
		&models.Knowledge{},
		&models.SQLScript{},
		&models.OperationLog{},
	)

	if err != nil {
		log.Fatal("Failed to migrate database: ", err)
	}
	log.Println("Database migrated successfully")

	DB.Model(&models.Task{}).Where("status = ?", "Dev").Update("status", models.TaskStatusCorrect)

	var tasks []models.Task
	DB.Unscoped().Order("created_at asc").Find(&tasks)
	counters := map[int]int{}
	for _, task := range tasks {
		year := task.CreatedAt.Year() % 100
		counters[year]++
		if task.TaskCode == "" {
			code := fmt.Sprintf("%02d/%03d", year, counters[year])
			DB.Model(&models.Task{}).Where("id = ?", task.ID).Update("task_code", code)
		}
	}
}
