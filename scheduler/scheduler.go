package scheduler

import (
	"context"
	"nofx/logger"
	"nofx/store"
	"sync"
	"time"
)

// Scheduler manages background jobs for data collection
type Scheduler struct {
	store    *store.Store
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	started  bool
	mu       sync.Mutex
}

// NewScheduler creates a new scheduler instance
func NewScheduler(st *store.Store) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		store:  st,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins all scheduled jobs
func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		logger.Warnf("[Scheduler] Already started")
		return
	}

	s.started = true
	logger.Infof("[Scheduler] Starting background jobs...")

	// Daily snapshot job - runs at 00:00 UTC
	s.wg.Add(1)
	go s.runDailySnapshotJob()

	// Narrative tracker job - runs every hour
	s.wg.Add(1)
	go s.runNarrativeTrackerJob()

	logger.Infof("[Scheduler] ✅ Background jobs started")
}

// Stop gracefully stops all scheduled jobs
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	logger.Infof("[Scheduler] Stopping background jobs...")
	s.cancel()
	s.wg.Wait()
	s.started = false
	logger.Infof("[Scheduler] ✅ All background jobs stopped")
}

// runDailySnapshotJob runs at 00:00 UTC daily
func (s *Scheduler) runDailySnapshotJob() {
	defer s.wg.Done()

	// Calculate time until next 00:00 UTC
	nextRun := getNextMidnightUTC()
	logger.Infof("[Scheduler] Daily snapshot job scheduled for %s", nextRun.Format(time.RFC3339))

	for {
		select {
		case <-s.ctx.Done():
			logger.Infof("[Scheduler] Daily snapshot job stopped")
			return
		case <-time.After(time.Until(nextRun)):
			s.executeDailySnapshot()
			nextRun = getNextMidnightUTC()
			logger.Infof("[Scheduler] Next daily snapshot at %s", nextRun.Format(time.RFC3339))
		}
	}
}

// runNarrativeTrackerJob runs every hour
func (s *Scheduler) runNarrativeTrackerJob() {
	defer s.wg.Done()

	// Run immediately on startup, then every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	// Execute once on startup
	s.executeNarrativeTracker()

	for {
		select {
		case <-s.ctx.Done():
			logger.Infof("[Scheduler] Narrative tracker job stopped")
			return
		case <-ticker.C:
			s.executeNarrativeTracker()
		}
	}
}

// executeDailySnapshot collects and saves daily market snapshot
func (s *Scheduler) executeDailySnapshot() {
	logger.Infof("[Scheduler] Executing daily snapshot job...")
	startTime := time.Now()

	// Get yesterday's date (we're running at 00:00, so aggregate previous day)
	snapshotDate := time.Now().UTC().AddDate(0, 0, -1).Truncate(24 * time.Hour)

	// TODO: In a real implementation, this would aggregate data from external sources
	// For now, create a placeholder snapshot that can be populated by external data feeds
	snapshot := &store.DailySnapshot{
		SnapshotDate:    snapshotDate,
		TotalVolume:     0, // To be populated by external data
		TotalMarketCap:  0,
		ActiveTokens:    0,
		NewTokens:       0,
		ChainStats:      []byte("[]"),
		NarrativeStats:  []byte("[]"),
		TopGainers:      []byte("[]"),
		TopLosers:       []byte("[]"),
		AvgBuySellRatio: 1.0,
		SentimentScore:  50,
	}

	if err := s.store.Insights().SaveDailySnapshot(snapshot); err != nil {
		logger.Errorf("[Scheduler] Failed to save daily snapshot: %v", err)
		return
	}

	logger.Infof("[Scheduler] ✅ Daily snapshot saved for %s (took %v)",
		snapshotDate.Format("2006-01-02"), time.Since(startTime))
}

// executeNarrativeTracker tracks narrative trends
func (s *Scheduler) executeNarrativeTracker() {
	logger.Infof("[Scheduler] Executing narrative tracker job...")
	startTime := time.Now()

	// TODO: In a real implementation, this would:
	// 1. Fetch narrative data from external API (e.g., DexScreener, CoinGecko)
	// 2. Aggregate token counts, volumes per narrative
	// 3. Identify emerging narratives based on volume growth
	//
	// For now, this is a placeholder that demonstrates the structure

	// Example narratives that would be populated from real data
	narratives := []struct {
		name       string
		tokenCount int
		volume     float64
		isEmerging bool
	}{
		// These would come from external data sources
	}

	now := time.Now().UTC()
	var histories []*store.NarrativeHistory

	for _, n := range narratives {
		history := &store.NarrativeHistory{
			RecordedAt:     now,
			Narrative:      n.name,
			TokenCount:     n.tokenCount,
			TotalVolume:    n.volume,
			AvgPriceChange: 0, // Would be calculated from real data
			VolumeShare:    0, // Would be calculated
			TopTokens:      []byte("[]"),
			IsEmerging:     n.isEmerging,
		}
		histories = append(histories, history)
	}

	if len(histories) > 0 {
		if err := s.store.Insights().SaveNarrativeHistoryBatch(histories); err != nil {
			logger.Errorf("[Scheduler] Failed to save narrative history: %v", err)
			return
		}
	}

	logger.Infof("[Scheduler] ✅ Narrative tracker completed (took %v)", time.Since(startTime))
}

// getNextMidnightUTC calculates the next 00:00 UTC
func getNextMidnightUTC() time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return next
}

// RunNow executes a job immediately (for manual triggers)
func (s *Scheduler) RunDailySnapshotNow() {
	go s.executeDailySnapshot()
}

func (s *Scheduler) RunNarrativeTrackerNow() {
	go s.executeNarrativeTracker()
}
