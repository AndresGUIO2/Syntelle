package speech

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

const maxActiveVoiceUsers = 50

var errVoiceAtCapacity = errors.New("voice capacity is full")

// voiceGate counts distinct active conversation users. Concurrent phrases from
// one conversation share a lease; a direct speech request gets its own lease.
type voiceGate struct {
	mu        sync.Mutex
	active    map[string]int
	anonymous uint64
}

func (g *voiceGate) acquire(_ context.Context, conversationID string) (func(), error) {
	g.mu.Lock()
	if g.active == nil {
		g.active = make(map[string]int)
	}
	key := "conversation:" + conversationID
	if conversationID == "" {
		g.anonymous++
		key = fmt.Sprintf("request:%d", g.anonymous)
	}
	if g.active[key] == 0 && len(g.active) >= maxActiveVoiceUsers {
		g.mu.Unlock()
		return nil, errVoiceAtCapacity
	}
	g.active[key]++
	g.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			g.active[key]--
			if g.active[key] == 0 {
				delete(g.active, key)
			}
			g.mu.Unlock()
		})
	}, nil
}

func (g *voiceGate) count() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.active)
}
