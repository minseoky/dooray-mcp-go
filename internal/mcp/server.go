// Package mcp implements the JSON-RPC 2.0 stdio transport and the small set of
// Model Context Protocol methods this server answers.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

const defaultProtocolVersion = "2024-11-05"

// Tool is one entry of the tools/list response.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// Handler executes one tool call and returns the text content to send back.
type Handler func(ctx context.Context, input map[string]any) (string, error)

// Server wires the stdio transport to the registered tools.
type Server struct {
	name     string
	version  string
	tools    []Tool
	handlers map[string]Handler

	output    io.Writer
	outputMu  sync.Mutex
	waitGroup sync.WaitGroup
}

// NewServer builds a server exposing the given tools.
func NewServer(name, version string, tools []Tool, handlers map[string]Handler, output io.Writer) *Server {
	return &Server{
		name:     name,
		version:  version,
		tools:    tools,
		handlers: handlers,
		output:   output,
	}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

// Serve reads newline-delimited JSON-RPC messages until the input closes. Each
// request is handled on its own goroutine, matching the async JavaScript
// server, and writes to the output stream are serialized.
func (s *Server) Serve(ctx context.Context, input io.Reader) error {
	scanner := bufio.NewScanner(input)
	// MCP payloads can carry long tool descriptions and file metadata, so the
	// default 64KiB line limit is raised.
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		// Trailing \r is stripped so Windows clients writing CRLF still parse.
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var message request
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			s.sendFailure(nil, -32700, fmt.Sprintf("parse error: %s", err))
			continue
		}

		s.waitGroup.Add(1)
		go func() {
			defer s.waitGroup.Done()
			s.handle(ctx, message)
		}()
	}

	s.waitGroup.Wait()

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (s *Server) handle(ctx context.Context, message request) {
	if message.JSONRPC != "2.0" {
		return
	}

	id := message.ID
	isNotification := len(id) == 0 || string(id) == "null"

	result, err := s.dispatch(ctx, message)
	if err != nil {
		if !isNotification {
			s.sendFailure(id, err.code, err.message)
		}
		return
	}
	if result == nil || isNotification {
		return
	}

	s.sendSuccess(id, result)
}

type rpcError struct {
	code    int
	message string
}

func (s *Server) dispatch(ctx context.Context, message request) (any, *rpcError) {
	switch message.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": s.protocolVersion(message.Params),
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.name, "version": s.version},
		}, nil

	case "notifications/initialized":
		return nil, nil

	case "ping":
		return map[string]any{}, nil

	case "tools/list":
		return map[string]any{"tools": s.tools}, nil

	case "resources/list":
		return map[string]any{"resources": []any{}}, nil

	case "prompts/list":
		return map[string]any{"prompts": []any{}}, nil

	case "tools/call":
		return s.callTool(ctx, message.Params)
	}

	return nil, &rpcError{code: -32601, message: fmt.Sprintf("method not found: %s", message.Method)}
}

func (s *Server) callTool(ctx context.Context, rawParams json.RawMessage) (any, *rpcError) {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if len(rawParams) > 0 {
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &rpcError{code: -32000, message: err.Error()}
		}
	}

	handler, ok := s.handlers[params.Name]
	if !ok {
		return nil, &rpcError{code: -32000, message: fmt.Sprintf("tool is not available: %s", params.Name)}
	}

	input := params.Arguments
	if input == nil {
		input = map[string]any{}
	}

	text, err := handler(ctx, input)
	if err != nil {
		return nil, &rpcError{code: -32000, message: err.Error()}
	}

	return map[string]any{
		"content": []any{map[string]any{"type": "text", "text": text}},
	}, nil
}

func (s *Server) protocolVersion(rawParams json.RawMessage) string {
	if len(rawParams) == 0 {
		return defaultProtocolVersion
	}

	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(rawParams, &params); err == nil && params.ProtocolVersion != "" {
		return params.ProtocolVersion
	}
	return defaultProtocolVersion
}

func (s *Server) sendSuccess(id json.RawMessage, result any) {
	s.send(response{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *Server) sendFailure(id json.RawMessage, code int, message string) {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	s.send(response{JSONRPC: "2.0", ID: id, Error: &responseError{Code: code, Message: message}})
}

func (s *Server) send(message response) {
	encoded, err := json.Marshal(message)
	if err != nil {
		return
	}

	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	s.output.Write(append(encoded, '\n'))
}
