package models

import (
	"time"

	"gorm.io/gorm"
)

// User Roles
const (
	RoleMaster       = "Master"
	RoleProjectAdmin = "ProjectAdmin"
	RoleMember       = "Member"
	RoleViewer       = "Viewer"
)

// User represents a system user
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;not null" json:"username"`
	Password  string         `gorm:"not null" json:"-"` // Hash
	Role      string         `gorm:"default:'Viewer'" json:"role"`
	Email     string         `json:"email"`
	LineID    string         `json:"line_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Project represents a workspace
type Project struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"not null" json:"name"`
	Key         string         `gorm:"uniqueIndex;size:10" json:"key"` // e.g. "INFRA", "BACK"
	Description string         `json:"description"`
	Status      string         `gorm:"default:'Active'" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Tasks       []Task         `json:"tasks"`
}

// Task statuses
const (
	TaskStatusPlanning = "Planning"
	TaskStatusCorrect  = "Correct"
	TaskStatusTest     = "Test"
	TaskStatusReady    = "Ready"
	TaskStatusDeploy   = "Deploy"
	TaskStatusDone     = "Done"
)

// Task represents a unit of work
type Task struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	TaskCode    string         `gorm:"index;size:20" json:"task_code"`
	ProjectID   uint           `gorm:"index;not null" json:"project_id"`
	Project     Project        `json:"project"`
	Title       string         `gorm:"not null" json:"title"`
	Description string         `json:"description"`                            // Markdown
	Status      string         `gorm:"index;default:'Planning'" json:"status"` // Planning, Correct, Test, Ready, Deploy, Done
	Priority    string         `gorm:"default:'Medium'" json:"priority"`       // Low, Medium, High, Critical
	AssigneeID  *uint          `gorm:"index" json:"assignee_id"`
	Assignee    *User          `gorm:"foreignKey:AssigneeID" json:"assignee"`
	CreatedByID uint           `gorm:"index" json:"created_by_id"`
	CreatedBy   User           `gorm:"foreignKey:CreatedByID" json:"created_by"`
	ParentID    *uint          `gorm:"index" json:"parent_id"`
	Parent      *Task          `gorm:"foreignKey:ParentID" json:"parent"`
	Subtasks    []Task         `gorm:"foreignKey:ParentID" json:"subtasks"`
	StartDate   *time.Time     `gorm:"index" json:"start_date"` // For Gantt/Timeline
	DueDate     *time.Time     `gorm:"index" json:"due_date"`
	Tags        string         `json:"tags"` // Comma separated
	CreatedAt   time.Time      `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Logs        []TaskLog      `json:"logs"`
}

func (t Task) IsOverdue() bool {
	if t.DueDate == nil {
		return false
	}
	if t.Status == TaskStatusDone {
		return false
	}
	return t.DueDate.Before(time.Now())
}

func IsValidTaskStatus(status string) bool {
	switch status {
	case TaskStatusPlanning, TaskStatusCorrect, TaskStatusTest, TaskStatusReady, TaskStatusDeploy, TaskStatusDone:
		return true
	default:
		return false
	}
}

// TaskLog tracks history and comments
type TaskLog struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	TaskID         uint      `json:"task_id" gorm:"index"`
	UserID         uint      `json:"user_id"`
	User           User      `json:"user"`
	Action         string    `json:"action"`
	Detail         string    `json:"detail"`
	Link           string    `json:"link"`
	AttachmentName string    `json:"attachment_name"`
	AttachmentPath string    `json:"attachment_path"`
	CreatedAt      time.Time `json:"created_at"`
}

// SQLScript manages stored SQL scripts
type SQLScript struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Type        string    `json:"type"` // Migration, Report, Fix
	Version     int       `json:"version" gorm:"default:1"`
	CreatedBy   uint      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Knowledge struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`  // Markdown or Text
	Category  string         `json:"category"` // Hardware, Software, Network, Account
	Tags      string         `json:"tags"`
	CreatedBy uint           `json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// OperationLog records system-wide actions
type OperationLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	User      User      `json:"user"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}
