package eventWorker

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	UserID  string
	Handler func(Event)
	TraceID string
	Payload []any
}

type UserQueue struct {
	mu            sync.Mutex
	userQueues    map[string]chan Event
	workerPool    chan struct{}
	maxWorkers    int
	scalingTicker *time.Ticker
	stopScaling   chan bool
}

var (
	EventWorker *UserQueue
	once        sync.Once
)

func NewWorker() {
	once.Do(func() {
		EventWorker = &UserQueue{
			userQueues:    make(map[string]chan Event),
			workerPool:    make(chan struct{}, 1),
			maxWorkers:    5,
			scalingTicker: time.NewTicker(5 * time.Second),
			stopScaling:   make(chan bool),
		}
		go EventWorker.autoScale()
	})
}

func (uq *UserQueue) startWorker(userID string) {
	uq.workerPool <- struct{}{}
	go func() {
		EventWorker.processUserQueue(userID)
		<-uq.workerPool
	}()
}

func AddEvent(userID string, handler func(Event), payload ...any) {
	event := Event{
		UserID:  userID,
		Handler: handler,
		TraceID: uuid.New().String(),
		Payload: payload,
	}

	EventWorker.mu.Lock()

	if _, exists := EventWorker.userQueues[event.UserID]; !exists {

		EventWorker.userQueues[event.UserID] = make(chan Event, 100)
		go EventWorker.startWorker(event.UserID)
	}
	userQueue := EventWorker.userQueues[event.UserID]
	EventWorker.mu.Unlock()

	userQueue <- event
}

func (uq *UserQueue) processUserQueue(userID string) {
	queue := uq.userQueues[userID]
	for event := range queue {
		event.Handler(event)
	}

	uq.mu.Lock()
	delete(uq.userQueues, userID)
	uq.mu.Unlock()
}

func (uq *UserQueue) autoScale() {
	for {
		select {
		case <-uq.stopScaling:
			uq.scalingTicker.Stop()
			return
		case <-uq.scalingTicker.C:
			uq.adjustedWorkers()
		}
	}
}

func (uq *UserQueue) adjustedWorkers() {
	uq.mu.Lock()
	defer uq.mu.Unlock()

	activeQueues := len(uq.userQueues)
	currentWorkers := len(uq.workerPool)

	if activeQueues > currentWorkers && currentWorkers < uq.maxWorkers {

		additionalWorkers := min(uq.maxWorkers-currentWorkers, activeQueues-currentWorkers)

		for range additionalWorkers {
			uq.workerPool <- struct{}{}
		}

	} else if activeQueues == 0 && currentWorkers > 1 {
		extraWorkers := currentWorkers - 1
		for range extraWorkers {
			<-uq.workerPool
		}
	}

}
