package logging

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/cenkalti/backoff/v4"
	"metar.live/environment"
)

type Logger struct {
	axiomClient *axiom.Client
	storeMutex  sync.Mutex
	eventStore  []axiom.Event
}

type Message struct {
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Time    time.Time              `json:"time"`
	Data    map[string]interface{} `json:"data"`
}

func (m *Message) String() string {
	return fmt.Sprintf("%s [%s] %s\n", m.Time.Format("2006-01-02 15:04:05"), m.Level, m.Message)
}

func NewLogger() *Logger {
	loggerObject := &Logger{}

	// Trigger upload every 10 seconds, when axiom is configured
	if environment.Global.AxiomDataset != "" {
		client, err := axiom.NewClient()
		if err != nil {
			log.Fatal(err)
		}

		loggerObject.axiomClient = client

		go func() {
			for {
				time.Sleep(10 * time.Second)
				go loggerObject.uploadLog()
			}
		}()
	}

	return loggerObject
}

func (l *Logger) Debug(input string) {
	m := Message{
		Level:   "DEBUG",
		Message: input,
		Time:    time.Now(),
	}

	message := fmt.Sprintf("[DEBUG] %s\n", input)
	go l.messageToAxiomEvent(&m)

	log.Print(message)
}

func (l *Logger) Info(input string) {
	m := Message{
		Level:   "INFO",
		Message: input,
		Time:    time.Now(),
	}

	message := fmt.Sprintf("[INFO] %s\n", input)
	go l.messageToAxiomEvent(&m)

	log.Print(message)
}

func (l *Logger) Warn(input string) {
	m := Message{
		Level:   "WARN",
		Message: input,
		Time:    time.Now(),
	}

	message := fmt.Sprintf("[WARN] %s\n", input)
	go l.messageToAxiomEvent(&m)

	log.Print(message)
}

func (l *Logger) Error(input string) {
	m := Message{
		Level:   "ERROR",
		Message: input,
		Time:    time.Now(),
	}

	message := fmt.Sprintf("[ERROR] %s\n", input)
	go l.messageToAxiomEvent(&m)

	log.Print(message)
}

func (l *Logger) Fatal(input error) {
	m := Message{
		Level:   "FATAL",
		Message: input.Error(),
		Time:    time.Now(),
	}

	message := fmt.Sprintf("[FATAL] %s\n", input)
	go l.messageToAxiomEvent(&m)

	log.Fatal(message)
}

func (l *Logger) messageToAxiomEvent(message *Message) {
	if environment.Global.AxiomDataset == "" {
		return
	}

	// Marshal to JSON string
	event := axiom.Event{
		"level":   message.Level,
		"message": message.Message,
		"time":    message.Time,
	}

	for s, i := range message.Data {
		event[s] = i
	}

	// Append to the message store
	l.storeMutex.Lock()
	l.eventStore = append(l.eventStore, event)

	// If the store is too big, upload it
	if len(l.eventStore) >= 999 {
		go l.uploadLog()
	}

	l.storeMutex.Unlock()
}

func (l *Logger) uploadLog() {
	l.storeMutex.Lock()
	defer l.storeMutex.Unlock()

	storeSize := len(l.eventStore)

	if storeSize == 0 || environment.Global.AxiomDataset == "" {
		return
	}

	log.Printf("Uploading %d log events to Axiom\n", storeSize)

	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute

	//Ingest with backoff
	err := backoff.Retry(func() error {
		res, err := l.axiomClient.Datasets.IngestEvents(context.Background(), environment.Global.AxiomDataset, l.eventStore)
		if err != nil {
			l.Error(fmt.Sprintf("Error ingesting events: %s", err))
		}

		// Make sure everything went smoothly.
		if res != nil {
			for _, fail := range res.Failures {
				l.Error(fail.Error)
			}
		}

		return err
	}, b)
	if err != nil {
		l.Error(fmt.Sprintf("Error uploading log events to Axiom: %s", err.Error()))
		return
	}

	// Clear the store
	l.eventStore = []axiom.Event{}
}
