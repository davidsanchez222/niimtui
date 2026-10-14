package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func mcpClient(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	s := &mcpServer{configPath: testLabelConfig(t)}
	t.Cleanup(s.close)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := s.server().Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return result
}

func TestMCPListDesignsReturnsBindings(t *testing.T) {
	result := callTool(t, mcpClient(t), "list_designs", map[string]any{})
	var designs []designInfo
	data, _ := json.Marshal(result.StructuredContent)
	if err := json.Unmarshal(data, &designs); err != nil {
		t.Fatal(err)
	}
	if result.IsError || len(designs) != 1 || designs[0].Name != "box" || designs[0].Bindings[0].Name != "url" {
		t.Fatalf("designs = %+v", designs)
	}
}

func TestMCPPreviewReturnsPNG(t *testing.T) {
	result := callTool(t, mcpClient(t), "preview_label", map[string]any{"design": "box", "values": map[string]string{"url": "https://homebox.example/item/1"}})
	if result.IsError {
		t.Fatalf("preview failed: %+v", result.Content)
	}
	image, ok := result.Content[0].(*mcp.ImageContent)
	if !ok || image.MIMEType != "image/png" {
		t.Fatalf("content = %+v", result.Content)
	}
	if _, err := png.Decode(bytes.NewReader(image.Data)); err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}
}

func TestMCPPrintToolsRejectBadInputBeforeConnecting(t *testing.T) {
	session := mcpClient(t)
	for _, tc := range []struct {
		tool string
		args map[string]any
		want string
	}{
		{"print_label", map[string]any{"design": "box"}, "requires --set url"},
		{"print_label", map[string]any{"design": "box", "values": map[string]string{"url": "x"}, "preset": "small"}, "LABEL_STOCK_MISMATCH"},
		{"print_labels", map[string]any{"design": "box", "rows": []map[string]string{{"url": "x"}, {"typo": "y"}}}, `"row":2`},
	} {
		result := callTool(t, session, tc.tool, tc.args)
		text := result.Content[0].(*mcp.TextContent).Text
		if !result.IsError || !strings.Contains(text, tc.want) {
			t.Fatalf("%s(%v) = %v %s, want error containing %q", tc.tool, tc.args, result.IsError, text, tc.want)
		}
	}
}
