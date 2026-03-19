package handlers

import (
	"bytes"
	"fmt"
	"html"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/line/line-bot-sdk-go/v7/linebot"

	"scflow/internal/database"
	"scflow/internal/log_analyzer"
	"scflow/internal/models"
	"scflow/internal/services"
)

// getUsername retrieves the logged-in user's username from context
func getUsername(c *fiber.Ctx) string {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return ""
	}
	var user models.User
	if err := database.DB.Select("username").First(&user, userID).Error; err != nil {
		return ""
	}
	return user.Username
}

// getUserID retrieves the logged-in user's ID from context
func getUserID(c *fiber.Ctx) uint {
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		return 0
	}
	return userID
}

// stringToUint converts string to uint
func stringToUint(s string) uint {
	var id uint
	fmt.Sscanf(s, "%d", &id)
	return id
}

// getUserRole retrieves the logged-in user's role from context
func getUserRole(c *fiber.Ctx) string {
	role, ok := c.Locals("role").(string)
	if !ok {
		return models.RoleMember
	}
	return role
}

// getBaseMap returns the base fiber.Map with common variables
func getBaseMap(c *fiber.Ctx) fiber.Map {
	return fiber.Map{
		"Username": getUsername(c),
		"UserRole": getUserRole(c),
	}
}

// --- Dashboard ---

func Dashboard(c *fiber.Ctx) error {
	var tasks []models.Task
	var logs []models.OperationLog

	// Get recent tasks
	database.DB.Limit(5).Order("created_at desc").Preload("Assignee").Find(&tasks)

	// Get recent logs
	database.DB.Limit(10).Order("created_at desc").Preload("User").Find(&logs)

	// Calculate Stats
	var totalTasks int64
	var pendingTasks int64
	var completedTasks int64
	var totalProjects int64

	database.DB.Model(&models.Task{}).Count(&totalTasks)
	database.DB.Model(&models.Task{}).Where("status IN ?", []string{models.TaskStatusPlanning, models.TaskStatusCorrect, models.TaskStatusTest, models.TaskStatusReady}).Count(&pendingTasks)
	database.DB.Model(&models.Task{}).Where("status = ?", "Done").Count(&completedTasks) // Assuming Done is today/recent or all time? Let's just do all time for now
	database.DB.Model(&models.Project{}).Count(&totalProjects)

	return c.Render("pages/index", fiber.Map{
		"Tasks":          tasks,
		"Logs":           logs,
		"User":           c.Locals("user_id"),
		"Role":           c.Locals("role"),
		"Username":       getUsername(c),
		"TotalTasks":     totalTasks,
		"PendingTasks":   pendingTasks,
		"CompletedTasks": completedTasks,
		"TotalProjects":  totalProjects,
	}, "layouts/main")
}

// --- Tasks ---

func GetTasks(c *fiber.Ctx) error {
	var tasks []models.Task
	var users []models.User

	// Fetch Users for Filter
	database.DB.Find(&users)

	filterAssignee := c.Query("filter_assignee")
	searchQuery := strings.TrimSpace(c.Query("q"))

	query := database.DB.Preload("Assignee").Preload("CreatedBy").Preload("Parent").Preload("Subtasks").Preload("Project").Model(&models.Task{})

	if filterAssignee != "" {
		query = query.Where("assignee_id = ?", filterAssignee)
	}
	if searchQuery != "" {
		pattern := "%" + searchQuery + "%"
		query = query.Where("task_code LIKE ? OR title LIKE ? OR description LIKE ? OR tags LIKE ?", pattern, pattern, pattern, pattern)
	}

	query.Order("created_at desc").Find(&tasks)

	// Group tasks by Status for Kanban
	columns := []struct {
		Status string
		Title  string
		Color  string
		Tasks  []models.Task
	}{
		{"Planning", "Planning", "#6c757d", []models.Task{}},
		{"Correct", "Correct", "#007bff", []models.Task{}},
		{"Test", "Test", "#ffc107", []models.Task{}},
		{"Ready", "Ready", "#17a2b8", []models.Task{}},
		{"Deploy", "Deploy", "#6610f2", []models.Task{}},
		{"Done", "Done", "#28a745", []models.Task{}},
	}

	for _, task := range tasks {
		for i := range columns {
			if columns[i].Status == task.Status {
				columns[i].Tasks = append(columns[i].Tasks, task)
				break
			}
		}
	}

	type StatusNode struct {
		Status string
		Count  int
		Tasks  []models.Task
	}
	type ServiceNode struct {
		Name      string
		Count     int
		Statuses  []StatusNode
		statusMap map[string]*StatusNode
	}
	type StageNode struct {
		Name       string
		Count      int
		Services   []ServiceNode
		serviceMap map[string]*ServiceNode
	}
	type UserNode struct {
		ID       uint
		Name     string
		Count    int
		Stages   []StageNode
		stageMap map[string]*StageNode
	}

	userMap := map[uint]*UserNode{}

	for _, task := range tasks {
		userID := uint(0)
		userName := "Unassigned"
		if task.Assignee != nil {
			userID = task.Assignee.ID
			userName = task.Assignee.Username
		}

		userNode, ok := userMap[userID]
		if !ok {
			userNode = &UserNode{
				ID:       userID,
				Name:     userName,
				stageMap: map[string]*StageNode{},
			}
			userMap[userID] = userNode
		}

		stageName := "Main"
		if task.ParentID != nil {
			stageName = "Sub"
		}
		stageNode, ok := userNode.stageMap[stageName]
		if !ok {
			stageNode = &StageNode{
				Name:       stageName,
				serviceMap: map[string]*ServiceNode{},
			}
			userNode.stageMap[stageName] = stageNode
		}

		serviceName := "Unassigned Service"
		if task.Project.Name != "" {
			serviceName = task.Project.Name
		}
		serviceNode, ok := stageNode.serviceMap[serviceName]
		if !ok {
			serviceNode = &ServiceNode{
				Name:      serviceName,
				statusMap: map[string]*StatusNode{},
			}
			stageNode.serviceMap[serviceName] = serviceNode
		}

		statusName := task.Status
		if statusName == "" {
			statusName = models.TaskStatusPlanning
		}
		statusNode, ok := serviceNode.statusMap[statusName]
		if !ok {
			statusNode = &StatusNode{
				Status: statusName,
			}
			serviceNode.statusMap[statusName] = statusNode
		}

		statusNode.Tasks = append(statusNode.Tasks, task)
		statusNode.Count++
		serviceNode.Count++
		stageNode.Count++
		userNode.Count++
	}

	statusOrder := []string{
		models.TaskStatusPlanning,
		models.TaskStatusCorrect,
		models.TaskStatusTest,
		models.TaskStatusReady,
		models.TaskStatusDeploy,
		models.TaskStatusDone,
	}
	statusIndex := map[string]int{}
	for i, s := range statusOrder {
		statusIndex[s] = i
	}

	var userTree []UserNode
	for _, node := range userMap {
		for _, stageName := range []string{"Main", "Sub"} {
			stageNode, ok := node.stageMap[stageName]
			if !ok {
				continue
			}

			var serviceNames []string
			for name := range stageNode.serviceMap {
				serviceNames = append(serviceNames, name)
			}
			sort.Strings(serviceNames)

			var services []ServiceNode
			for _, name := range serviceNames {
				serviceNode := stageNode.serviceMap[name]
				var statusNames []string
				for s := range serviceNode.statusMap {
					statusNames = append(statusNames, s)
				}
				sort.Slice(statusNames, func(i, j int) bool {
					iIdx, iOk := statusIndex[statusNames[i]]
					jIdx, jOk := statusIndex[statusNames[j]]
					if iOk && jOk {
						return iIdx < jIdx
					}
					if iOk {
						return true
					}
					if jOk {
						return false
					}
					return statusNames[i] < statusNames[j]
				})

				var statuses []StatusNode
				for _, s := range statusNames {
					statuses = append(statuses, *serviceNode.statusMap[s])
				}
				serviceNode.Statuses = statuses
				services = append(services, *serviceNode)
			}
			stageNode.Services = services
			node.Stages = append(node.Stages, *stageNode)
		}
		userTree = append(userTree, *node)
	}

	sort.Slice(userTree, func(i, j int) bool {
		return strings.ToLower(userTree[i].Name) < strings.ToLower(userTree[j].Name)
	})

	// If HTMX request for filter update, render just the board content?
	// But our board structure is inside the main page template loop.
	// For simplicity, we re-render the whole page or part.
	// Ideally we should extract Kanban board to a partial if we want partial updates.
	// But let's just render the full page for now or check HX-Target.

	if c.Get("HX-Request") == "true" && c.Get("HX-Target") == "kanban-board" {
		return c.Render("partials/kanban_board", fiber.Map{
			"Columns": columns,
		})
	}

	baseMap := getBaseMap(c)
	baseMap["Columns"] = columns
	baseMap["UserTree"] = userTree
	baseMap["Users"] = users
	baseMap["Query"] = searchQuery
	baseMap["FilterAssignee"] = func() uint {
		if filterAssignee == "" {
			return 0
		}
		var id uint
		fmt.Sscanf(filterAssignee, "%d", &id)
		return id
	}()
	return c.Render("pages/tasks", baseMap, "layouts/main")
}

func GetTaskDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	return renderTaskDetail(c, id)
}

func UpdateTaskStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	newStatus := c.FormValue("status")
	if !models.IsValidTaskStatus(newStatus) {
		return c.Status(400).SendString("Invalid status")
	}

	var task models.Task
	if err := database.DB.Preload("Assignee").First(&task, id).Error; err != nil {
		return c.Status(404).SendString("Task not found")
	}

	oldStatus := task.Status
	task.Status = newStatus
	database.DB.Save(&task)

	// Notify LINE
	services.NotifyTaskStatusChange(&task, oldStatus)

	// Record Log
	var userID uint
	if uid, ok := c.Locals("user_id").(uint); ok {
		userID = uid
	} else {
		userID = 1 // Default to admin if auth fails or testing
	}

	log := models.TaskLog{
		TaskID:    task.ID,
		UserID:    userID,
		Action:    "StatusChange",
		Detail:    fmt.Sprintf("Changed status from %s to %s", oldStatus, newStatus),
		CreatedAt: time.Now(),
	}
	database.DB.Create(&log)

	return c.SendString(newStatus)
}

func UpdateTaskAssignee(c *fiber.Ctx) error {
	id := c.Params("id")
	var task models.Task
	if err := database.DB.First(&task, id).Error; err != nil {
		return c.Status(404).SendString("Task not found")
	}

	assigneeValue := strings.TrimSpace(c.FormValue("assignee_id"))
	var newAssigneeID *uint
	if assigneeValue != "" {
		var parsed uint
		fmt.Sscanf(assigneeValue, "%d", &parsed)
		if parsed != 0 {
			newAssigneeID = &parsed
		}
	}

	oldAssigneeID := task.AssigneeID
	if oldAssigneeID != nil {
		if newAssigneeID == nil || *newAssigneeID != *oldAssigneeID {
			return c.Status(400).SendString("Assignee cannot be changed")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
	if newAssigneeID == nil {
		return c.SendStatus(fiber.StatusNoContent)
	}
	task.AssigneeID = newAssigneeID
	database.DB.Save(&task)

	oldName := "Unassigned"
	if oldAssigneeID != nil {
		var oldUser models.User
		if err := database.DB.First(&oldUser, *oldAssigneeID).Error; err == nil {
			oldName = oldUser.Username
		}
	}

	newName := "Unassigned"
	if newAssigneeID != nil {
		var newUser models.User
		if err := database.DB.First(&newUser, *newAssigneeID).Error; err == nil {
			newName = newUser.Username
		}
	}

	var userID uint
	if uid, ok := c.Locals("user_id").(uint); ok {
		userID = uid
	} else {
		userID = 1
	}

	log := models.TaskLog{
		TaskID:    task.ID,
		UserID:    userID,
		Action:    "Assign",
		Detail:    fmt.Sprintf("Changed assignee from %s to %s", oldName, newName),
		CreatedAt: time.Now(),
	}
	database.DB.Create(&log)

	if c.Get("HX-Request") == "true" {
		c.Set("HX-Redirect", "/tasks")
		return c.SendStatus(fiber.StatusNoContent)
	}
	return c.Redirect("/tasks")
}

func CreateTaskLog(c *fiber.Ctx) error {
	id := c.Params("id")
	var task models.Task
	if err := database.DB.First(&task, id).Error; err != nil {
		return c.Status(404).SendString("Task not found")
	}

	message := strings.TrimSpace(c.FormValue("message"))
	link := strings.TrimSpace(c.FormValue("link"))
	action := strings.TrimSpace(c.FormValue("action"))

	var attachmentName string
	var attachmentPath string
	file, err := c.FormFile("attachment")
	if err == nil && file != nil {
		originalName := strings.ReplaceAll(filepath.Base(file.Filename), " ", "_")
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), originalName)
		savePath := fmt.Sprintf("./public/uploads/%s", filename)
		if err := c.SaveFile(file, savePath); err == nil {
			attachmentName = file.Filename
			attachmentPath = "/uploads/" + filename
		}
	}

	if message == "" && link == "" && attachmentPath == "" {
		return c.Status(400).SendString("Empty log")
	}
	if action == "" {
		action = "Progress"
	}

	var userID uint
	if uid, ok := c.Locals("user_id").(uint); ok {
		userID = uid
	} else {
		userID = 1
	}

	log := models.TaskLog{
		TaskID:         task.ID,
		UserID:         userID,
		Action:         action,
		Detail:         message,
		Link:           link,
		AttachmentName: attachmentName,
		AttachmentPath: attachmentPath,
		CreatedAt:      time.Now(),
	}
	database.DB.Create(&log)

	return renderTaskDetail(c, id)
}

func renderTaskDetail(c *fiber.Ctx, id string) error {
	var task models.Task
	if err := database.DB.Preload("Project").Preload("Assignee").Preload("CreatedBy").Preload("Parent").Preload("Subtasks").First(&task, id).Error; err != nil {
		return c.Status(404).SendString("Task not found")
	}

	var logs []models.TaskLog
	database.DB.Preload("User").Where("task_id = ?", task.ID).Order("created_at desc").Find(&logs)

	type TaskLogView struct {
		models.TaskLog
		IsImage bool
	}
	var logViews []TaskLogView
	for _, logItem := range logs {
		ext := strings.ToLower(filepath.Ext(logItem.AttachmentPath))
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp"
		logViews = append(logViews, TaskLogView{
			TaskLog: logItem,
			IsImage: isImage,
		})
	}

	return c.Render("partials/task_detail", fiber.Map{
		"Task":     task,
		"TaskLogs": logViews,
	})
}

func GetTaskForm(c *fiber.Ctx) error {
	var users []models.User
	database.DB.Find(&users)

	parentID := c.Query("parent_id")

	return c.Render("partials/task_form", fiber.Map{
		"Users":    users,
		"ParentID": parentID,
	})
}

func CreateTask(c *fiber.Ctx) error {
	task := new(models.Task)
	if err := c.BodyParser(task); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	// Parse Dates
	startDateStr := c.FormValue("start_date")
	dueDateStr := c.FormValue("due_date")

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02T15:04", startDateStr); err == nil {
			task.StartDate = &t
		}
	}
	if dueDateStr != "" {
		if t, err := time.Parse("2006-01-02T15:04", dueDateStr); err == nil {
			task.DueDate = &t
		}
	}

	// Handle File Upload if present
	file, err := c.FormFile("attachment")
	if err == nil {
		c.SaveFile(file, fmt.Sprintf("./public/uploads/%s", file.Filename))
	}

	if task.Status == "" {
		task.Status = models.TaskStatusPlanning
	} else if !models.IsValidTaskStatus(task.Status) {
		return c.Status(400).SendString("Invalid status")
	}
	task.CreatedAt = time.Now()
	if task.TaskCode == "" {
		year := task.CreatedAt.Year()
		yearStr := fmt.Sprintf("%04d", year)
		var count int64
		database.DB.Unscoped().Model(&models.Task{}).Where("strftime('%Y', created_at) = ?", yearStr).Count(&count)
		task.TaskCode = fmt.Sprintf("%02d/%03d", year%100, count+1)
	}

	if task.ProjectID == 0 {
		task.ProjectID = 1
	}

	// Get Current User ID from context
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		userID = 1 // Default to admin for testing
		ok = true
	}

	if ok {
		task.CreatedByID = userID

		// If Assignee is not set, default to creator?
		// Actually, let's keep it nil or set if form provided it.
		// If form provided "assignee_id", BodyParser should have set it.
		// If not, we can default to creator if that's the desired behavior.
		// User requirement: "Assign to User other". So we should respect the form.
		// if task.AssigneeID == nil {
		// 	task.AssigneeID = &userID // Default to self if not specified
		// }
	}

	// Handle Parent ID for Subtasks (Spinning)
	parentIDStr := c.FormValue("parent_id")
	if parentIDStr != "" {
		var pID uint
		fmt.Sscanf(parentIDStr, "%d", &pID)
		task.ParentID = &pID
	}

	if result := database.DB.Create(&task); result.Error != nil {
		return c.Status(500).SendString(result.Error.Error())
	}

	// Notify LINE
	// Reload task to get associations
	database.DB.Preload("Assignee").Preload("CreatedBy").First(task, task.ID)
	services.NotifyTaskCreated(task)

	return c.Redirect("/tasks")
}

func DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")
	if result := database.DB.Delete(&models.Task{}, id); result.Error != nil {
		return c.Status(500).SendString("Failed to delete task")
	}
	// Return empty string or partial update if using hx-swap="outerHTML" to remove element
	// If hx-target is the row, returning empty string removes it.
	return c.SendString("")
}

// --- Projects ---

func GetProjects(c *fiber.Ctx) error {
	var projects []models.Project
	database.DB.Order("created_at desc").Find(&projects)

	// Fetch Upcoming Tasks (Due within next 7 days)
	var upcomingTasks []models.Task
	nextWeek := time.Now().Add(7 * 24 * time.Hour)
	database.DB.Where("due_date BETWEEN ? AND ? AND status != ?", time.Now(), nextWeek, models.TaskStatusDone).Order("due_date asc").Limit(5).Find(&upcomingTasks)

	// Fetch Timeline Tasks (Tasks with StartDate and DueDate)
	var timelineTasks []models.Task
	// Fetch tasks active in the next 30 days window for timeline
	timelineStart := time.Now().Add(-2 * 24 * time.Hour) // Show a bit of past
	timelineEnd := time.Now().Add(30 * 24 * time.Hour)
	database.DB.Where("start_date IS NOT NULL AND due_date IS NOT NULL AND start_date <= ? AND due_date >= ?", timelineEnd, timelineStart).Order("start_date asc").Limit(20).Find(&timelineTasks)

	// Calculate timeline visualization data
	type TimelineTask struct {
		models.Task
		Offset int
		Width  int
	}
	var visualTasks []TimelineTask

	totalDays := 32 // 2 days past + 30 days future
	dayWidth := 30  // pixels per day (approx)

	for _, t := range timelineTasks {
		// Calculate offset from timelineStart
		start := *t.StartDate
		if start.Before(timelineStart) {
			start = timelineStart
		}

		daysFromStart := int(start.Sub(timelineStart).Hours() / 24)
		if daysFromStart < 0 {
			daysFromStart = 0
		}

		duration := int(t.DueDate.Sub(start).Hours() / 24)
		if duration < 1 {
			duration = 1
		}

		// Clip duration if extends beyond view
		if daysFromStart+duration > totalDays {
			duration = totalDays - daysFromStart
		}

		visualTasks = append(visualTasks, TimelineTask{
			Task:   t,
			Offset: daysFromStart * dayWidth,
			Width:  duration * dayWidth,
		})
	}

	// Generate Date Headers
	var timelineDates []time.Time
	for i := 0; i < totalDays; i += 5 { // Show every 5th day
		timelineDates = append(timelineDates, timelineStart.Add(time.Duration(i)*24*time.Hour))
	}

	return c.Render("pages/projects", fiber.Map{
		"Projects":      projects,
		"UpcomingTasks": upcomingTasks,
		"TimelineTasks": visualTasks,
		"TimelineDates": timelineDates,
		"Username":      getUsername(c),
	}, "layouts/main")
}

func CreateProject(c *fiber.Ctx) error {
	project := new(models.Project)
	if err := c.BodyParser(project); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	if result := database.DB.Create(&project); result.Error != nil {
		return c.Status(500).SendString("Failed to create project")
	}

	return c.Redirect("/projects")
}

func DeleteProject(c *fiber.Ctx) error {
	id := c.Params("id")
	if result := database.DB.Delete(&models.Project{}, id); result.Error != nil {
		return c.Status(500).SendString("Failed to delete project")
	}
	return c.SendString("")
}

func GetProjectDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	var project models.Project
	if result := database.DB.Preload("Attachments").Find(&project, id); result.Error != nil {
		return c.Status(404).SendString("Project not found")
	}

	// Get tasks for this project
	var tasks []models.Task
	database.DB.Where("project_id = ?", id).Order("status, created_at desc").Find(&tasks)

	attachmentsHTML := getProjectAttachmentsHTML(project.Attachments)

	return c.SendString(fmt.Sprintf(`
		<div style="position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 9999;" onclick="if(event.target===this)this.innerHTML=''">
			<div style="background: var(--bg-color); padding: 20px; border-radius: 8px; max-width: 600px; max-height: 80vh; overflow-y: auto; width: 90%%; border: 1px solid var(--border-color);">
				<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
					<h2 style="margin: 0;">%s #%d</h2>
					<button onclick="if(this.closest('[onclick]'))this.closest('[onclick]').innerHTML=''" style="background: none; border: none; color: var(--text-muted); font-size: 1.5em; cursor: pointer;">✕</button>
				</div>

				<div style="margin-bottom: 15px;">
					<label style="display: block; margin-bottom: 5px; color: var(--text-muted);">Project Key</label>
					<div style="padding: 8px; background: var(--input-bg); border-radius: 4px;">%s</div>
				</div>

				<div style="margin-bottom: 15px;">
					<label style="display: block; margin-bottom: 5px; color: var(--text-muted);">Status</label>
					<div style="padding: 8px; background: var(--input-bg); border-radius: 4px;">
						<span class="badge" style="background-color: %s;">%s</span>
					</div>
				</div>

				<div style="margin-bottom: 15px;">
					<label style="display: block; margin-bottom: 5px; color: var(--text-muted);">Description</label>
					<div style="padding: 8px; background: var(--input-bg); border-radius: 4px;">%s</div>
				</div>

				<div style="margin-bottom: 15px;">
					<label style="display: block; margin-bottom: 5px; color: var(--text-muted);">Tasks</label>
					<div style="padding: 8px; background: var(--input-bg); border-radius: 4px;">%d tasks</div>
				</div>

				<div style="margin-bottom: 20px;">
					<label style="display: block; margin-bottom: 5px; color: var(--text-muted);">File Attachments</label>
					<div style="max-height: 200px; overflow-y: auto; background: var(--input-bg); border-radius: 4px; padding: 8px; margin-bottom: 8px;">
						%s
					</div>
					<form hx-post="/projects/%d/upload" hx-encoding="multipart/form-data" hx-swap="none" style="display: flex; gap: 8px;">
						<input type="file" name="file" required style="flex: 1; padding: 6px; background: var(--input-bg); border: 1px solid var(--border-color); border-radius: 4px;">
						<button type="submit" class="btn btn-primary" style="padding: 6px 12px;">Upload</button>
					</form>
				</div>

				<button onclick="if(this.closest('[onclick]'))this.closest('[onclick]').innerHTML=''" class="btn" style="width: 100%%; padding: 8px;">Close</button>
			</div>
		</div>
	`,
		html.EscapeString(project.Name),
		project.ID,
		html.EscapeString(project.Key),
		getStatusColor(project.Status),
		html.EscapeString(project.Status),
		html.EscapeString(project.Description),
		len(tasks),
		attachmentsHTML,
		project.ID,
	))
}

func getStatusColor(status string) string {
	switch status {
	case "Active":
		return "var(--success-color)"
	case "Paused":
		return "var(--warning-color)"
	case "Completed":
		return "var(--text-muted)"
	default:
		return "var(--primary-color)"
	}
}

func getProjectAttachmentsHTML(attachments []models.ProjectFile) string {
	if len(attachments) == 0 {
		return `<div style="color: var(--text-muted); font-size: 0.9em;">No attachments</div>`
	}
	html := ""
	for _, file := range attachments {
		var uploader string
		var uploaderUser models.User
		if err := database.DB.First(&uploaderUser, file.UploadedBy).Error; err != nil {
			uploader = "Unknown"
		} else {
			uploader = uploaderUser.Username
		}
		html += fmt.Sprintf(`
			<div style="padding: 8px; border-bottom: 1px solid var(--border-color); display: flex; justify-content: space-between;">
				<div>
					<div style="font-weight: 500;">%s</div>
					<div style="font-size: 0.8em; color: var(--text-muted);">By %s - %s</div>
				</div>
				<button hx-delete="/projects/files/%d" hx-swap="none" class="btn btn-sm" style="color: var(--danger-color);">Delete</button>
			</div>
		`, file.FileName, uploader, file.CreatedAt.Format("2006-01-02"), file.ID)
	}
	return html
}

func UpdateProjectStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	status := c.FormValue("status")

	// Validate status
	if status != "Active" && status != "Paused" && status != "Completed" {
		return c.Status(400).SendString("Invalid status")
	}

	if result := database.DB.Model(&models.Project{}).Where("id = ?", id).Update("status", status); result.Error != nil {
		return c.Status(500).SendString("Failed to update project status")
	}

	return c.SendString("")
}

func UploadProjectFile(c *fiber.Ctx) error {
	id := c.Params("id")
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).SendString("File upload failed")
	}

	// Save file
	fileName := fmt.Sprintf("project_%s_%d_%s", id, time.Now().Unix(), file.Filename)
	filePath := fmt.Sprintf("./public/uploads/%s", fileName)

	if err := c.SaveFile(file, filePath); err != nil {
		return c.Status(500).SendString("Failed to save file")
	}

	// Create database record
	projectFile := models.ProjectFile{
		ProjectID:  stringToUint(id),
		FileName:   file.Filename,
		FilePath:   filePath,
		FileSize:   file.Size,
		UploadedBy: getUserID(c),
	}

	if result := database.DB.Create(&projectFile); result.Error != nil {
		return c.Status(500).SendString("Failed to save file record")
	}

	return c.SendString("")
}

// --- Users (Master Only) ---

func GetUsers(c *fiber.Ctx) error {
	var users []models.User
	database.DB.Find(&users)

	baseMap := getBaseMap(c)
	baseMap["Users"] = users
	return c.Render("pages/users", baseMap, "layouts/main")
}

func CreateUser(c *fiber.Ctx) error {
	user := new(models.User)
	if err := c.BodyParser(user); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	// Hash Password
	hashed, err := services.HashPassword(user.Password)
	if err != nil {
		return c.Status(500).SendString("Failed to hash password")
	}
	user.Password = hashed

	if result := database.DB.Create(&user); result.Error != nil {
		return c.Status(500).SendString("Failed to create user")
	}

	return c.Redirect("/users")
}

func DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")
	if result := database.DB.Delete(&models.User{}, id); result.Error != nil {
		return c.Status(500).SendString("Failed to delete user")
	}
	return c.SendString("")
}

// --- Logs ---

func GetLogs(c *fiber.Ctx) error {
	// Analytics Data
	type LogStat struct {
		Type  string
		Count int64
	}
	var opStats []LogStat
	database.DB.Model(&models.OperationLog{}).Select("action as type, count(*) as count").Group("action").Scan(&opStats)

	type TaskStat struct {
		Type  string
		Count int64
	}
	var taskStats []TaskStat
	database.DB.Model(&models.TaskLog{}).Select("action as type, count(*) as count").Group("action").Scan(&taskStats)

	// Recent Logs
	var ops []models.OperationLog
	var tasks []models.TaskLog

	database.DB.Order("created_at desc").Limit(50).Preload("User").Find(&ops)
	database.DB.Order("created_at desc").Limit(50).Preload("User").Find(&tasks)

	taskCodes := map[uint]string{}
	if len(tasks) > 0 {
		idSet := map[uint]struct{}{}
		for _, logItem := range tasks {
			idSet[logItem.TaskID] = struct{}{}
		}
		var ids []uint
		for id := range idSet {
			ids = append(ids, id)
		}
		var taskRows []models.Task
		database.DB.Select("id, task_code").Where("id IN ?", ids).Find(&taskRows)
		for _, row := range taskRows {
			if row.TaskCode != "" {
				taskCodes[row.ID] = row.TaskCode
			}
		}
	}

	return c.Render("pages/logs", fiber.Map{
		"OperationLogs": ops,
		"TaskLogs":      tasks,
		"TaskCodes":     taskCodes,
		"OpStats":       opStats,
		"TaskStats":     taskStats,
		"Username":      getUsername(c),
	}, "layouts/main")
}

// --- SQL Scripts ---

func GetSQLScripts(c *fiber.Ctx) error {
	var scripts []models.SQLScript
	search := c.Query("search")

	query := database.DB.Order("name asc, version desc")

	if search != "" {
		query = query.Where("name LIKE ? OR content LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	query.Find(&scripts)

	return c.Render("pages/sql_scripts", fiber.Map{
		"Scripts":  scripts,
		"Search":   search,
		"Username": getUsername(c),
	}, "layouts/main")
}

func GetSQLScript(c *fiber.Ctx) error {
	id := c.Params("id")
	var script models.SQLScript
	if err := database.DB.First(&script, id).Error; err != nil {
		return c.Status(404).SendString("Script not found")
	}

	// Return HTML Fragment for Display
	html := fmt.Sprintf(`
		<div class="mb-4">
			<div class="flex-between mb-2">
				<h3 style="margin: 0;">%s <span style="font-size: 0.6em; color: var(--text-muted);">v%d</span></h3>
				<button class="btn" onclick="copyToClipboard()">Copy to Clipboard</button>
			</div>
			<div style="background-color: #1e1e1e; padding: 15px; border-radius: 8px; overflow-x: auto;">
				<pre style="margin: 0;"><code id="script-content" style="font-family: monospace; color: #d4d4d4;">%s</code></pre>
			</div>
			<div class="mt-2" style="font-size: 0.9em; color: var(--text-muted);">
				%s
			</div>
		</div>
	`, script.Name, script.Version, script.Content, script.Description)

	return c.SendString(html)
}

func GetSQLScriptDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	var script models.SQLScript
	if err := database.DB.First(&script, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Script not found"})
	}
	return c.JSON(script)
}

func CreateSQLScript(c *fiber.Ctx) error {
	id := c.FormValue("id") // Check if updating existing
	name := c.FormValue("name")
	content := c.FormValue("content")
	desc := c.FormValue("description")

	if name == "" || content == "" {
		return c.Status(400).SendString("Name and Content are required")
	}

	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		userID = 1
	}

	// If ID is provided, update existing
	if id != "" {
		var script models.SQLScript
		if err := database.DB.First(&script, id).Error; err == nil {
			script.Name = name
			script.Content = content
			script.Description = desc
			script.Version = script.Version + 1
			script.UpdatedAt = time.Now()
			database.DB.Save(&script)
			return c.Redirect("/sql")
		}
	}

	// Create new
	var latest models.SQLScript
	var newVersion int = 1

	// Find latest version of this script
	if err := database.DB.Where("name = ?", name).Order("version desc").First(&latest).Error; err == nil {
		newVersion = latest.Version + 1
	}

	script := models.SQLScript{
		Name:        name,
		Content:     content,
		Description: desc,
		Version:     newVersion,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := database.DB.Create(&script).Error; err != nil {
		return c.Status(500).SendString("Failed to save script")
	}

	return c.Redirect("/sql")
}

func DeleteSQLScript(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&models.SQLScript{}, id).Error; err != nil {
		return c.Status(500).SendString("Error deleting script")
	}
	// Return empty string to remove element from DOM
	return c.SendString("")
}

// Knowledge Base Handlers
func GetKnowledgeBase(c *fiber.Ctx) error {
	var knowledges []models.Knowledge
	search := c.Query("search")
	category := c.Query("category")

	query := database.DB.Order("created_at desc")

	if search != "" {
		query = query.Where("title LIKE ? OR content LIKE ? OR tags LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}

	query.Find(&knowledges)

	return c.Render("pages/knowledge", fiber.Map{
		"Knowledges": knowledges,
		"Search":     search,
		"Category":   category,
		"Username":   getUsername(c),
	}, "layouts/main")
}

func CreateKnowledge(c *fiber.Ctx) error {
	knowledge := new(models.Knowledge)
	if err := c.BodyParser(knowledge); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	// Simple validation
	if knowledge.Title == "" || knowledge.Content == "" {
		return c.Status(400).SendString("Title and Content are required")
	}

	// Default user ID (should be from context in real app)
	knowledge.CreatedBy = 1

	if err := database.DB.Create(knowledge).Error; err != nil {
		return c.Status(500).SendString("Error creating knowledge entry")
	}

	return c.Redirect("/knowledge")
}

func GetKnowledgeContent(c *fiber.Ctx) error {
	id := c.Params("id")
	var kb models.Knowledge
	if err := database.DB.First(&kb, id).Error; err != nil {
		return c.Status(404).SendString("Knowledge not found")
	}

	// Render content (simple HTML format for now, could be Markdown rendered)
	html := fmt.Sprintf(`
		<div class="mb-4">
			<div class="flex-between mb-2">
				<h3 style="margin: 0;">%s</h3>
				<span class="badge" style="background-color: var(--secondary-color);">%s</span>
			</div>
			<div style="font-size: 0.8em; color: var(--text-muted); margin-bottom: 15px;">
				Tags: %s | Updated: %s
			</div>
			<div style="background-color: #1e1e1e; padding: 20px; border-radius: 8px; line-height: 1.6; white-space: pre-wrap;">%s</div>
			<div class="mt-4 text-right">
				<button class="btn btn-danger" hx-delete="/knowledge/%d" hx-confirm="Are you sure?" hx-target="#knowledge-%d" hx-swap="outerHTML">Delete</button>
			</div>
		</div>
	`, kb.Title, kb.Category, kb.Tags, kb.UpdatedAt.Format("Jan 02, 2006"), kb.Content, kb.ID, kb.ID)

	return c.SendString(html)
}

func DeleteKnowledge(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Delete(&models.Knowledge{}, id).Error; err != nil {
		return c.Status(500).SendString("Error deleting knowledge")
	}
	return c.SendString("") // Remove from UI
}

func RunCustomSQL(c *fiber.Ctx) error {
	query := c.FormValue("query")
	mode := c.FormValue("mode")

	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		userID = 1
	}

	role, ok := c.Locals("role").(string)
	if !ok {
		role = models.RoleAdmin
	} // Default to admin for dev safety or handle otherwise

	isAdmin := (role == models.RoleAdmin)
	readOnly := (mode == "readonly")

	result, err := services.ExecuteSafeQuery(query, userID, isAdmin, readOnly)
	if err != nil {
		return c.Status(400).SendString(fmt.Sprintf("<div style='color:red'>Error: %v</div>", err))
	}

	// Build HTML Table
	html := fmt.Sprintf("<p>Duration: %v | Rows: %d</p>", result.Duration, len(result.Rows))
	html += "<table border='1'><thead><tr>"
	for _, col := range result.Columns {
		html += "<th>" + col + "</th>"
	}
	html += "</tr></thead><tbody>"

	for _, row := range result.Rows {
		html += "<tr>"
		for _, val := range row {
			html += fmt.Sprintf("<td>%v</td>", val)
		}
		html += "</tr>"
	}
	html += "</tbody></table>"

	return c.SendString(html)
}

func RunSQLScript(c *fiber.Ctx) error {
	id := c.Params("id")
	var script models.SQLScript
	if err := database.DB.First(&script, id).Error; err != nil {
		return c.Status(404).SendString("Script not found")
	}

	// Run as read-only by default for safety
	userID, ok := c.Locals("user_id").(uint)
	if !ok {
		userID = 1
	}

	role, ok := c.Locals("role").(string)
	if !ok {
		role = models.RoleAdmin
	}

	isAdmin := (role == models.RoleAdmin)

	result, err := services.ExecuteSafeQuery(script.Content, userID, isAdmin, true)
	if err != nil {
		return c.Status(400).SendString("Error executing script: " + err.Error())
	}

	return c.SendString(fmt.Sprintf("Executed successfully. %d rows returned.", len(result.Rows)))
}

// --- Log Tools ---

func AnalyzeLogFile(c *fiber.Ctx) error {
	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		return c.Status(400).SendString("<div class='alert alert-danger'>Failed to parse form.</div>")
	}

	files := form.File["log_file"]
	if len(files) == 0 {
		return c.Status(400).SendString("<div class='alert alert-danger'>Please select at least one file.</div>")
	}

	// Check file count limit
	if len(files) > 5 {
		return c.Status(400).SendString("<div class='alert alert-danger'>Max 5 files allowed.</div>")
	}

	service := log_analyzer.NewAnalyzerService()
	result, err := service.ParseAndAnalyze(files)
	if err != nil {
		return c.Status(500).SendString(fmt.Sprintf("<div class='alert alert-danger'>Analysis failed: %v</div>", err))
	}

	// Check user question
	question := c.FormValue("question")
	var questionResults []log_analyzer.RootCause
	if question != "" {
		questionResults = service.AnswerUserQuestion(result.Entries, question)
	}

	// Build Result HTML (Advanced View)
	resp := fmt.Sprintf(`
		<div style="background-color: var(--bg-color); padding: 15px; border-radius: 8px; border: 1px solid var(--border-color); margin-top: 15px;">
			<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
				<h4 style="margin: 0;">Analysis Result (%d files)</h4>
                <div>
                    <span style="font-size: 0.8em; color: var(--text-muted); margin-right: 10px;">%s - %s</span>
                    <button class="btn btn-sm" onclick="exportLogReport()" style="padding: 4px 8px; font-size: 0.8em;">Export Report</button>
                </div>
			</div>
			
			<div style="display: flex; gap: 15px; margin-bottom: 20px;">
				<div class="stat-card" style="flex: 1; padding: 10px; background: #2d2d2d; border-radius: 4px; text-align: center;">
					<div style="color: var(--text-muted); font-size: 0.8em;">Total Lines</div>
					<div style="font-size: 1.5em; font-weight: bold;">%d</div>
				</div>
				<div class="stat-card" style="flex: 1; padding: 10px; background: #3d2d2d; border-radius: 4px; text-align: center;">
					<div style="color: var(--danger-color); font-size: 0.8em;">Errors</div>
					<div style="font-size: 1.5em; font-weight: bold; color: var(--danger-color);">%d</div>
				</div>
				<div class="stat-card" style="flex: 1; padding: 10px; background: #3d3d2d; border-radius: 4px; text-align: center;">
					<div style="color: var(--warning-color); font-size: 0.8em;">Warnings</div>
					<div style="font-size: 1.5em; font-weight: bold; color: var(--warning-color);">%d</div>
				</div>
			</div>
	`, len(files), result.StartTime.Format("15:04:05"), result.EndTime.Format("15:04:05"), result.TotalLines, result.ErrorCount, result.WarningCount)

	// User Question Results
	if question != "" {
		safeQuestion := html.EscapeString(question)
		resp += fmt.Sprintf(`<div style="margin-bottom: 20px; padding: 15px; background: #1a2a3a; border-left: 4px solid var(--primary-color); border-radius: 4px;">
            <h5 style="margin-top: 0; color: var(--primary-color); display: flex; align-items: center; gap: 8px;">
                <span>Answer to: "%s"</span>
            </h5>`, safeQuestion)

		for _, cause := range questionResults {
			resp += fmt.Sprintf(`
                <div style="margin-bottom: 10px;">
                    <div style="font-weight: bold;">%s</div>
                    <div style="font-size: 0.9em; margin-top: 4px;">%s</div>
                </div>
            `, html.EscapeString(cause.Title), html.EscapeString(cause.Description))
		}
		resp += `</div>`
	}

	// Root Causes
	if len(result.RootCauses) > 0 {
		resp += `<h5 style="margin-bottom: 10px;">Potential Root Causes</h5>
		<div style="display: grid; gap: 10px; margin-bottom: 20px;">`
		for _, rc := range result.RootCauses {
			// Skip generic "Found relevant log entries" if we already showed it in Answer section
			if question != "" && rc.Title == "Found relevant log entries" {
				continue
			}

			resp += fmt.Sprintf(`
				<div style="padding: 10px; border: 1px solid var(--border-color); border-radius: 4px; background: #222;">
					<div style="display: flex; justify-content: space-between;">
						<strong style="color: var(--danger-color);">%s</strong>
						<span class="badge">%s Confidence</span>
					</div>
					<div style="font-size: 0.9em; margin-top: 5px;">%s</div>
				</div>
			`, html.EscapeString(rc.Title), html.EscapeString(rc.Confidence), html.EscapeString(rc.Description))
		}
		resp += `</div>`
	}

	// ML: Pattern Clustering
	if len(result.Clusters) > 0 {
		resp += `<h5 style="margin-top: 20px; margin-bottom: 10px;">Log Pattern Clustering (Top 10)</h5>
		<div style="overflow-x: auto; margin-bottom: 20px;">
			<table style="width: 100%; border-collapse: collapse; font-size: 0.9em; border: 1px solid var(--border-color);">
				<thead style="background: #2a2a2a;">
					<tr>
						<th style="padding: 8px; text-align: left;">Count</th>
						<th style="padding: 8px; text-align: left;">%</th>
						<th style="padding: 8px; text-align: left;">Pattern Sample</th>
					</tr>
				</thead>
				<tbody>`

		for i, cluster := range result.Clusters {
			if i >= 10 {
				break
			} // Show top 10
			resp += fmt.Sprintf(`
				<tr style="border-bottom: 1px solid #333;">
					<td style="padding: 8px;">%d</td>
					<td style="padding: 8px;">%.1f%%</td>
					<td style="padding: 8px; font-family: monospace; color: var(--text-muted);">%s</td>
				</tr>
			`, cluster.Count, cluster.Percentage, html.EscapeString(cluster.Sample))
		}
		resp += `</tbody></table></div>`
	}

	// ML: Anomaly Detection
	if len(result.Anomalies) > 0 {
		resp += `<h5 style="margin-top: 20px; margin-bottom: 10px;">Detected Anomalies (Volume Spikes)</h5>
		<div style="display: grid; gap: 10px; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); margin-bottom: 20px;">`

		for _, anomaly := range result.Anomalies {
			color := "var(--warning-color)"
			if anomaly.Severity == "High" {
				color = "var(--danger-color)"
			}

			resp += fmt.Sprintf(`
				<div style="padding: 10px; background: #2a2a2a; border-left: 3px solid %s; border-radius: 4px;">
					<div style="font-weight: bold; margin-bottom: 5px;">%s</div>
					<div style="font-size: 0.8em; color: var(--text-muted);">
						Volume: %d (Exp: %.0f)<br>
						Deviation: %.1fx σ
					</div>
				</div>
			`, color, anomaly.Timestamp.Format("15:04"), anomaly.LogCount, anomaly.Expected, anomaly.Deviation)
		}
		resp += `</div>`
	}

	// Timeline Table Controls
	resp += `<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px;">
        <h5 style="margin: 0;">Event Timeline</h5>
        <select onchange="filterLogTable(this.value)" style="padding: 4px; background: var(--input-bg); color: var(--text-color); border: 1px solid var(--border-color); border-radius: 4px;">
            <option value="ALL">All Levels</option>
            <option value="ERROR">Errors Only</option>
            <option value="WARN">Warnings Only</option>
        </select>
    </div>
	<div style="max-height: 400px; overflow-y: auto; border: 1px solid var(--border-color); border-radius: 4px;">
		<table id="logTable" style="width: 100%; border-collapse: collapse; font-size: 0.85em;">
			<thead style="background: #333; position: sticky; top: 0;">
				<tr>
					<th style="padding: 8px; text-align: left;">Time</th>
					<th style="padding: 8px; text-align: left;">Level</th>
					<th style="padding: 8px; text-align: left;">Source</th>
					<th style="padding: 8px; text-align: left;">Message</th>
				</tr>
			</thead>
			<tbody>`

	for _, entry := range result.Timeline {
		color := "var(--text-color)"
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			color = "var(--danger-color)"
		} else if entry.Level == "WARN" {
			color = "var(--warning-color)"
		}

		resp += fmt.Sprintf(`
			<tr class="log-row" data-level="%s" style="border-bottom: 1px solid #333;">
				<td style="padding: 6px; white-space: nowrap; color: var(--text-muted);">%s</td>
				<td style="padding: 6px; color: %s;">%s</td>
				<td style="padding: 6px; color: var(--text-muted);">%s</td>
				<td style="padding: 6px; color: %s;">%s</td>
			</tr>
		`, html.EscapeString(string(entry.Level)), entry.Timestamp.Format("15:04:05.000"), color, html.EscapeString(string(entry.Level)), html.EscapeString(entry.Source), color, html.EscapeString(entry.Message))
	}

	resp += `</tbody></table></div>
    
    <script>
    function filterLogTable(level) {
        const rows = document.querySelectorAll('.log-row');
        rows.forEach(row => {
            if (level === 'ALL' || row.dataset.level === level) {
                row.style.display = '';
            } else {
                row.style.display = 'none';
            }
        });
    }
    
    function exportLogReport() {
        let csvContent = "data:text/csv;charset=utf-8,Time,Level,Source,Message\n";
        const rows = document.querySelectorAll('.log-row');
        rows.forEach(row => {
            if (row.style.display !== 'none') {
                const cols = row.querySelectorAll('td');
                const rowData = Array.from(cols).map(col => '"' + col.innerText.replace(/"/g, '""') + '"').join(",");
                csvContent += rowData + "\n";
            }
        });
        
        const encodedUri = encodeURI(csvContent);
        const link = document.createElement("a");
        link.setAttribute("href", encodedUri);
        link.setAttribute("download", "log_analysis_report.csv");
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    }
    </script>
    </div>`

	return c.SendString(resp)
}

// --- Auth ---

func LoginPage(c *fiber.Ctx) error {
	return c.Render("pages/login", fiber.Map{
		"IsLogin": true,
	}, "layouts/main")
}

func LoginPost(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	token, err := services.Login(username, password)
	if err != nil {
		return c.Render("pages/login", fiber.Map{
			"Error":   "Invalid credentials",
			"IsLogin": true,
		}, "layouts/main")
	}

	// Set Cookie
	cookie := new(fiber.Cookie)
	cookie.Name = "jwt"
	cookie.Value = token
	cookie.Expires = time.Now().Add(24 * time.Hour)
	cookie.HTTPOnly = true
	cookie.Secure = c.Protocol() == "https"
	c.Cookie(cookie)

	return c.Redirect("/")
}

func Logout(c *fiber.Ctx) error {
	c.ClearCookie("jwt")
	return c.Redirect("/login")
}

// --- File Upload ---
func UploadFile(c *fiber.Ctx) error {
	file, err := c.FormFile("attachment")
	if err != nil {
		return c.Status(400).SendString("Upload failed")
	}

	c.SaveFile(file, fmt.Sprintf("./public/uploads/%s", file.Filename))
	return c.SendString("File uploaded successfully: " + file.Filename)
}

// --- LINE Webhook ---
func LineWebhook(c *fiber.Ctx) error {
	req, err := http.NewRequest(http.MethodPost, "", bytes.NewReader(c.Body()))
	if err != nil {
		return c.SendStatus(500)
	}
	req.Header.Set("X-Line-Signature", c.Get("X-Line-Signature"))

	events, err := services.Bot.ParseRequest(req)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			return c.Status(400).SendString("Invalid signature")
		}
		return c.Status(500).SendString("Internal Error")
	}

	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				reply := services.HandleCommand(event.Source.UserID, message.Text)
				if reply != "" {
					if _, err = services.Bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(reply)).Do(); err != nil {
						fmt.Println("Error replying:", err)
					}
				}
			}
		}
	}

	return c.SendStatus(200)
}
