package entity

import "time"

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusOpen       TaskStatus = "Open"
	TaskStatusInProgress TaskStatus = "In Progress"
	TaskStatusCompleted  TaskStatus = "Completed"
	TaskStatusDeferred   TaskStatus = "Deferred"
	TaskStatusCancelled  TaskStatus = "Cancelled"
)

// TaskPriority represents the priority of a task
type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "Low"
	TaskPriorityNormal TaskPriority = "Normal"
	TaskPriorityHigh   TaskPriority = "High"
	TaskPriorityUrgent TaskPriority = "Urgent"
)

// TaskType represents the type of task/activity
type TaskType string

const (
	TaskTypeCall    TaskType = "Call"
	TaskTypeEmail   TaskType = "Email"
	TaskTypeMeeting TaskType = "Meeting"
	TaskTypeTodo    TaskType = "Todo"
)

// Task represents a CRM task/activity with polymorphic linking
type Task struct {
	ID             string                 `json:"id" db:"id"`
	OrgID          string                 `json:"orgId" db:"org_id"`
	Subject        string                 `json:"subject" db:"subject"`
	Description    string                 `json:"description" db:"description"`
	Status         TaskStatus             `json:"status" db:"status"`
	Priority       TaskPriority           `json:"priority" db:"priority"`
	Type           TaskType               `json:"type" db:"type"`
	DueDate        *string                `json:"dueDate,omitempty" db:"due_date"`
	ParentID       *string                `json:"parentId,omitempty" db:"parent_id"`
	ParentType     *string                `json:"parentType,omitempty" db:"parent_type"`
	ParentName     string                 `json:"parentName" db:"parent_name"`
	GmailMessageID *string                `json:"gmailMessageId,omitempty" db:"gmail_message_id"`
	AssignedUserID *string                `json:"assignedUserId,omitempty" db:"assigned_user_id"`
	CreatedByID    *string                `json:"createdById,omitempty" db:"created_by_id"`
	CreatedByName  string                 `json:"createdByName" db:"-"`
	ModifiedByID   *string                `json:"modifiedById,omitempty" db:"modified_by_id"`
	ModifiedByName string                 `json:"modifiedByName" db:"-"`
	CreatedAt      time.Time              `json:"createdAt" db:"created_at"`
	ModifiedAt     time.Time              `json:"modifiedAt" db:"modified_at"`
	Deleted        bool                   `json:"deleted" db:"deleted"`
	CustomFieldsRaw string                `json:"-" db:"custom_fields"`
	CustomFields   map[string]interface{} `json:"customFields,omitempty" db:"-"`
}

// TaskCreateInput represents the input for creating a task
type TaskCreateInput struct {
	Subject        string                 `json:"subject" validate:"required"`
	Description    string                 `json:"description"`
	Status         TaskStatus             `json:"status"`
	Priority       TaskPriority           `json:"priority"`
	Type           TaskType               `json:"type"`
	DueDate        *string                `json:"dueDate"`
	ParentID       *string                `json:"parentId"`
	ParentType     *string                `json:"parentType"`
	ParentName     string                 `json:"parentName"`
	GmailMessageID *string                `json:"gmailMessageId"`
	AssignedUserID *string                `json:"assignedUserId"`
	CustomFields   map[string]interface{} `json:"customFields"`
}

// TaskUpdateInput represents the input for updating a task
type TaskUpdateInput struct {
	Subject        *string                `json:"subject"`
	Description    *string                `json:"description"`
	Status         *TaskStatus            `json:"status"`
	Priority       *TaskPriority          `json:"priority"`
	Type           *TaskType              `json:"type"`
	DueDate        *string                `json:"dueDate"`
	ParentID       *string                `json:"parentId"`
	ParentType     *string                `json:"parentType"`
	ParentName     *string                `json:"parentName"`
	GmailMessageID *string                `json:"gmailMessageId"`
	AssignedUserID *string                `json:"assignedUserId"`
	CustomFields   map[string]interface{} `json:"customFields"`
}

// TaskListParams represents query parameters for listing tasks
type TaskListParams struct {
	Search         string `query:"search"`
	SortBy         string `query:"sortBy"`
	SortDir        string `query:"sortDir"`
	Page           int    `query:"page"`
	PageSize       int    `query:"pageSize"`
	Status         string `query:"status"`
	Type           string `query:"type"`
	ParentType     string `query:"parentType"`
	ParentID       string `query:"parentId"`
	DueBefore      string `query:"dueBefore"`
	DueAfter       string `query:"dueAfter"`
	GmailMessageID string `query:"gmailMessageId"`
	Filter         string `query:"filter"`
	KnownTotal     int    `query:"knownTotal"`
}

// TaskListResponse represents the response for listing tasks
type TaskListResponse struct {
	Data       []Task `json:"data"`
	Total      int    `json:"total"`
	Page       int    `json:"page"`
	PageSize   int    `json:"pageSize"`
	TotalPages int    `json:"totalPages"`
}
