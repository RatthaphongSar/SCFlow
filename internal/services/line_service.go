package services

import (
	"fmt"
	"log"
	"os"
	"strings"

	"scflow/internal/database"
	"scflow/internal/models"

	"github.com/line/line-bot-sdk-go/v7/linebot"
)

var Bot *linebot.Client

func InitLineBot() {
	var err error
	// In production, load from env
	secret := os.Getenv("LINE_CHANNEL_SECRET")
	token := os.Getenv("LINE_CHANNEL_TOKEN")

	if secret == "" || token == "" {
		log.Println("LINE Bot credentials missing, skipping initialization")
		return
	}

	Bot, err = linebot.New(secret, token)
	if err != nil {
		log.Fatal(err)
	}
}

// PushMessage sends a message to a user or group asynchronously
func PushMessage(to string, message string) {
	if Bot == nil {
		return
	}

	go func() {
		if _, err := Bot.PushMessage(to, linebot.NewTextMessage(message)).Do(); err != nil {
			log.Println("Line Push Error:", err)
		}
	}()
}

// HandleCommand processes chat commands
func HandleCommand(userID, text string) string {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}

	cmd := parts[0]
	switch cmd {
	case "/tasks":
		return handleListTasks()
	case "/status":
		if len(parts) < 2 {
			return "Usage: /status {task_id}"
		}
		return handleTaskStatus(parts[1])
	case "/deploy":
		if len(parts) < 3 || parts[1] != "done" {
			return "Usage: /deploy done {task_id}"
		}
		return handleDeployDone(parts[2], userID)
	default:
		return "Unknown command. Try /tasks, /status {id}, /deploy done {id}"
	}
}

func handleListTasks() string {
	var tasks []models.Task
	database.DB.Where("status != ?", models.TaskStatusDone).Limit(5).Find(&tasks)

	if len(tasks) == 0 {
		return "No active tasks."
	}

	msg := "Active Tasks:\n"
	for _, t := range tasks {
		idLabel := fmt.Sprintf("#%d", t.ID)
		if t.TaskCode != "" {
			idLabel = t.TaskCode
		}
		msg += fmt.Sprintf("%s %s [%s]\n", idLabel, t.Title, t.Status)
	}
	return msg
}

func handleTaskStatus(id string) string {
	var task models.Task
	if err := database.DB.First(&task, id).Error; err != nil {
		return "Task not found."
	}
	idLabel := fmt.Sprintf("#%d", task.ID)
	if task.TaskCode != "" {
		idLabel = task.TaskCode
	}
	return fmt.Sprintf("Task %s: %s\nStatus: %s\nPriority: %s", idLabel, task.Title, task.Status, task.Priority)
}

func handleDeployDone(id string, userID string) string {
	// Simple permission check: Assume if they can chat, they can deploy (refine later)
	var task models.Task
	if err := database.DB.First(&task, id).Error; err != nil {
		return "Task not found."
	}

	task.Status = models.TaskStatusDone
	database.DB.Save(&task)

	idLabel := fmt.Sprintf("#%d", task.ID)
	if task.TaskCode != "" {
		idLabel = task.TaskCode
	}
	return fmt.Sprintf("Task %s marked as DONE by user %s", idLabel, userID)
}

// NotifyTaskCreated sends a notification to a default group
func NotifyTaskCreated(task *models.Task) {
	if Bot == nil {
		return
	}

	groupID := os.Getenv("LINE_GROUP_ID")
	idLabel := fmt.Sprintf("#%d", task.ID)
	if task.TaskCode != "" {
		idLabel = task.TaskCode
	}
	msg := fmt.Sprintf("New Task Created\n\nID: %s\nTitle: %s\nStatus: %s\nPriority: %s",
		idLabel, task.Title, task.Status, task.Priority)

	if task.Assignee != nil {
		msg += fmt.Sprintf("\nAssigned To: %s", task.Assignee.Username)
	}

	if groupID != "" {
		PushMessage(groupID, msg)
	} else {
		log.Println("LINE Notify (No Group ID):", msg)
	}
}

// NotifyTaskStatusChange has been moved to notification_service.go to consolidate notification logic
