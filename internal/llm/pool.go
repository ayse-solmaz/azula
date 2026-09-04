package llm

import (
	"context"
	"fmt"
	"sync"

	"github.com/ayse-solmaz/azula/internal/domain"
)

type Slot struct {
	ID    int
	Depth int
}

type Pool struct {
	mu    sync.Mutex
	slots []Slot
	sem   chan struct{}
}

func NewPool(n int) *Pool {
	if n < 1 {
		n = 5
	}
	slots := make([]Slot, n)
	for i := range slots {
		slots[i].ID = i
	}
	return &Pool{
		slots: slots,
		sem:   make(chan struct{}, n),
	}
}

func (p *Pool) Acquire(ctx context.Context) (int, error) {
	select {
	case p.sem <- struct{}{}:
		p.mu.Lock()
		defer p.mu.Unlock()
		best := 0
		for i := range p.slots {
			if p.slots[i].Depth < p.slots[best].Depth {
				best = i
			}
		}
		p.slots[best].Depth++
		return p.slots[best].ID, nil
	case <-ctx.Done():
		return -1, fmt.Errorf("%w: %v", domain.ErrBusy, ctx.Err())
	}
}

func (p *Pool) Release(id int) {
	p.mu.Lock()
	if id >= 0 && id < len(p.slots) && p.slots[id].Depth > 0 {
		p.slots[id].Depth--
	}
	p.mu.Unlock()
	<-p.sem
}
