package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/minseoky/dooray-mcp-go/internal/dooray"
	"github.com/minseoky/dooray-mcp-go/internal/mcp"
)

// Registry exposes the tools visible in the current mode together with their
// handlers.
type Registry struct {
	Tools    []mcp.Tool
	Handlers map[string]mcp.Handler
}

// NewRegistry builds the tool set for the given mode. When readOnly is true,
// write-capable tools are neither listed nor callable.
func NewRegistry(client *dooray.Client, readOnly bool) *Registry {
	all := Definitions()
	handlers := map[string]mcp.Handler{
		"dooray_messenger":           messenger(client),
		"dooray_calendar_calendars":  calendarCalendars(client),
		"dooray_calendar_events":     calendarEvents(client),
		"dooray_calendar_post_event": calendarPostEvent(client),
		"dooray_account_members":     accountMembers(client),
		"dooray_account_member":      accountMember(client),
		"dooray_project":             project(client),
		"dooray_posts":               posts(client),
		"dooray_post_logs":           postLogs(client),
		"dooray_post_log":            postLog(client),
		"dooray_post_log_create":     postLogCreate(client),
		"dooray_post_log_update":     postLogUpdate(client),
		"dooray_post_files":          postFiles(client),
		"dooray_post_file_download":  postFileDownload(client),
		"os":                         osDateTime(),
	}

	registry := &Registry{Handlers: map[string]mcp.Handler{}}
	for _, tool := range all {
		if readOnly && IsWriteTool(tool.Name) {
			continue
		}
		handler, ok := handlers[tool.Name]
		if !ok {
			continue
		}
		registry.Tools = append(registry.Tools, tool)
		registry.Handlers[tool.Name] = handler
	}
	return registry
}

func messenger(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "send"); err != nil {
			return "", err
		}
		if err := requireConfirmation(input); err != nil {
			return "", err
		}
		message, err := requireString(input, "message")
		if err != nil {
			return "", err
		}
		to, err := requireString(input, "to")
		if err != nil {
			return "", err
		}
		return client.Request(ctx, http.MethodPost, "/messenger/v1/channels/direct-send", nil, map[string]any{
			"text":                 message,
			"organizationMemberId": to,
		})
	}
}

func calendarCalendars(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_calendars"); err != nil {
			return "", err
		}
		return client.Request(ctx, http.MethodGet, "/calendar/v1/calendars", nil, nil)
	}
}

func calendarEvents(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_events"); err != nil {
			return "", err
		}
		timeMin, err := requireString(input, "timeMin")
		if err != nil {
			return "", err
		}
		timeMax, err := requireString(input, "timeMax")
		if err != nil {
			return "", err
		}

		query := url.Values{}
		set(query, "calendars", input["calendars"])
		query.Set("timeMin", timeMin)
		query.Set("timeMax", timeMax)

		return client.Request(ctx, http.MethodGet, "/calendar/v1/calendars/*/events", query, nil)
	}
}

func calendarPostEvent(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "create_event"); err != nil {
			return "", err
		}
		if err := requireConfirmation(input); err != nil {
			return "", err
		}
		subject, err := requireString(input, "subject")
		if err != nil {
			return "", err
		}
		content, err := requireString(input, "content")
		if err != nil {
			return "", err
		}
		startedAt, err := requireString(input, "startedAt")
		if err != nil {
			return "", err
		}
		endedAt, err := requireString(input, "endedAt")
		if err != nil {
			return "", err
		}

		calendarID := optionalString(input, "calendarId")
		if calendarID == "" {
			calendarID = "*"
		}

		wholeDay, _ := input["wholeDayFlag"].(bool)
		body := map[string]any{
			"users":   map[string]any{},
			"subject": subject,
			"body": map[string]any{
				"mimeType": "text/html",
				"content":  content,
			},
			"startedAt":    startedAt,
			"endedAt":      endedAt,
			"wholeDayFlag": wholeDay,
			"location":     "",
		}
		if rule := buildRecurrenceRule(input); rule != nil {
			body["recurrenceRule"] = rule
		}

		path := fmt.Sprintf("/calendar/v1/calendars/%s/events", dooray.EscapeComponent(calendarID))
		return client.Request(ctx, http.MethodPost, path, nil, body)
	}
}

func accountMembers(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_member_id"); err != nil {
			return "", err
		}
		name, err := requireString(input, "member_name")
		if err != nil {
			return "", err
		}

		query := url.Values{}
		query.Set("name", name)
		set(query, "userCode", input["user_code"])

		return client.Request(ctx, http.MethodGet, "/common/v1/members", query, nil)
	}
}

func accountMember(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_member_details"); err != nil {
			return "", err
		}
		memberID, err := requireString(input, "member_id")
		if err != nil {
			return "", err
		}
		path := "/common/v1/members/" + dooray.EscapeComponent(memberID)
		return client.Request(ctx, http.MethodGet, path, nil, nil)
	}
}

func project(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if operation, _ := input["operation"].(string); operation == "find_project" {
			projectID, err := requireString(input, "projectId")
			if err != nil {
				return "", err
			}
			return client.Request(ctx, http.MethodGet, "/project/v1/projects/"+dooray.EscapeComponent(projectID), nil, nil)
		}

		if err := requireOperation(input, "find_projects"); err != nil {
			return "", err
		}
		projectType, err := requireString(input, "type")
		if err != nil {
			return "", err
		}
		scope, err := requireString(input, "scope")
		if err != nil {
			return "", err
		}
		state, err := requireString(input, "state")
		if err != nil {
			return "", err
		}

		query := url.Values{}
		query.Set("member", "me")
		set(query, "page", coalesce(input, "page", 0))
		set(query, "size", coalesce(input, "size", 100))
		query.Set("type", projectType)
		query.Set("scope", scope)
		query.Set("state", state)

		return client.Request(ctx, http.MethodGet, "/project/v1/projects", query, nil)
	}
}

var postQueryKeys = []string{
	"page", "size", "fromEmailAddress", "fromMemberIds", "toMemberIds",
	"toMemberSize", "ccMemberIds", "tagIds", "parentPostId", "postNumber",
	"postWorkflowClasses", "postWorkflowIds", "milestoneIds", "subjects",
	"createdAt", "updatedAt", "dueAt", "order",
}

func posts(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_posts"); err != nil {
			return "", err
		}
		projectID, err := requireString(input, "projectId")
		if err != nil {
			return "", err
		}

		path := fmt.Sprintf("/project/v1/projects/%s/posts", dooray.EscapeComponent(projectID))
		return client.Request(ctx, http.MethodGet, path, pick(input, postQueryKeys), nil)
	}
}

func postLogs(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_logs"); err != nil {
			return "", err
		}
		projectID, postID, err := requirePostPath(input)
		if err != nil {
			return "", err
		}

		path := fmt.Sprintf("/project/v1/projects/%s/posts/%s/logs", projectID, postID)
		return client.Request(ctx, http.MethodGet, path, pick(input, []string{"page", "size", "order"}), nil)
	}
}

func postLog(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_log"); err != nil {
			return "", err
		}
		projectID, postID, err := requirePostPath(input)
		if err != nil {
			return "", err
		}
		logID, err := requireString(input, "logId")
		if err != nil {
			return "", err
		}

		path := fmt.Sprintf("/project/v1/projects/%s/posts/%s/logs/%s", projectID, postID, dooray.EscapeComponent(logID))
		return client.Request(ctx, http.MethodGet, path, nil, nil)
	}
}

func postLogCreate(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "create_log"); err != nil {
			return "", err
		}
		if err := requireConfirmation(input); err != nil {
			return "", err
		}
		projectID, postID, err := requirePostPath(input)
		if err != nil {
			return "", err
		}
		body, err := requireLogBody(input)
		if err != nil {
			return "", err
		}

		path := fmt.Sprintf("/project/v1/projects/%s/posts/%s/logs", projectID, postID)
		return client.Request(ctx, http.MethodPost, path, nil, map[string]any{"body": body})
	}
}

func postLogUpdate(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "update_log"); err != nil {
			return "", err
		}
		if err := requireConfirmation(input); err != nil {
			return "", err
		}
		projectID, postID, err := requirePostPath(input)
		if err != nil {
			return "", err
		}
		logID, err := requireString(input, "logId")
		if err != nil {
			return "", err
		}
		body, err := requireLogBody(input)
		if err != nil {
			return "", err
		}

		path := fmt.Sprintf("/project/v1/projects/%s/posts/%s/logs/%s", projectID, postID, dooray.EscapeComponent(logID))
		return client.Request(ctx, http.MethodPut, path, nil, map[string]any{"body": body})
	}
}

func postFiles(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "find_files"); err != nil {
			return "", err
		}
		projectID, postID, err := requirePostPath(input)
		if err != nil {
			return "", err
		}

		path := fmt.Sprintf("/project/v1/projects/%s/posts/%s/files", projectID, postID)
		return client.Request(ctx, http.MethodGet, path, nil, nil)
	}
}

func postFileDownload(client *dooray.Client) mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "download"); err != nil {
			return "", err
		}
		projectID, postID, err := requirePostPath(input)
		if err != nil {
			return "", err
		}
		fileID, err := requireString(input, "fileId")
		if err != nil {
			return "", err
		}

		path := fmt.Sprintf("/project/v1/projects/%s/posts/%s/files/%s?media=raw",
			projectID, postID, dooray.EscapeComponent(fileID))

		result, err := client.Download(ctx, path, optionalString(input, "fileName"), fileID)
		if err != nil {
			return "", err
		}
		return marshalResult(result)
	}
}

func osDateTime() mcp.Handler {
	return func(ctx context.Context, input map[string]any) (string, error) {
		if err := requireOperation(input, "get_date_time"); err != nil {
			return "", err
		}
		return marshalResult(map[string]string{
			"time": time.Now().Format("2006-01-02 15:04:05"),
		})
	}
}

func requirePostPath(input map[string]any) (string, string, error) {
	projectID, err := requireString(input, "projectId")
	if err != nil {
		return "", "", err
	}
	postID, err := requireString(input, "postId")
	if err != nil {
		return "", "", err
	}
	return dooray.EscapeComponent(projectID), dooray.EscapeComponent(postID), nil
}

func requireLogBody(input map[string]any) (map[string]any, error) {
	body, ok := objectField(input, "body")
	if !ok {
		return nil, errors.New("body must be an object with mimeType and content")
	}
	mimeType, err := requireString(body, "mimeType")
	if err != nil {
		return nil, err
	}
	content, err := requireString(body, "content")
	if err != nil {
		return nil, err
	}
	return map[string]any{"mimeType": mimeType, "content": content}, nil
}

func buildRecurrenceRule(input map[string]any) map[string]any {
	frequency := optionalString(input, "recurrenceFrequency")
	if frequency == "" {
		return nil
	}

	interval := 1.0
	if value, ok := input["recurrenceInterval"].(float64); ok && value != 0 {
		interval = value
	}

	timezone := optionalString(input, "recurrenceTimezoneName")
	if timezone == "" {
		timezone = "Asia/Seoul"
	}

	return map[string]any{
		"frequency":    frequency,
		"interval":     interval,
		"until":        optionalString(input, "recurrenceUntil"),
		"byday":        optionalString(input, "recurrenceByday"),
		"bymonth":      optionalString(input, "recurrenceBymonth"),
		"bymonthday":   optionalString(input, "recurrenceBymonthday"),
		"timezoneName": timezone,
	}
}

func marshalResult(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
