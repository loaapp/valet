package api

import (
	"context"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/loaapp/valet/valetapp/internal/agent"
	"github.com/loaapp/valet/valetapp/internal/conversations"
)

// AgentService is the Wails-bound service for the AI agent.
type AgentService struct {
	ctx    context.Context
	engine *agent.Engine
	store  *conversations.Store
}

// NewAgentService creates a new AgentService with its own conversation store.
func NewAgentService(store *conversations.Store) *AgentService {
	return &AgentService{
		engine: agent.NewEngine(),
		store:  store,
	}
}

// SetContext sets the Wails runtime context.
func (s *AgentService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// GetHistory loads the current session messages from SQLite.
func (s *AgentService) GetHistory() ([]conversations.Message, error) {
	messages, err := s.store.GetCurrentSession()
	if err != nil {
		return nil, err
	}
	if messages == nil {
		messages = []conversations.Message{}
	}
	return messages, nil
}

// SendMessage persists the user message, then runs the agent.
func (s *AgentService) SendMessage(modelBaseURL, modelID, apiKey, message string) error {
	s.store.Push("user", message, "", "", "")
	go s.runAgent(modelBaseURL, modelID, apiKey, message)
	return nil
}

// StopGeneration cancels the current agent run.
func (s *AgentService) StopGeneration() error {
	s.engine.Stop("session-1")
	return nil
}

// ClearHistory inserts a tombstone and resets the ADK in-memory session.
func (s *AgentService) ClearHistory() {
	s.store.InsertTombstone()
	s.engine.ClearHistory()
}

func (s *AgentService) runAgent(modelBaseURL, modelID, apiKey, message string) {
	s.engine.Run(context.Background(), agent.RunConfig{
		ModelBaseURL: modelBaseURL,
		ModelID:      modelID,
		APIKey:       apiKey,
		UserMessage:  message,

		OnToken: func(text string) {
			wailsrt.EventsEmit(s.ctx, "agent:token", map[string]any{"text": text})
		},

		OnToolCall: func(name string, args string) {
			s.store.Push("toolcall", "", name, args, "")
			wailsrt.EventsEmit(s.ctx, "agent:toolcall", map[string]any{"name": name, "args": args})
		},

		OnToolResult: func(name string, result string) {
			s.store.Push("toolresult", result, name, "", result)
			wailsrt.EventsEmit(s.ctx, "agent:toolresult", map[string]any{"name": name, "result": result})
		},

		OnComplete: func(content string) {
			s.store.Push("assistant", content, "", "", "")
			wailsrt.EventsEmit(s.ctx, "agent:done", map[string]any{"content": content})
		},

		OnError: func(err error) {
			s.store.Push("error", err.Error(), "", "", "")
			wailsrt.EventsEmit(s.ctx, "agent:error", map[string]any{"error": err.Error()})
		},
	})
}
