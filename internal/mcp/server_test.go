package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func run(t *testing.T, input string, tools []Tool, handlers map[string]Handler) []map[string]any {
	t.Helper()

	var output bytes.Buffer
	server := NewServer("dooray", "0.1.0", tools, handlers, &output)
	if err := server.Serve(context.Background(), strings.NewReader(input)); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	var messages []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if line == "" {
			continue
		}
		var message map[string]any
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		messages = append(messages, message)
	}
	return messages
}

func byID(messages []map[string]any, id float64) map[string]any {
	for _, message := range messages {
		if value, ok := message["id"].(float64); ok && value == id {
			return message
		}
	}
	return nil
}

func TestInitializeEchoesProtocolVersion(t *testing.T) {
	messages := run(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`+"\n", nil, nil)

	result := byID(messages, 1)["result"].(map[string]any)
	if result["protocolVersion"] != "2025-06-18" {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
	serverInfo := result["serverInfo"].(map[string]any)
	if serverInfo["name"] != "dooray" {
		t.Errorf("serverInfo = %v", serverInfo)
	}
}

func TestInitializeFallsBackToDefaultProtocolVersion(t *testing.T) {
	messages := run(t, `{"jsonrpc":"2.0","id":1,"method":"initialize"}`+"\n", nil, nil)

	result := byID(messages, 1)["result"].(map[string]any)
	if result["protocolVersion"] != defaultProtocolVersion {
		t.Errorf("protocolVersion = %v", result["protocolVersion"])
	}
}

func TestNotificationsProduceNoOutput(t *testing.T) {
	messages := run(t, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"+
		`{"jsonrpc":"2.0","method":"ping"}`+"\n", nil, nil)

	if len(messages) != 0 {
		t.Errorf("expected no output, got %v", messages)
	}
}

func TestCRLFAndBlankLinesAreTolerated(t *testing.T) {
	messages := run(t, "\r\n"+`{"jsonrpc":"2.0","id":7,"method":"ping"}`+"\r\n\r\n", nil, nil)

	if len(messages) != 1 || byID(messages, 7) == nil {
		t.Errorf("messages = %v", messages)
	}
}

func TestParseErrorUsesNullID(t *testing.T) {
	messages := run(t, "not json\n", nil, nil)

	if len(messages) != 1 {
		t.Fatalf("messages = %v", messages)
	}
	if messages[0]["id"] != nil {
		t.Errorf("id = %v, want null", messages[0]["id"])
	}
	rpcError := messages[0]["error"].(map[string]any)
	if rpcError["code"] != float64(-32700) {
		t.Errorf("code = %v", rpcError["code"])
	}
}

func TestUnknownMethodReturnsMethodNotFound(t *testing.T) {
	messages := run(t, `{"jsonrpc":"2.0","id":2,"method":"nope"}`+"\n", nil, nil)

	rpcError := byID(messages, 2)["error"].(map[string]any)
	if rpcError["code"] != float64(-32601) || rpcError["message"] != "method not found: nope" {
		t.Errorf("error = %v", rpcError)
	}
}

func TestNonJSONRPCMessageIsIgnored(t *testing.T) {
	messages := run(t, `{"id":1,"method":"ping"}`+"\n", nil, nil)

	if len(messages) != 0 {
		t.Errorf("messages = %v", messages)
	}
}

func TestToolCallWrapsTextContent(t *testing.T) {
	handlers := map[string]Handler{
		"echo": func(ctx context.Context, input map[string]any) (string, error) {
			return input["value"].(string), nil
		},
	}
	messages := run(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"value":"hi"}}}`+"\n",
		[]Tool{{Name: "echo"}}, handlers)

	result := byID(messages, 3)["result"].(map[string]any)
	content := result["content"].([]any)[0].(map[string]any)
	if content["type"] != "text" || content["text"] != "hi" {
		t.Errorf("content = %v", content)
	}
}

func TestToolCallErrorBecomesRPCError(t *testing.T) {
	handlers := map[string]Handler{
		"boom": func(ctx context.Context, input map[string]any) (string, error) {
			return "", errors.New("operation must be send")
		},
	}
	messages := run(t, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"boom"}}`+"\n",
		[]Tool{{Name: "boom"}}, handlers)

	rpcError := byID(messages, 4)["error"].(map[string]any)
	if rpcError["code"] != float64(-32000) || rpcError["message"] != "operation must be send" {
		t.Errorf("error = %v", rpcError)
	}
}

func TestHiddenToolIsNotCallable(t *testing.T) {
	messages := run(t, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"dooray_messenger"}}`+"\n", nil, nil)

	rpcError := byID(messages, 5)["error"].(map[string]any)
	if rpcError["message"] != "tool is not available: dooray_messenger" {
		t.Errorf("error = %v", rpcError)
	}
}

func TestEmptyListsAreSerializedAsArrays(t *testing.T) {
	messages := run(t, `{"jsonrpc":"2.0","id":6,"method":"resources/list"}`+"\n"+
		`{"jsonrpc":"2.0","id":7,"method":"prompts/list"}`+"\n", nil, nil)

	if resources := byID(messages, 6)["result"].(map[string]any)["resources"]; len(resources.([]any)) != 0 {
		t.Errorf("resources = %v", resources)
	}
	if prompts := byID(messages, 7)["result"].(map[string]any)["prompts"]; len(prompts.([]any)) != 0 {
		t.Errorf("prompts = %v", prompts)
	}
}
