package wsserver

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"llsoai-websocket/server/internal/protocol"
)

// Subscriber 表示一个 workspace 级 SSE 订阅者。
// 一个用户的同一个 workspace 可能同时有多个浏览器 tab 订阅。
type Subscriber struct {
	ID          uint64
	UserID      uint64
	WorkspaceID string
	Ch          chan protocol.Envelope
	closed      atomic.Bool
	mu          sync.Mutex
}

// Send 向订阅者推送消息；通道满则丢弃并返回 false。
func (s *Subscriber) Send(msg protocol.Envelope) bool {
	if s.closed.Load() {
		return false
	}
	select {
	case s.Ch <- msg:
		return true
	default:
		return false
	}
}

// Close 安全关闭通道，可多次调用。
func (s *Subscriber) Close() {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	close(s.Ch)
}

type subscriptionState struct {
	mu         sync.RWMutex
	subs       map[string]map[uint64]*Subscriber // key = "userId:workspaceId"
	nextID     uint64
	bufferSize int
}

func newSubscriptionState(bufferSize int) *subscriptionState {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	return &subscriptionState{
		subs:       map[string]map[uint64]*Subscriber{},
		bufferSize: bufferSize,
	}
}

func subKey(userID uint64, workspaceID string) string {
	return formatUserKey(userID) + ":" + workspaceID
}

func formatUserKey(userID uint64) string {
	// 简单避免引入 strconv 多次分配，使用最朴素拼接
	const digits = "0123456789"
	if userID == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for userID > 0 {
		i--
		buf[i] = digits[userID%10]
		userID /= 10
	}
	return string(buf[i:])
}

// Subscribe 创建并注册一个新的订阅者。
func (h *Hub) Subscribe(userID uint64, workspaceID string) *Subscriber {
	if h.subscriptions == nil {
		h.subscriptions = newSubscriptionState(64)
	}
	id := atomic.AddUint64(&h.subscriptions.nextID, 1)
	sub := &Subscriber{
		ID:          id,
		UserID:      userID,
		WorkspaceID: workspaceID,
		Ch:          make(chan protocol.Envelope, h.subscriptions.bufferSize),
	}
	key := subKey(userID, workspaceID)
	h.subscriptions.mu.Lock()
	if _, ok := h.subscriptions.subs[key]; !ok {
		h.subscriptions.subs[key] = map[uint64]*Subscriber{}
	}
	h.subscriptions.subs[key][id] = sub
	count := len(h.subscriptions.subs[key])
	h.subscriptions.mu.Unlock()
	log.Printf("[sub] subscribe userId=%d workspaceId=%s subId=%d total=%d", userID, workspaceID, id, count)
	return sub
}

// Unsubscribe 移除订阅者并关闭其通道。
func (h *Hub) Unsubscribe(sub *Subscriber) {
	if sub == nil || h.subscriptions == nil {
		return
	}
	key := subKey(sub.UserID, sub.WorkspaceID)
	h.subscriptions.mu.Lock()
	if m, ok := h.subscriptions.subs[key]; ok {
		if _, exists := m[sub.ID]; exists {
			delete(m, sub.ID)
		}
		if len(m) == 0 {
			delete(h.subscriptions.subs, key)
		}
	}
	remaining := 0
	if m, ok := h.subscriptions.subs[key]; ok {
		remaining = len(m)
	}
	h.subscriptions.mu.Unlock()
	sub.Close()
	log.Printf("[sub] unsubscribe userId=%d workspaceId=%s subId=%d remaining=%d", sub.UserID, sub.WorkspaceID, sub.ID, remaining)
}

// Broadcast 将消息广播给指定 user+workspace 下所有订阅者。
// 返回成功投递的数量。
func (h *Hub) Broadcast(userID uint64, workspaceID string, msg protocol.Envelope) int {
	if h.subscriptions == nil {
		return 0
	}
	key := subKey(userID, workspaceID)
	h.subscriptions.mu.RLock()
	m := h.subscriptions.subs[key]
	subs := make([]*Subscriber, 0, len(m))
	for _, s := range m {
		subs = append(subs, s)
	}
	h.subscriptions.mu.RUnlock()
	delivered := 0
	for _, s := range subs {
		if s.Send(msg) {
			delivered++
		} else {
			log.Printf("[sub] drop msg userId=%d workspaceId=%s subId=%d type=%s (channel full or closed)", userID, workspaceID, s.ID, msg.Type)
		}
	}
	return delivered
}

// HasSubscribers 是否存在订阅者。
func (h *Hub) HasSubscribers(userID uint64, workspaceID string) bool {
	if h.subscriptions == nil {
		return false
	}
	key := subKey(userID, workspaceID)
	h.subscriptions.mu.RLock()
	defer h.subscriptions.mu.RUnlock()
	return len(h.subscriptions.subs[key]) > 0
}

// 保留一个空函数避免 time 包未来失引导致编译失败（订阅生命周期由外层 SSE handler 控制）。
var _ = time.Now
