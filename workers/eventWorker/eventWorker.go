package eventWorker

import (
	"astralHRBot/bot/identity"
	"astralHRBot/logger"
	"errors"
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

type WorkerPool struct {
	mu           sync.Mutex
	userChannels map[string]chan Event
	idleTimeout  time.Duration
	shuttingDown bool
	wg           sync.WaitGroup
}

var wp *WorkerPool

// NewWorkerPool initializes the singleton worker pool
func NewWorkerPool() *WorkerPool {
	wp = &WorkerPool{
		userChannels: make(map[string]chan Event),
		idleTimeout:  10 * time.Second,
	}
	logger.Info(logger.LogData{
		"action":  "worker_pool_init",
		"message": "Worker pool initialized",
	})
	return wp
}

// Submit adds an event to the appropriate user's channel
func Submit(userID string, handler func(Event), payload ...any) error {
	if wp == nil {
		return errors.New("worker pool not initialized")
	}

	// Skip events from the bot itself
	if userID == identity.GetBotID() {
		logger.Debug(logger.LogData{
			"action":  "skip_bot_event",
			"message": "Skipping event from bot",
			"user_id": userID,
		})
		return nil
	}

	wp.mu.Lock()
	if wp.shuttingDown {
		wp.mu.Unlock()
		return errors.New("worker pool is shutting down")
	}

	event := Event{
		UserID:  userID,
		TraceID: uuid.New().String(),
		Payload: payload,
		Handler: handler,
	}

	ch, exists := wp.userChannels[userID]
	if !exists {
		ch = make(chan Event, 100)
		wp.userChannels[userID] = ch
		wp.wg.Add(1)
		go wp.startUserRoutine(userID, ch)
		logger.Debug(logger.LogData{
			"action":  "new_user_routine",
			"message": "Started new user routine",
			"user_id": userID,
		})
	}
	wp.mu.Unlock()

	ch <- event
	return nil
}

// startUserRoutine manages the processing of events for a user
func (wp *WorkerPool) startUserRoutine(userID string, ch chan Event) {
	defer wp.wg.Done()
	idleTimer := time.NewTimer(wp.idleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				logger.Debug(logger.LogData{
					"action":  "routine_shutdown",
					"message": "User routine shutting down",
					"user_id": userID,
				})
				return // channel closed externally
			}
			safeHandle(event)

			// Reset idle timer on activity
			if !idleTimer.Stop() {
				<-idleTimer.C
			}
			idleTimer.Reset(wp.idleTimeout)

		case <-idleTimer.C:
			wp.mu.Lock()
			if len(ch) == 0 {
				close(ch)
				delete(wp.userChannels, userID)
				logger.Debug(logger.LogData{
					"action":  "cleanup",
					"message": "User Routine closed due to inactivity",
					"user_id": userID,
				})
				wp.mu.Unlock()
				return
			}
			idleTimer.Reset(wp.idleTimeout)
			wp.mu.Unlock()
		}
	}
}

// safeHandle wraps event handling to recover from potential panics
func safeHandle(e Event) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(logger.LogData{
				"trace_id": e.TraceID,
				"action":   "handler_panic",
				"message":  "Recovered from panic in handler",
				"user_id":  e.UserID,
				"error":    r,
			})
		}
	}()
	e.Handler(e)
}

// Shutdown gracefully shuts down the worker pool
func Shutdown() {
	if wp == nil {
		return
	}

	wp.mu.Lock()
	wp.shuttingDown = true
	userCount := len(wp.userChannels)
	for _, ch := range wp.userChannels {
		close(ch)
	}
	wp.mu.Unlock()

	logger.Info(logger.LogData{
		"action":     "shutdown_start",
		"message":    "Starting worker pool shutdown",
		"user_count": userCount,
	})

	// Wait for all routines to finish
	wp.wg.Wait()
	logger.Info(logger.LogData{
		"action":  "shutdown_complete",
		"message": "Worker pool has shut down gracefully",
	})
}
