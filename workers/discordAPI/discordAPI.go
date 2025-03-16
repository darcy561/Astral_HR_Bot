package discordAPIWorker

import (
	"astralHRBot/logger"
	"astralHRBot/workers/eventWorker"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

var discordAPIWorker *DiscordAPISubmissionWorker
var once sync.Once
var workerReady sync.WaitGroup

type apiRequest struct {
	Event   eventWorker.Event
	Execute func() error
}

type DiscordAPISubmissionWorker struct {
	session      *discordgo.Session
	requestQueue chan apiRequest
	quit         chan struct{}
	wg           sync.WaitGroup
}

func NewWorker(s *discordgo.Session) {
	once.Do(func() {
		discordAPIWorker = &DiscordAPISubmissionWorker{
			session:      s,
			requestQueue: make(chan apiRequest),
			quit:         make(chan struct{}),
		}
		discordAPIWorker.wg.Add(1)
		workerReady.Add(1)
		go discordAPIWorker.run()
	})
	workerReady.Wait()
}

func (w *DiscordAPISubmissionWorker) run() {
	defer w.wg.Done()
	log.Println("DiscordAPIWoker Running")
	workerReady.Done()
	for {
		select {
		case request := <-w.requestQueue:
			if err := request.Execute(); err != nil {
				logger.Error(request.Event.TraceID, err.Error())
			}
			time.Sleep(1 * time.Second)

		case <-w.quit:
			return
		}
	}
}

func NewRequest(e eventWorker.Event, f func() error) {
	if discordAPIWorker == nil {
		log.Println("Worker not initialized yet!")
		return
	}

	discordAPIWorker.requestQueue <- apiRequest{
		Event:   e,
		Execute: f,
	}
}

func Stop() {
	if discordAPIWorker != nil {
		close(discordAPIWorker.quit)
		discordAPIWorker.wg.Wait()
	}
}
