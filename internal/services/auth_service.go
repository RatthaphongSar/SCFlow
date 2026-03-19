package services

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"scflow/internal/database"
	"scflow/internal/models"
)

var SecretKey []byte

func init() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "scflow_default_secret_change_in_prod"
		log.Println("[WARN] JWT_SECRET not set, using default (change in production!)")
	}
	SecretKey = []byte(secret)
}

// AuthClaims defines standard claims + custom roles
type AuthClaims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Login authenticates a user and returns a JWT token
func Login(username, password string) (string, error) {
	var user models.User
	if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("invalid credentials")
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	// Generate JWT
	claims := AuthClaims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "scflow-admin",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(SecretKey)
}

// CreateUser creates a new user with hashed password
func CreateUser(username, password, role string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := models.User{
		Username: username,
		Password: string(hashedPassword),
		Role:     role,
	}

	return database.DB.Create(&user).Error
}

// SeedAdmin creates default admin users if none exist
func SeedAdmin() {
	var count int64
	database.DB.Model(&models.User{}).Count(&count)
	if count == 0 {
		// Create first admin user
		user := models.User{
			Username: "admin",
			Role:     models.RoleAdmin,
		}
		hashed, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		user.Password = string(hashed)
		database.DB.Create(&user)

		// Create second admin user
		adminUser := models.User{
			Username: "superadmin",
			Role:     models.RoleAdmin,
		}
		hashedSuper, _ := bcrypt.GenerateFromPassword([]byte("superadmin123"), bcrypt.DefaultCost)
		adminUser.Password = string(hashedSuper)
		database.DB.Create(&adminUser)
	}

	// Seed Default Project
	var pCount int64
	database.DB.Model(&models.Project{}).Count(&pCount)
	if pCount == 0 {
		project := models.Project{
			Name:        "Internal Tools",
			Description: "Internal admin tools and scripts",
			Key:         "INT",
			CreatedAt:   time.Now(),
		}
		database.DB.Create(&project)
	}
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
