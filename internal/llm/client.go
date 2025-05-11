package llm

import "context"

type Client interface {
	Chat(context.Context, *ChatRequest) (*ChatResponse, error)
}

type ChatRequest struct {
	Messages []Message
}

type ChatResponse struct {
	Message Message
}

type Message struct {
	Role    Role
	Content string
}

type Role string

const (
	// RoleSystem specifies that the message is from the system iteself.
	RoleSystem Role = "system"
	// RoleAssistant specifies that the message is from the assistant / LLM.
	RoleAssistant Role = "assistant"
	// RoleUser specifies that the message is from an end-user.
	RoleUser Role = "user"
)
