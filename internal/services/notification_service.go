package services

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"scflow/internal/database"
	"scflow/internal/models"
)

// StartDeadlineChecker starts a background goroutine to check for approaching deadlines
func StartDeadlineChecker() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	go func() {
		for range ticker.C {
			checkDeadlines()
		}
	}()
}

func checkDeadlines() {
	var tasks []models.Task
	now := time.Now()
	// Check tasks due within the next 24 hours that are not Done
	// And have not been notified yet (need a flag or just check last notification time if we had one)
	// For simplicity, we'll just log/notify for tasks due in < 24 hours and < 1 hour.
	// To avoid spamming, we might need a "LastNotifiedAt" field or similar logic.
	// Let's implement a simple rule: Notify if DueDate is between Now and Now+15min (Approaching)

	windowStart := now
	windowEnd := now.Add(15 * time.Minute)

	// Find tasks due in the next 15 minutes
	database.DB.Preload("Assignee").Where("due_date BETWEEN ? AND ? AND status != ?", windowStart, windowEnd, models.TaskStatusDone).Find(&tasks)

	for _, task := range tasks {
		// Log Notification
		log.Printf("[DEADLINE WARNING] Task #%d '%s' is due at %s", task.ID, task.Title, task.DueDate.Format(time.RFC3339))

		// Send Line Notification if Assignee has LineID
		if task.Assignee != nil && task.Assignee.LineID != "" {
			idLabel := fmt.Sprintf("#%d", task.ID)
			if task.TaskCode != "" {
				idLabel = task.TaskCode
			}
			msg := fmt.Sprintf("Deadline Approaching\nTask %s: %s\nDue: %s\nPlease update status or complete.",
				idLabel, task.Title, task.DueDate.Format("15:04"))
			PushMessage(task.Assignee.LineID, msg)
		} else {
			// Broadcast to group or default channel if configured
			// PushMessage("GROUP_ID", fmt.Sprintf("Task #%d is due soon! (No assignee or LineID)", task.ID))
		}
	}
	cleanupDoneTasks()
}

// NotifyTaskStatusChange sends a notification when a task status changes
func NotifyTaskStatusChange(task *models.Task, oldStatus string) {
	// Simple log for now
	log.Printf("[STATUS CHANGE] Task #%d '%s': %s -> %s", task.ID, task.Title, oldStatus, task.Status)

	if task.Assignee != nil && task.Assignee.LineID != "" {
		idLabel := fmt.Sprintf("#%d", task.ID)
		if task.TaskCode != "" {
			idLabel = task.TaskCode
		}
		msg := fmt.Sprintf("Status Update\nTask %s: %s\n%s -> %s",
			idLabel, task.Title, oldStatus, task.Status)
		PushMessage(task.Assignee.LineID, msg)
	}
}

func cleanupDoneTasks() {
	daysStr := strings.TrimSpace(os.Getenv("DONE_TASK_DELETE_DAYS"))
	if daysStr == "" {
		return
	}
	days, err := strconv.Atoi(daysStr)
	if err != nil || days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	database.DB.Where("status = ? AND updated_at < ?", models.TaskStatusDone, cutoff).Delete(&models.Task{})
}
