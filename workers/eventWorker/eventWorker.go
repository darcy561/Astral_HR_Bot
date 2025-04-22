package eventWorker

import (
	"astralHRBot/logger"
	"fmt"
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
	workerTimeout time.Duration
}

var (
	EventWorker *UserQueue
	once        sync.Once
)

func NewWorker() {
	once.Do(func() {
		logger.System(logger.LogData{
			"action":  "init_worker",
			"message": "Initializing event worker system",
		})
		EventWorker = &UserQueue{
			userQueues:    make(map[string]chan Event),
			workerPool:    make(chan struct{}, 5),
			maxWorkers:    5,
			scalingTicker: time.NewTicker(5 * time.Second),
			stopScaling:   make(chan bool),
			workerTimeout: 30 * time.Second,
		}
		go EventWorker.autoScale()
		logger.System(logger.LogData{
			"action":  "worker_ready",
			"message": "Event worker system initialized and autoscaling started",
		})
	})
}

func (uq *UserQueue) startWorker(userID string) {
	logger.SystemDebug(logger.LogData{
		"action":  "acquire_worker",
		"user_id": userID,
	})
	uq.workerPool <- struct{}{}
	logger.SystemDebug(logger.LogData{
		"action":  "start_worker",
		"user_id": userID,
	})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.System(logger.LogData{
					"action":  "worker_panic",
					"user_id": userID,
					"error":   fmt.Sprintf("%v", r),
				})
			}
			logger.SystemDebug(logger.LogData{
				"action":  "worker_cleanup",
				"user_id": userID,
			})
			<-uq.workerPool
			logger.SystemDebug(logger.LogData{
				"action":  "worker_release",
				"user_id": userID,
			})
		}()
		EventWorker.processUserQueue(userID)
	}()
}

func AddEvent(userID string, handler func(Event), payload ...any) {
	event := Event{
		UserID:  userID,
		Handler: handler,
		TraceID: uuid.New().String(),
		Payload: payload,
	}

	logger.SystemDebug(logger.LogData{
		"action":   "add_event",
		"user_id":  userID,
		"trace_id": event.TraceID,
	})

	EventWorker.mu.Lock()
	defer EventWorker.mu.Unlock()

	if _, exists := EventWorker.userQueues[event.UserID]; !exists {
		logger.SystemDebug(logger.LogData{
			"action":  "create_queue",
			"user_id": userID,
		})
		EventWorker.userQueues[event.UserID] = make(chan Event)
		go EventWorker.startWorker(event.UserID)
	}

	userQueue := EventWorker.userQueues[event.UserID]
	userQueue <- event
	logger.SystemDebug(logger.LogData{
		"action":   "event_queued",
		"user_id":  userID,
		"trace_id": event.TraceID,
	})
}

func (uq *UserQueue) processUserQueue(userID string) {
	queue := uq.userQueues[userID]
	logger.SystemDebug(logger.LogData{
		"action":  "start_processing",
		"user_id": userID,
	})

	for event := range queue {
		startTime := time.Now()
		logger.SystemDebug(logger.LogData{
			"action":   "process_event",
			"user_id":  userID,
			"trace_id": event.TraceID,
		})

		// Process event with timeout
		done := make(chan bool)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.System(logger.LogData{
						"action":   "handler_panic",
						"user_id":  userID,
						"trace_id": event.TraceID,
						"error":    fmt.Sprintf("%v", r),
					})
					done <- false
				}
			}()
			event.Handler(event)
			done <- true
		}()

		select {
		case success := <-done:
			processingTime := time.Since(startTime)
			if success {
				logger.SystemDebug(logger.LogData{
					"action":          "event_complete",
					"user_id":         userID,
					"trace_id":        event.TraceID,
					"processing_time": processingTime.String(),
				})
			} else {
				logger.System(logger.LogData{
					"action":          "event_failed",
					"user_id":         userID,
					"trace_id":        event.TraceID,
					"processing_time": processingTime.String(),
				})
			}
		case <-time.After(uq.workerTimeout):
			logger.System(logger.LogData{
				"action":   "event_timeout",
				"user_id":  userID,
				"trace_id": event.TraceID,
			})
		}

		// Check if queue is empty after processing
		uq.mu.Lock()
		if len(queue) == 0 {
			logger.SystemDebug(logger.LogData{
				"action":  "cleanup_queue",
				"user_id": userID,
			})
			delete(uq.userQueues, userID)
			uq.mu.Unlock()
			return
		}
		uq.mu.Unlock()
	}
}

func (uq *UserQueue) autoScale() {
	logger.SystemDebug(logger.LogData{
		"action": "start_autoscaling",
	})
	for {
		select {
		case <-uq.stopScaling:
			logger.SystemDebug(logger.LogData{
				"action": "stop_autoscaling",
			})
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

	// First, clean up any empty queues
	cleanedQueues := 0
	for userID, queue := range uq.userQueues {
		if len(queue) == 0 {
			delete(uq.userQueues, userID)
			cleanedQueues++
		}
	}
	if cleanedQueues > 0 {
		logger.SystemDebug(logger.LogData{
			"action": "cleanup_queues",
			"count":  cleanedQueues,
		})
	}

	activeQueues := len(uq.userQueues)
	currentWorkers := len(uq.workerPool)

	if activeQueues > currentWorkers && currentWorkers < uq.maxWorkers {
		additionalWorkers := min(uq.maxWorkers-currentWorkers, activeQueues-currentWorkers)
		logger.SystemDebug(logger.LogData{
			"action":          "add_workers",
			"count":           additionalWorkers,
			"current_workers": currentWorkers,
			"max_workers":     uq.maxWorkers,
		})
		for range additionalWorkers {
			uq.workerPool <- struct{}{}
		}
	} else if activeQueues == 0 && currentWorkers > 1 {
		extraWorkers := currentWorkers - 1
		logger.SystemDebug(logger.LogData{
			"action":          "remove_workers",
			"count":           extraWorkers,
			"current_workers": currentWorkers,
		})
		for range extraWorkers {
			<-uq.workerPool
		}
	}
}
