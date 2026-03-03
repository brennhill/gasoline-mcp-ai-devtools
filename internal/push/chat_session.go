// chat_session.go — Bounded conversation store with pub/sub for SSE delivery.
package push

import (
	"encoding/json"
	"sync"
	"time"
)

// maxChatMessages is the maximum number of messages per session before FIFO eviction.
const maxChatMessages = 100

// subscriberBufferSize is the channel buffer for subscriber delivery.
// Sized to absorb short bursts without dropping messages.
const subscriberBufferSize = 16

// ChatRole is the role of a chat message sender.
type ChatRole string

const (
	ChatRoleUser       ChatRole = "user"
	ChatRoleAssistant  ChatRole = "assistant"
	ChatRoleAnnotation ChatRole = "annotation"
)

// ChatMessage represents a single message in a chat conversation.
type ChatMessage struct {
	Role           ChatRole        `json:"role"`
	Text           string          `json:"text"`
	Timestamp      int64           `json:"timestamp"`
	ConversationID string          `json:"conversation_id"`
	Annotations    json.RawMessage `json:"annotations,omitempty"`
}

// ChatSession manages a bounded conversation with pub/sub for live subscribers.
type ChatSession struct {
	mu             sync.RWMutex
	conversationID string
	messages       []ChatMessage
	subscribers    map[int]chan ChatMessage
	nextSubID      int
	closed         bool
}

// NewChatSession creates a new chat session with the given conversation ID.
func NewChatSession(conversationID string) *ChatSession {
	return &ChatSession{
		conversationID: conversationID,
		messages:       make([]ChatMessage, 0, maxChatMessages),
		subscribers:    make(map[int]chan ChatMessage),
	}
}

// ConversationID returns the session's conversation ID.
func (cs *ChatSession) ConversationID() string {
	return cs.conversationID
}

// AddMessage appends a message and notifies all subscribers. Evicts oldest if over cap.
func (cs *ChatSession) AddMessage(msg ChatMessage) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.closed {
		return
	}

	// Auto-fill timestamp if not set
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}
	// Auto-fill conversation ID
	msg.ConversationID = cs.conversationID

	// Evict oldest if at capacity — copy to fresh slice to release old backing array
	if len(cs.messages) >= maxChatMessages {
		fresh := make([]ChatMessage, maxChatMessages-1, maxChatMessages)
		copy(fresh, cs.messages[1:])
		cs.messages = fresh
	}
	cs.messages = append(cs.messages, msg)

	// Notify subscribers (non-blocking send to buffered channels)
	for _, ch := range cs.subscribers {
		select {
		case ch <- msg:
		default:
			// Slow subscriber — drop message to avoid blocking
		}
	}
}

// Subscribe returns a channel that receives new messages and an unsubscribe function.
// The channel is buffered to absorb bursts without blocking AddMessage.
func (cs *ChatSession) Subscribe() (<-chan ChatMessage, func()) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	ch := make(chan ChatMessage, subscriberBufferSize)
	if cs.closed {
		close(ch)
		return ch, func() {}
	}

	id := cs.nextSubID
	cs.nextSubID++
	cs.subscribers[id] = ch

	return ch, func() {
		cs.mu.Lock()
		defer cs.mu.Unlock()
		if _, ok := cs.subscribers[id]; ok {
			delete(cs.subscribers, id)
			close(ch)
		}
	}
}

// Messages returns a snapshot copy of all messages.
func (cs *ChatSession) Messages() []ChatMessage {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if len(cs.messages) == 0 {
		return nil
	}
	out := make([]ChatMessage, len(cs.messages))
	copy(out, cs.messages)
	return out
}

// Close marks the session as closed and closes all subscriber channels.
func (cs *ChatSession) Close() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.closed {
		return
	}
	cs.closed = true

	for id, ch := range cs.subscribers {
		close(ch)
		delete(cs.subscribers, id)
	}
}
