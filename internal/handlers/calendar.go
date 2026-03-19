package handlers

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"scflow/internal/database"
	"scflow/internal/models"
)

// GetCalendarPage renders the calendar view
func GetCalendarPage(c *fiber.Ctx) error {
	return c.Render("pages/calendar", fiber.Map{
		"Title":    "Project Timeline & Calendar",
		"Username": getUsername(c),
	}, "layouts/main")
}

// CalendarEvent represents an event for FullCalendar
type CalendarEvent struct {
	ID            uint                   `json:"id"`
	Title         string                 `json:"title"`
	Start         string                 `json:"start"`
	End           string                 `json:"end,omitempty"`
	Color         string                 `json:"color,omitempty"`
	Url           string                 `json:"url,omitempty"`
	Description   string                 `json:"description,omitempty"`
	AllDay        bool                   `json:"allDay"`
	ExtendedProps map[string]interface{} `json:"extendedProps"`
}

// GetCalendarEvents returns tasks as JSON events for FullCalendar
func GetCalendarEvents(c *fiber.Ctx) error {
	start := c.Query("start")
	end := c.Query("end")
	filter := c.Query("filter", "all") // "all" or "my"
	
	// Safely get user ID
	userIDVal := c.Locals("user_id")
	var userID uint
	if userIDVal != nil {
		if id, ok := userIDVal.(uint); ok {
			userID = id
		} else if id, ok := userIDVal.(int); ok {
			userID = uint(id)
		} else if id, ok := userIDVal.(float64); ok {
			userID = uint(id)
		}
	}

	var tasks []models.Task
	query := database.DB.Model(&models.Task{}).Preload("Assignee").Preload("Project")

	// Filter by date range (if provided)
	if start != "" && end != "" {
		// Simple logic: return tasks that have any date overlap with the window
		// or were created in the window if no dates set.
		query = query.Where(
			"(start_date >= ? AND start_date < ?) OR (due_date >= ? AND due_date < ?) OR (created_at >= ? AND created_at < ?)", 
			start, end, start, end, start, end,
		)
	}

	// Filter by user
	if filter == "my" {
		query = query.Where("assignee_id = ?", userID)
	}

	query.Find(&tasks)

	events := make([]CalendarEvent, 0)
	for _, task := range tasks {
		var eventStart, eventEnd string
		var color string
		
		// Determine Color based on Status
		switch task.Status {
		case models.TaskStatusPlanning:
			color = "#6c757d" // Grey
	case models.TaskStatusCorrect:
			color = "#007bff" // Blue
		case models.TaskStatusTest:
			color = "#ffc107" // Yellow
		case models.TaskStatusReady:
			color = "#17a2b8" // Cyan
		case models.TaskStatusDeploy:
			color = "#6610f2" // Purple
		case models.TaskStatusDone:
			color = "#28a745" // Green
		default:
			color = "#343a40"
		}

		// Determine Start/End
		if task.StartDate != nil {
			eventStart = task.StartDate.Format(time.RFC3339)
		} else {
			eventStart = task.CreatedAt.Format(time.RFC3339)
		}

		if task.DueDate != nil {
			eventEnd = task.DueDate.Format(time.RFC3339)
		}
		
		assigneeName := "Unassigned"
		if task.Assignee != nil {
			assigneeName = task.Assignee.Username
		}

		events = append(events, CalendarEvent{
			ID:    task.ID,
			Title: fmt.Sprintf("[%s] %s", task.Project.Key, task.Title),
			Start: eventStart,
			End:   eventEnd,
			Color: color,
			ExtendedProps: map[string]interface{}{
				"status":   task.Status,
				"assignee": assigneeName,
				"priority": task.Priority,
			},
		})
	}

	return c.JSON(events)
}
