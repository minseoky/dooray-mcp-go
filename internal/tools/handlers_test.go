package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/minseoky/dooray-mcp-go/internal/dooray"
	"github.com/minseoky/dooray-mcp-go/internal/jsonschema"
)

type capture struct {
	method string
	path   string
	query  string
	body   string
}

func newRegistry(t *testing.T, readOnly bool, recorded *capture) *Registry {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded.method = r.Method
		recorded.path = r.URL.EscapedPath()
		recorded.query = r.URL.RawQuery
		raw, _ := io.ReadAll(r.Body)
		recorded.body = string(raw)
		w.Write([]byte(`{"header":{"isSuccessful":true}}`))
	}))
	t.Cleanup(server.Close)

	client := dooray.New(dooray.Options{
		Endpoint:          server.URL,
		Token:             "test-token",
		Timeout:           5 * time.Second,
		DownloadDirectory: t.TempDir(),
	})
	return NewRegistry(client, readOnly)
}

func call(t *testing.T, registry *Registry, name string, input map[string]any) (string, error) {
	t.Helper()
	handler, ok := registry.Handlers[name]
	if !ok {
		t.Fatalf("tool %q is not registered", name)
	}
	return handler(context.Background(), input)
}

func TestReadOnlyModeHidesWriteTools(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, true, &recorded)

	for _, name := range []string{"dooray_messenger", "dooray_calendar_post_event", "dooray_post_log_create", "dooray_post_log_update"} {
		if _, ok := registry.Handlers[name]; ok {
			t.Errorf("%s must be hidden in read-only mode", name)
		}
	}
	for _, name := range []string{"dooray_posts", "dooray_post_file_download", "os"} {
		if _, ok := registry.Handlers[name]; !ok {
			t.Errorf("%s must stay visible in read-only mode", name)
		}
	}
	if len(registry.Tools) != len(registry.Handlers) {
		t.Errorf("listed %d tools but registered %d handlers", len(registry.Tools), len(registry.Handlers))
	}
}

func TestFullModeExposesEveryTool(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if len(registry.Tools) != len(Definitions()) {
		t.Fatalf("listed %d tools, want %d", len(registry.Tools), len(Definitions()))
	}
	for _, tool := range Definitions() {
		if _, ok := registry.Handlers[tool.Name]; !ok {
			t.Errorf("%s has no handler", tool.Name)
		}
	}
}

func TestOperationMismatchIsRejected(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	_, err := call(t, registry, "dooray_posts", map[string]any{"operation": "wrong", "projectId": "1"})
	if err == nil || err.Error() != "operation must be find_posts" {
		t.Errorf("error = %v", err)
	}
}

func TestPostsSkipsEmptyOptionalFilters(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_posts", map[string]any{
		"operation": "find_posts",
		"projectId": "3157232",
		"page":      float64(0),
		"size":      float64(100),
		"subjects":  "",
		"order":     "-postUpdatedAt",
		"tagIds":    nil,
	}); err != nil {
		t.Fatalf("call: %v", err)
	}

	if recorded.path != "/project/v1/projects/3157232/posts" {
		t.Errorf("path = %q", recorded.path)
	}
	if recorded.query != "order=-postUpdatedAt&page=0&size=100" {
		t.Errorf("query = %q", recorded.query)
	}
}

func TestProjectAppliesPageAndSizeDefaults(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_project", map[string]any{
		"operation": "find_projects",
		"type":      "private",
		"scope":     "private",
		"state":     "active",
	}); err != nil {
		t.Fatalf("call: %v", err)
	}

	if recorded.query != "member=me&page=0&scope=private&size=100&state=active&type=private" {
		t.Errorf("query = %q", recorded.query)
	}
}

func TestProjectFindProjectRequiresProjectID(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_project", map[string]any{"operation": "find_project"}); err == nil {
		t.Fatal("expected an error")
	}

	if _, err := call(t, registry, "dooray_project", map[string]any{"operation": "find_project", "projectId": "42"}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if recorded.path != "/project/v1/projects/42" {
		t.Errorf("path = %q", recorded.path)
	}
}

func TestPostLogCreateWrapsBody(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_post_log_create", map[string]any{
		"operation": "create_log",
		"projectId": "1",
		"postId":    "2",
		"confirm":   true,
		"body": map[string]any{
			"mimeType": "text/x-markdown",
			"content":  "hello",
		},
	}); err != nil {
		t.Fatalf("call: %v", err)
	}

	if recorded.method != http.MethodPost {
		t.Errorf("method = %q", recorded.method)
	}
	if recorded.path != "/project/v1/projects/1/posts/2/logs" {
		t.Errorf("path = %q", recorded.path)
	}
	if recorded.body != `{"body":{"content":"hello","mimeType":"text/x-markdown"}}` {
		t.Errorf("body = %q", recorded.body)
	}
}

func TestPostLogUpdateUsesPut(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_post_log_update", map[string]any{
		"operation": "update_log",
		"projectId": "1",
		"postId":    "2",
		"logId":     "3",
		"confirm":   true,
		"body":      map[string]any{"mimeType": "text/x-markdown", "content": "edited"},
	}); err != nil {
		t.Fatalf("call: %v", err)
	}

	if recorded.method != http.MethodPut {
		t.Errorf("method = %q", recorded.method)
	}
	if recorded.path != "/project/v1/projects/1/posts/2/logs/3" {
		t.Errorf("path = %q", recorded.path)
	}
}

func TestPostLogCreateRejectsMissingBody(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	_, err := call(t, registry, "dooray_post_log_create", map[string]any{
		"operation": "create_log", "projectId": "1", "postId": "2", "confirm": true,
	})
	if err == nil || !strings.Contains(err.Error(), "body must be an object") {
		t.Errorf("error = %v", err)
	}
}

func TestCalendarEventsRequiresTimeRange(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_calendar_events", map[string]any{"operation": "find_events"}); err == nil {
		t.Fatal("expected an error for the missing time range")
	}

	if _, err := call(t, registry, "dooray_calendar_events", map[string]any{
		"operation": "find_events",
		"timeMin":   "2025-04-11T00:00:00+09:00",
		"timeMax":   "2025-04-12T00:00:00+09:00",
	}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if recorded.path != "/calendar/v1/calendars/*/events" {
		t.Errorf("path = %q", recorded.path)
	}
	if strings.Contains(recorded.query, "calendars=") {
		t.Errorf("empty calendars must be omitted: %q", recorded.query)
	}
}

func TestCalendarPostEventDefaultsAndRecurrence(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_calendar_post_event", map[string]any{
		"operation":           "create_event",
		"confirm":             true,
		"subject":             "standup",
		"content":             "<p>daily</p>",
		"startedAt":           "2025-04-11T09:00:00+09:00",
		"endedAt":             "2025-04-11T09:15:00+09:00",
		"recurrenceFrequency": "weekly",
		"recurrenceByday":     "MO,TU",
	}); err != nil {
		t.Fatalf("call: %v", err)
	}

	if recorded.path != "/calendar/v1/calendars/%2A/events" && recorded.path != "/calendar/v1/calendars/*/events" {
		t.Errorf("path = %q", recorded.path)
	}

	var sent map[string]any
	if err := json.Unmarshal([]byte(recorded.body), &sent); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if sent["wholeDayFlag"] != false {
		t.Errorf("wholeDayFlag = %v", sent["wholeDayFlag"])
	}
	rule, ok := sent["recurrenceRule"].(map[string]any)
	if !ok {
		t.Fatalf("recurrenceRule missing: %v", sent)
	}
	if rule["interval"] != float64(1) || rule["timezoneName"] != "Asia/Seoul" || rule["byday"] != "MO,TU" {
		t.Errorf("recurrenceRule = %v", rule)
	}
}

func TestCalendarPostEventOmitsRecurrenceWhenUnset(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_calendar_post_event", map[string]any{
		"operation": "create_event",
		"confirm":   true,
		"subject":   "one off",
		"content":   "x",
		"startedAt": "2025-04-11T09:00:00+09:00",
		"endedAt":   "2025-04-11T09:15:00+09:00",
	}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if strings.Contains(recorded.body, "recurrenceRule") {
		t.Errorf("body = %q", recorded.body)
	}
}

func TestAccountMembersSendsNameAndUserCode(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_account_members", map[string]any{
		"operation":   "find_member_id",
		"member_name": "김민석",
		"user_code":   "minseoky",
	}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if recorded.path != "/common/v1/members" {
		t.Errorf("path = %q", recorded.path)
	}
	if !strings.Contains(recorded.query, "userCode=minseoky") || !strings.Contains(recorded.query, "name=") {
		t.Errorf("query = %q", recorded.query)
	}
}

func TestMessengerSendsDirectMessage(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	if _, err := call(t, registry, "dooray_messenger", map[string]any{
		"operation": "send",
		"confirm":   true,
		"to":        "12345",
		"message":   "hi",
	}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if recorded.path != "/messenger/v1/channels/direct-send" {
		t.Errorf("path = %q", recorded.path)
	}
	if recorded.body != `{"organizationMemberId":"12345","text":"hi"}` {
		t.Errorf("body = %q", recorded.body)
	}
}

func TestOSReturnsFormattedLocalTime(t *testing.T) {
	var recorded capture
	registry := newRegistry(t, false, &recorded)

	text, err := call(t, registry, "os", map[string]any{"operation": "get_date_time"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}

	var result struct {
		Time string `json:"time"`
	}
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := time.ParseInLocation("2006-01-02 15:04:05", result.Time, time.Local); err != nil {
		t.Errorf("time = %q: %v", result.Time, err)
	}
}

// writeToolCalls are minimal valid arguments for every write tool, without the
// confirmation flag.
func writeToolCalls() map[string]map[string]any {
	return map[string]map[string]any{
		"dooray_messenger": {
			"operation": "send", "to": "12345", "message": "hi",
		},
		"dooray_calendar_post_event": {
			"operation": "create_event", "subject": "s", "content": "c",
			"startedAt": "2025-04-11T09:00:00+09:00", "endedAt": "2025-04-11T09:15:00+09:00",
		},
		"dooray_post_log_create": {
			"operation": "create_log", "projectId": "1", "postId": "2",
			"body": map[string]any{"mimeType": "text/x-markdown", "content": "hello"},
		},
		"dooray_post_log_update": {
			"operation": "update_log", "projectId": "1", "postId": "2", "logId": "3",
			"body": map[string]any{"mimeType": "text/x-markdown", "content": "edited"},
		},
	}
}

func TestWriteToolsRequireConfirmation(t *testing.T) {
	for name, input := range writeToolCalls() {
		// A missing confirm, an explicitly false one, and a non-boolean all
		// have to be refused before any request reaches Dooray.
		for _, confirm := range []any{nil, false, "true", float64(1)} {
			var recorded capture
			registry := newRegistry(t, false, &recorded)

			arguments := map[string]any{}
			for key, value := range input {
				arguments[key] = value
			}
			if confirm != nil {
				arguments["confirm"] = confirm
			}

			_, err := call(t, registry, name, arguments)
			if err == nil {
				t.Errorf("%s with confirm=%#v was executed", name, confirm)
				continue
			}
			if !strings.Contains(err.Error(), "confirm must be true") {
				t.Errorf("%s with confirm=%#v: error = %v", name, confirm, err)
			}
			if recorded.method != "" {
				t.Errorf("%s with confirm=%#v reached Dooray: %s %s", name, confirm, recorded.method, recorded.path)
			}
		}
	}
}

func TestWriteToolsRunWithConfirmation(t *testing.T) {
	for name, input := range writeToolCalls() {
		var recorded capture
		registry := newRegistry(t, false, &recorded)

		arguments := map[string]any{"confirm": true}
		for key, value := range input {
			arguments[key] = value
		}

		if _, err := call(t, registry, name, arguments); err != nil {
			t.Errorf("%s with confirm=true: %v", name, err)
			continue
		}
		if recorded.method == "" {
			t.Errorf("%s with confirm=true did not reach Dooray", name)
		}
		if strings.Contains(recorded.body, "confirm") {
			t.Errorf("%s forwarded the confirmation flag to Dooray: %s", name, recorded.body)
		}
	}
}

func TestWriteToolSchemasRequireConfirm(t *testing.T) {
	for _, tool := range Definitions() {
		if !IsWriteTool(tool.Name) {
			continue
		}

		schema := tool.InputSchema.(map[string]any)
		required := schema["required"].([]string)
		found := false
		for _, name := range required {
			if name == "confirm" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s schema does not require confirm: %v", tool.Name, required)
		}

		properties := schema["properties"].(jsonschema.Properties)
		declared := false
		for _, property := range properties {
			if property.Name == "confirm" {
				declared = true
			}
		}
		if !declared {
			t.Errorf("%s schema does not declare a confirm property", tool.Name)
		}
	}
}

func TestReadToolsDoNotRequireConfirm(t *testing.T) {
	for _, tool := range Definitions() {
		if IsWriteTool(tool.Name) {
			continue
		}
		for _, name := range tool.InputSchema.(map[string]any)["required"].([]string) {
			if name == "confirm" {
				t.Errorf("read tool %s must not require confirm", tool.Name)
			}
		}
	}
}
