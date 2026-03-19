package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"scflow/internal/database"
	"scflow/internal/models"
)

var DangerousKeywords = []string{
	"DROP", "DELETE", "TRUNCATE", "ALTER", "GRANT", "REVOKE", "CREATE", "INSERT", "UPDATE",
}

type SQLResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	Duration time.Duration   `json:"duration"`
}

// ValidateQuery checks for dangerous keywords
func ValidateQuery(query string, isMaster bool) error {
	upperQuery := strings.ToUpper(query)
	
	// Master can run anything except maybe system tables? 
	// The prompt says: "Only MASTER role can run queries" 
	// AND "Block dangerous keywords: DROP DELETE TRUNCATE ALTER"
	// This implies even MASTER cannot run these unless there is an override.
	// But usually, Admin tools allow INSERT/UPDATE. 
	// The prompt says: "Block DROP/DELETE by default".
	
	for _, keyword := range DangerousKeywords {
		if strings.Contains(upperQuery, keyword) {
			// Allow INSERT/UPDATE if explicitly needed, but prompt says "Block DROP/DELETE".
			// Let's block structural changes and destructive actions.
			if keyword == "INSERT" || keyword == "UPDATE" {
				continue // Allow data manipulation
			}
			return fmt.Errorf("dangerous keyword detected: %s", keyword)
		}
	}
	return nil
}

// ExecuteSafeQuery runs a query with timeout and read-only transaction if needed
func ExecuteSafeQuery(query string, userID uint, isMaster bool, readOnly bool) (*SQLResult, error) {
	if !isMaster {
		return nil, errors.New("unauthorized: only MASTER can execute SQL")
	}

	if err := ValidateQuery(query, isMaster); err != nil {
		return nil, err
	}

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	
	// Start transaction
	tx := database.DB.WithContext(ctx).Begin()
	defer func() {
		// Always rollback if read-only or error
		if readOnly {
			tx.Rollback()
		} else {
			// Commit if write mode and no error (handled below)
		}
	}()

	// Execute
	rows, err := tx.Raw(query).Rows()
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	defer rows.Close()

	// Parse result
	columns, _ := rows.Columns()
	var resultRows [][]interface{}
	
	// Limit to 500 rows
	rowCount := 0
	for rows.Next() {
		if rowCount >= 500 {
			break
		}
		
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		
		rows.Scan(valuePtrs...)
		
		// Copy values to avoid pointer issues
		rowCopy := make([]interface{}, len(columns))
		copy(rowCopy, values)
		resultRows = append(resultRows, rowCopy)
		rowCount++
	}

	// Commit if not read-only
	if !readOnly {
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}
	}

	duration := time.Since(start)

	// Log execution
	logEntry := models.OperationLog{
		UserID:    userID,
		Action:    "SQL_EXEC",
		Target:    fmt.Sprintf("Query (ReadOnly=%v): %s", readOnly, query),
		CreatedAt: time.Now(),
	}
	database.DB.Create(&logEntry)

	return &SQLResult{
		Columns: columns,
		Rows:    resultRows,
		Duration: duration,
	}, nil
}
