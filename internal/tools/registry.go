// Package tools declares the Dooray MCP tool definitions and their handlers.
package tools

import (
	"strings"

	"github.com/minseoky/dooray-mcp-go/internal/jsonschema"
	"github.com/minseoky/dooray-mcp-go/internal/mcp"
)

// writeToolNames are hidden when the server runs in read-only mode.
var writeToolNames = map[string]bool{
	"dooray_messenger":           true,
	"dooray_calendar_post_event": true,
	"dooray_post_log_create":     true,
	"dooray_post_log_update":     true,
}

// Definitions returns every implemented tool in declaration order.
func Definitions() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "dooray_messenger",
			Description: "send message to dooray messenger",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"send"}, "The operation to perform")),
				jsonschema.Prop("to", jsonschema.String("recipient organizationMemberId")),
				jsonschema.Prop("message", jsonschema.String("message to send")),
			}, "operation", "to", "message"),
		},
		{
			Name:        "dooray_calendar_calendars",
			Description: "find dooray calendars",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_calendars"}, "The operation to perform")),
			}, "operation"),
		},
		{
			Name:        "dooray_calendar_events",
			Description: "find dooray events of calendars",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_events"}, "The operation to perform")),
				jsonschema.Prop("calendars", jsonschema.String("calendar ids separated by commas")),
				jsonschema.Prop("timeMin", jsonschema.String("inclusive start time in ISO 8601, e.g. 2025-04-11T00:00:00+09:00")),
				jsonschema.Prop("timeMax", jsonschema.String("exclusive end time in ISO 8601, e.g. 2025-04-12T00:00:00+09:00")),
			}, "operation", "timeMin", "timeMax"),
		},
		{
			Name:        "dooray_calendar_post_event",
			Description: "register dooray events on a calendar",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"create_event"}, "The operation to perform")),
				jsonschema.Prop("calendarId", jsonschema.String("calendar id to register an event")),
				jsonschema.Prop("subject", jsonschema.String("event subject")),
				jsonschema.Prop("content", jsonschema.String("event content")),
				jsonschema.Prop("startedAt", jsonschema.String("event start time in ISO 8601")),
				jsonschema.Prop("endedAt", jsonschema.String("event end time in ISO 8601")),
				jsonschema.Prop("wholeDayFlag", jsonschema.Boolean("set true for whole day event")),
				jsonschema.Prop("recurrenceFrequency", jsonschema.Enum([]string{"", "daily", "weekly", "monthly", "yearly"}, "recurrence frequency")),
				jsonschema.Prop("recurrenceInterval", jsonschema.Number("recurrence interval, default is 1")),
				jsonschema.Prop("recurrenceUntil", jsonschema.String("recurrence end date in ISO 8601")),
				jsonschema.Prop("recurrenceByday", jsonschema.String("recurrence by day, e.g. MO,TU,WE")),
				jsonschema.Prop("recurrenceBymonth", jsonschema.String("recurrence by month, 1-12")),
				jsonschema.Prop("recurrenceBymonthday", jsonschema.String("recurrence by day of month, 1-31")),
				jsonschema.Prop("recurrenceTimezoneName", jsonschema.String("timezone for recurrence rule, default Asia/Seoul")),
			}, "operation", "subject", "content", "startedAt", "endedAt"),
		},
		{
			Name:        "dooray_account_members",
			Description: "find dooray account members by name or userCode",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_member_id"}, "The operation to perform")),
				jsonschema.Prop("member_name", jsonschema.String("member name")),
				jsonschema.Prop("user_code", jsonschema.String("user code")),
			}, "operation", "member_name"),
		},
		{
			Name:        "dooray_account_member",
			Description: "find dooray account members by id",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_member_details"}, "The operation to perform")),
				jsonschema.Prop("member_id", jsonschema.String("member id")),
			}, "operation", "member_id"),
		},
		{
			Name: "dooray_project",
			Description: strings.Join([]string{
				"List Dooray projects accessible to the token or get one project by ID.",
				"A /task/{projectId}/{postId} URL provides projectId directly, while a legacy /project/tasks/{postId} URL contains only postId and requires project discovery.",
				"find_projects requires type, scope, and state; use page and size to exhaust every relevant filter combination because one page is not proof that a project or post is inaccessible.",
				"find_project requires projectId.",
			}, " "),
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_projects", "find_project"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("project id; required for find_project")),
				jsonschema.Prop("type", jsonschema.Enum([]string{"public", "private"}, "project type; required for find_projects")),
				jsonschema.Prop("state", jsonschema.Enum([]string{"active", "archived"}, "project state; required for find_projects")),
				jsonschema.Prop("scope", jsonschema.Enum([]string{"private", "public"}, "project scope; required for find_projects")),
				jsonschema.Prop("page", jsonschema.Number("project list page number; default is 0")),
				jsonschema.Prop("size", jsonschema.Number("number of projects per page; default is 100, max is 100")),
			}, "operation"),
		},
		{
			Name: "dooray_posts",
			Description: strings.Join([]string{
				"Find Dooray task posts in one project or a comma-separated set of accessible project IDs.",
				"Parse the URL first: /task/{projectId}/{postId} provides both IDs, while legacy /project/tasks/{postId} provides only postId and requires project discovery with dooray_project.",
				"Query the known or candidate project IDs with size up to 100 and continue through pages until the returned post ID matches; a lookup under one wrong project ID or only the first page does not prove that the post is unavailable.",
				"The matching post response contains the task body, so do not call dooray_post_logs unless comments or activity were explicitly requested.",
				"When the post has fileIdList, treat every ID as a downloadable body file, commonly an inline image, and call dooray_post_file_download with the same verified projectId and postId.",
				"Always try that direct download even if dooray_post_files is empty or returns AUTH_FORBIDDEN_ERROR.",
			}, " "),
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_posts"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("project id from /task/{projectId}/{postId}, or comma separated accessible project ids discovered for a legacy /project/tasks/{postId} URL")),
				jsonschema.Prop("page", jsonschema.Number("page number, default is 0")),
				jsonschema.Prop("size", jsonschema.Number("number of posts per page, default is 20, max is 100")),
				jsonschema.Prop("fromEmailAddress", jsonschema.String("filter posts by sender email address")),
				jsonschema.Prop("fromMemberIds", jsonschema.String("filter posts by creator member ids, comma separated")),
				jsonschema.Prop("toMemberIds", jsonschema.String("filter posts by assignee member ids, comma separated")),
				jsonschema.Prop("toMemberSize", jsonschema.Number("filter by number of assignees")),
				jsonschema.Prop("ccMemberIds", jsonschema.String("filter posts by cc member ids, comma separated")),
				jsonschema.Prop("tagIds", jsonschema.String("filter posts by tag ids, comma separated")),
				jsonschema.Prop("parentPostId", jsonschema.String("filter sub-tasks of a parent post")),
				jsonschema.Prop("postNumber", jsonschema.String("filter by post number")),
				jsonschema.Prop("postWorkflowClasses", jsonschema.String("backlog, registered, working, closed")),
				jsonschema.Prop("postWorkflowIds", jsonschema.String("filter by workflow ids, comma separated")),
				jsonschema.Prop("milestoneIds", jsonschema.String("filter by milestone ids, comma separated")),
				jsonschema.Prop("subjects", jsonschema.String("filter by post subject keyword")),
				jsonschema.Prop("createdAt", jsonschema.String("date filter: today, thisweek, prev-{N}d, next-{N}d, or ISO8601~ISO8601")),
				jsonschema.Prop("updatedAt", jsonschema.String("date filter: today, thisweek, prev-{N}d, next-{N}d, or ISO8601~ISO8601")),
				jsonschema.Prop("dueAt", jsonschema.String("date filter: today, thisweek, prev-{N}d, next-{N}d, or ISO8601~ISO8601")),
				jsonschema.Prop("order", jsonschema.String("sort order: postDueAt, postUpdatedAt, createdAt, prefix '-' for descending")),
			}, "operation", "projectId"),
		},
		{
			Name:        "dooray_post_logs",
			Description: "find comments and activity logs of a Dooray post",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_logs"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("project id")),
				jsonschema.Prop("postId", jsonschema.String("post id")),
				jsonschema.Prop("page", jsonschema.Number("page number, default is 0")),
				jsonschema.Prop("size", jsonschema.Number("number of logs per page, default is 20")),
				jsonschema.Prop("order", jsonschema.Enum([]string{"createdAt", "-createdAt"}, "log sort order")),
			}, "operation", "projectId", "postId"),
		},
		{
			Name:        "dooray_post_log",
			Description: "find a specific comment or activity log of a Dooray post",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_log"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("project id")),
				jsonschema.Prop("postId", jsonschema.String("post id")),
				jsonschema.Prop("logId", jsonschema.String("log id, or comment id")),
			}, "operation", "projectId", "postId", "logId"),
		},
		{
			Name:        "dooray_post_log_create",
			Description: "add a comment to a Dooray post",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"create_log"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("project id")),
				jsonschema.Prop("postId", jsonschema.String("post id")),
				jsonschema.Prop("body", jsonschema.Object(jsonschema.Properties{
					jsonschema.Prop("mimeType", jsonschema.Enum([]string{"text/x-markdown"}, "comment body MIME type")),
					jsonschema.Prop("content", jsonschema.String("comment content")),
				}, "mimeType", "content")),
			}, "operation", "projectId", "postId", "body"),
		},
		{
			Name:        "dooray_post_log_update",
			Description: "update a comment or activity log of a Dooray post",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"update_log"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("project id")),
				jsonschema.Prop("postId", jsonschema.String("post id")),
				jsonschema.Prop("logId", jsonschema.String("log id, or comment id")),
				jsonschema.Prop("body", jsonschema.Object(jsonschema.Properties{
					jsonschema.Prop("mimeType", jsonschema.Enum([]string{"text/x-markdown"}, "comment body MIME type")),
					jsonschema.Prop("content", jsonschema.String("updated comment content")),
				}, "mimeType", "content")),
			}, "operation", "projectId", "postId", "logId", "body"),
		},
		{
			Name:        "dooray_post_files",
			Description: "List regular attachments only. This is not an availability check for inline body images in dooray_posts.fileIdList. The endpoint can be empty or return AUTH_FORBIDDEN_ERROR while direct dooray_post_file_download calls still succeed, so never use this result alone to conclude that body files cannot be read.",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"find_files"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("project id")),
				jsonschema.Prop("postId", jsonschema.String("post id")),
			}, "operation", "projectId", "postId"),
		},
		{
			Name: "dooray_post_file_download",
			Description: strings.Join([]string{
				"Download a Dooray post file by fileId through authenticated redirects to a local temporary directory.",
				"Authorization is forwarded only to the configured API origin and the HTTPS file-api.dooray.com download service; arbitrary redirect origins receive no Dooray token.",
				"Use this for every ID from the matching dooray_posts.fileIdList, including inline body images, with the same verified projectId and postId; it also supports regular attachment file IDs.",
				"dooray_post_files is not a prerequisite, and an empty result or AUTH_FORBIDDEN_ERROR from that separate endpoint does not imply that this direct download will fail.",
				"On success, the result contains filePath, fileName, mimeType, size, and temporary; inspect filePath with an appropriate local image viewer or document parser.",
				"A fetch failure or timeout is transient and must not be reported as a permission denial; retry or report the transport problem separately.",
				"Only report a body file as forbidden or missing when this direct request returns a terminal Dooray response such as 403 or 404 with the verified projectId, postId, and fileId.",
			}, " "),
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"download"}, "The operation to perform")),
				jsonschema.Prop("projectId", jsonschema.String("verified project id of the matching post; for /task/{projectId}/{postId} URLs this is the first numeric path value")),
				jsonschema.Prop("postId", jsonschema.String("matching post id; this is the final numeric path value in both supported task URL forms")),
				jsonschema.Prop("fileId", jsonschema.String("file id from the matching post's fileIdList or regular attachment list")),
				jsonschema.Prop("fileName", jsonschema.String("optional file name used when the response does not provide one")),
			}, "operation", "projectId", "postId", "fileId"),
		},
		{
			Name:        "os",
			Description: "get os time date",
			InputSchema: jsonschema.Object(jsonschema.Properties{
				jsonschema.Prop("operation", jsonschema.Enum([]string{"get_date_time"}, "The operation to get date time")),
			}, "operation"),
		},
	}
}

// IsWriteTool reports whether the tool mutates Dooray state.
func IsWriteTool(name string) bool {
	return writeToolNames[name]
}
