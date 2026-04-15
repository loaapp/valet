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
	appName = "valet"
	userID  = "valet-user"

	systemPrompt = "You are Valet's admin assistant. You manage local development reverse proxy routes, TLDs, and certificates. Use the available tools to list, add, update, remove, and diagnose routes. Be concise and helpful."
)

// Engine manages ADK agent runs.
type Engine struct {
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// RunConfig holds everything needed for an agent run.
type RunConfig struct {
	ModelBaseURL string // e.g., "http://localhost:11434/v1" for Ollama
	ModelID      string // e.g., "llama3.1"
	APIKey       string // optional
	UserMessage  string
	OnToken      func(text string)
	OnToolCall   func(name string, args string)
	OnToolResult func(name string, result string)
	OnComplete   func(content string)
	OnError      func(err error)
}

// NewEngine creates a new agent engine.
func NewEngine() *Engine {
	return &Engine{
		cancels: make(map[string]context.CancelFunc),
	}
}

// Run executes an agent turn through the ADK runner.
func (e *Engine) Run(ctx context.Context, cfg RunConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	sessionID := "session-1"

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

	// Create the runner with in-memory sessions
	sessSvc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:           appName,
		Agent:             a,
		SessionService:    sessSvc,
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

			// Regular text content
			if part.Text != "" && !part.Thought {
				if event.Partial {
					cfg.OnToken(part.Text)
				} else {
					accumulatedContent += part.Text
				}
			}

			// Tool calls
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Args)
				cfg.OnToolCall(part.FunctionCall.Name, string(argsJSON))
			}

			// Tool responses
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
