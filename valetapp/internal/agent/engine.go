// Package agent provides the ADK-powered AI agent engine for Valet.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/mcptoolset"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"google.golang.org/genai"

	adkopenai "github.com/byebyebruce/adk-go-openai"
)

const (
	appName   = "valet"
	userID    = "valet-user"
	sessionID = "session-1"

	systemPrompt = `You are Valet's admin assistant. Valet is a local development reverse proxy that gives developers trusted HTTPS on custom domain names.

How Valet works:
- A "TLD" (e.g., .test, example.com) is a managed DNS namespace. TLDs are registered via the CLI with sudo (valetd tld add --tld <name>) and cannot be added through tools. You can list them.
- A "DNS entry" is a subdomain registered within a TLD (e.g., app.example.com under example.com). DNS entries resolve to 127.0.0.1 by default, or to a custom IP or hostname (CNAME). You CAN add and remove DNS entries.
- A "route" maps a domain to a local upstream service (e.g., app.example.com → localhost:3000). Routes configure the Caddy reverse proxy with trusted HTTPS certificates. You CAN add, update, remove, and diagnose routes.
- Route templates (simple, spa-api, websocket, cors-proxy, multi-upstream) provide common configurations.
- The diagnose tool checks DNS, upstream connectivity, and HTTP health for a route.

Typical workflow: the user registers a TLD via CLI, then uses you to add DNS entries and routes within it.

Be concise and helpful. When adding a route, always confirm the domain and upstream with the user.`
)

// Engine manages ADK agent runs with persistent session history.
type Engine struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	sessSvc session.Service
}

// RunConfig holds everything needed for an agent run.
type RunConfig struct {
	ModelBaseURL string
	ModelID      string
	APIKey       string
	UserMessage  string
	OnToken      func(text string)
	OnToolCall   func(name string, args string)
	OnToolResult func(name string, result string)
	OnComplete   func(content string)
	OnError      func(err error)
}

// NewEngine creates a new agent engine with persistent session storage.
func NewEngine() *Engine {
	return &Engine{
		cancels: make(map[string]context.CancelFunc),
		sessSvc: session.InMemoryService(),
	}
}

// ClearHistory resets the conversation by creating a new session service.
func (e *Engine) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessSvc = session.InMemoryService()
}

// Run executes an agent turn through the ADK runner, using persistent session history.
func (e *Engine) Run(ctx context.Context, cfg RunConfig) error {
	ctx, cancel := context.WithCancel(ctx)

	e.mu.Lock()
	e.cancels[sessionID] = cancel
	e.mu.Unlock()

	defer func() {
		cancel()
		e.mu.Lock()
		delete(e.cancels, sessionID)
		e.mu.Unlock()
	}()

	// Create the OpenAI-compatible model via the adapter
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = "no-key"
	}
	oaiCfg := openai.DefaultConfig(apiKey)
	oaiCfg.BaseURL = cfg.ModelBaseURL
	model := adkopenai.NewOpenAIModel(cfg.ModelID, oaiCfg)

	// Create MCP toolset connecting to valetd
	mcpToolset, err := mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.StreamableClientTransport{
			Endpoint: "http://localhost:7801/",
		},
	})
	if err != nil {
		cfg.OnError(fmt.Errorf("connect to valetd MCP: %w", err))
		return err
	}

	// Create the agent
	a, err := llmagent.New(llmagent.Config{
		Name:        "valet-assistant",
		Model:       model,
		Description: "Valet admin assistant",
		Instruction: systemPrompt,
		Toolsets:    []tool.Toolset{mcpToolset},
	})
	if err != nil {
		cfg.OnError(fmt.Errorf("create agent: %w", err))
		return err
	}

	// Create the runner with the persistent session service
	r, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             a,
		SessionService:    e.sessSvc,
		AutoCreateSession: true,
	})
	if err != nil {
		cfg.OnError(fmt.Errorf("create runner: %w", err))
		return err
	}

	// Build the user message content
	userContent := genai.NewContentFromText(cfg.UserMessage, "user")

	// Run the agent
	var accumulatedContent string
	for event, err := range r.Run(ctx, userID, sessionID, userContent, agent.RunConfig{
		StreamingMode: agent.StreamingModeSSE,
	}) {
		if err != nil {
			cfg.OnError(err)
			return err
		}
		if event == nil || event.Content == nil {
			continue
		}

		for _, part := range event.Content.Parts {
			if part == nil {
				continue
			}

			if part.Text != "" && !part.Thought {
				if event.Partial {
					cfg.OnToken(part.Text)
				} else {
					accumulatedContent += part.Text
				}
			}

			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				cfg.OnToolCall(part.FunctionCall.Name, string(argsJSON))
			}

			if part.FunctionResponse != nil {
				resp, _ := json.Marshal(part.FunctionResponse.Response)
				cfg.OnToolResult(part.FunctionResponse.Name, string(resp))
			}
		}
	}

	cfg.OnComplete(accumulatedContent)
	return nil
}

// Stop cancels an in-progress generation.
func (e *Engine) Stop(id string) {
	e.mu.Lock()
	if cancel, ok := e.cancels[id]; ok {
		cancel()
		delete(e.cancels, id)
	}
	e.mu.Unlock()
}
