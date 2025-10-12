package importer

import (
	"fmt"
	"metar.live/logging"
	"sync"
	"time"
)

type ImportStatistics struct {
	logger *logging.Logger

	name string

	startTime time.Time
	endTime   time.Time

	// Number of stations imported.
	totalMutex sync.Mutex
	total      int

	updatedMutex sync.Mutex
	updated      int

	deletedMutex sync.Mutex
	deleted      int
}

func NewImportStatistics(name string, logger *logging.Logger) *ImportStatistics {
	return &ImportStatistics{
		name:   name,
		logger: logger,
	}
}

func (i *ImportStatistics) AddTotal() {
	i.totalMutex.Lock()
	defer i.totalMutex.Unlock()

	i.total++
}

func (i *ImportStatistics) AddUpdated() {
	i.updatedMutex.Lock()
	defer i.updatedMutex.Unlock()

	i.updated++
}

func (i *ImportStatistics) AddDeleted() {
	i.deletedMutex.Lock()
	defer i.deletedMutex.Unlock()

	i.deleted++
}

func (i *ImportStatistics) AddMultipleDeleted(deleted int) {
	i.deletedMutex.Lock()
	defer i.deletedMutex.Unlock()

	i.deleted += deleted
}

func (i *ImportStatistics) Start() {
	i.logger.Info(fmt.Sprintf("[IMPORT] Starting %s import", i.name))

	i.totalMutex.Lock()
	defer i.totalMutex.Unlock()

	i.updatedMutex.Lock()
	defer i.updatedMutex.Unlock()

	i.deletedMutex.Lock()
	defer i.deletedMutex.Unlock()

	i.total = 0
	i.updated = 0
	i.deleted = 0

	i.startTime = time.Now()
	i.endTime = time.Time{}
}

func (i *ImportStatistics) End() {
	i.endTime = time.Now()

	i.logger.Info(fmt.Sprintf("[IMPORT] Finished %s import", i.name))
	i.logger.Info(i.logStats())
}

func (i *ImportStatistics) logStats() string {
	return fmt.Sprintf("[IMPORT] Report for %s import: %d available for import, %d updated, %d deleted. Time: %s", i.name, i.total, i.updated, i.deleted, i.endTime.Sub(i.startTime))
}
