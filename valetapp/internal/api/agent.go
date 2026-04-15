package api

import (
	"context"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/loaapp/valet/valetapp/internal/agent"
)

// AgentService is the Wails-bound service for the AI agent.
type AgentService struct {
	ctx    context.Context
	engine *agent.Engine
}

// NewAgentService creates a new AgentService.
func NewAgentService() *AgentService {
	return &AgentService{
		engine: agent.NewEngine(),
	}
}

// SetContext sets the Wails runtime context.
func (s *AgentService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// SendMessage runs the agent with the given message and model config.
func (s *AgentService) SendMessage(modelBaseURL, modelID, apiKey, message string) error {
	go s.runAgent(modelBaseURL, modelID, apiKey, message)
	return nil
}

// StopGeneration cancels the current agent run.
func (s *AgentService) StopGeneration() error {
	s.engine.Stop("session-1")
	return nil
}

func (s *AgentService) runAgent(modelBaseURL, modelID, apiKey, message string) {
	s.engine.Run(context.Background(), agent.RunConfig{
		ModelBaseURL: modelBaseURL,
		ModelID:      modelID,
		APIKey:       apiKey,
		UserMessage:  message,

		OnToken: func(text string) {
			wailsrt.EventsEmit(s.ctx, "agent:token", map[string]any{
				"text": text,
			})
		},

		OnToolCall: func(name string, args string) {
			wailsrt.EventsEmit(s.ctx, "agent:toolcall", map[string]any{
				"name": name,
				"args": args,
			})
		},

		OnToolResult: func(name string, result string) {
			wailsrt.EventsEmit(s.ctx, "agent:toolresult", map[string]any{
				"name":   name,
				"result": result,
			})
		},

		OnComplete: func(content string) {
			wailsrt.EventsEmit(s.ctx, "agent:done", map[string]any{
				"content": content,
			})
		},

		OnError: func(err error) {
			wailsrt.EventsEmit(s.ctx, "agent:error", map[string]any{
				"error": err.Error(),
			})
		},
	})
}
