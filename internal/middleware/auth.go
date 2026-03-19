package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"scflow/internal/services"
)

func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get token from cookie
		tokenString := c.Cookies("jwt")

		// If no cookie, check Authorization header (Bearer token) for API access
		if tokenString == "" {
			authHeader := c.Get("Authorization")
			if len(authHeader) > 7 && strings.ToUpper(authHeader[:7]) == "BEARER " {
				tokenString = authHeader[7:]
			}
		}

		if tokenString == "" {
			return c.Redirect("/login")
		}

		token, err := jwt.ParseWithClaims(tokenString, &services.AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
			return services.SecretKey, nil
		})

		if err != nil || !token.Valid {
			return c.Redirect("/login")
		}

		claims, ok := token.Claims.(*services.AuthClaims)
		if !ok {
			return c.Redirect("/login")
		}

		// Set user context
		c.Locals("user_id", claims.UserID)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

func RoleCheck(allowedRoles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userRole, ok := c.Locals("role").(string)
		if !ok || userRole == "" {
			return c.Redirect("/login")
		}

		for _, role := range allowedRoles {
			if userRole == role {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).SendString("Access Denied: Insufficient Permissions")
	}
}
