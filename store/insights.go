package store

import (
	"fmt"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// InsightsStore provides storage for market insights and narrative tracking
type InsightsStore struct {
	db *gorm.DB
}

// DailySnapshot represents a daily market snapshot
type DailySnapshot struct {
	ID             int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	SnapshotDate   time.Time      `gorm:"column:snapshot_date;type:date;not null;uniqueIndex" json:"snapshot_date"`
	TotalVolume    float64        `gorm:"column:total_volume;not null;default:0" json:"total_volume"`
	TotalMarketCap float64        `gorm:"column:total_market_cap;not null;default:0" json:"total_market_cap"`
	ActiveTokens   int            `gorm:"column:active_tokens;not null;default:0" json:"active_tokens"`
	NewTokens      int            `gorm:"column:new_tokens;not null;default:0" json:"new_tokens"`
	ChainStats     datatypes.JSON `gorm:"column:chain_stats;type:jsonb;not null;default:'[]'" json:"chain_stats"`
	NarrativeStats datatypes.JSON `gorm:"column:narrative_stats;type:jsonb;not null;default:'[]'" json:"narrative_stats"`
	TopGainers     datatypes.JSON `gorm:"column:top_gainers;type:jsonb;not null;default:'[]'" json:"top_gainers"`
	TopLosers      datatypes.JSON `gorm:"column:top_losers;type:jsonb;not null;default:'[]'" json:"top_losers"`
	AvgBuySellRatio float64       `gorm:"column:avg_buy_sell_ratio;default:1.0" json:"avg_buy_sell_ratio"`
	SentimentScore int            `gorm:"column:sentiment_score;default:50" json:"sentiment_score"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
}

func (DailySnapshot) TableName() string { return "daily_snapshots" }

// NarrativeHistory represents narrative trend tracking over time
type NarrativeHistory struct {
	ID             int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	RecordedAt     time.Time      `gorm:"column:recorded_at;not null;default:CURRENT_TIMESTAMP;index:idx_narrative_history_time,sort:desc" json:"recorded_at"`
	Narrative      string         `gorm:"column:narrative;type:varchar(50);not null;uniqueIndex:idx_narrative_time" json:"narrative"`
	TokenCount     int            `gorm:"column:token_count;not null;default:0" json:"token_count"`
	TotalVolume    float64        `gorm:"column:total_volume;not null;default:0" json:"total_volume"`
	AvgPriceChange float64        `gorm:"column:avg_price_change;not null;default:0" json:"avg_price_change"`
	VolumeShare    float64        `gorm:"column:volume_share;not null;default:0" json:"volume_share"`
	TopTokens      datatypes.JSON `gorm:"column:top_tokens;type:jsonb;not null;default:'[]'" json:"top_tokens"`
	IsEmerging     bool           `gorm:"column:is_emerging;default:false" json:"is_emerging"`
}

func (NarrativeHistory) TableName() string { return "narrative_history" }

// Add unique index for recorded_at + narrative combination
func (n *NarrativeHistory) BeforeCreate(tx *gorm.DB) error {
	if n.RecordedAt.IsZero() {
		n.RecordedAt = time.Now().UTC()
	}
	return nil
}

// NewInsightsStore creates a new InsightsStore
func NewInsightsStore(db *gorm.DB) *InsightsStore {
	return &InsightsStore{db: db}
}

// initTables initializes insights tables
func (s *InsightsStore) initTables() error {
	// For PostgreSQL with existing table, skip AutoMigrate
	if s.db.Dialector.Name() == "postgres" {
		var snapshotExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'daily_snapshots'`).Scan(&snapshotExists)

		var narrativeExists int64
		s.db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'narrative_history'`).Scan(&narrativeExists)

		if snapshotExists > 0 && narrativeExists > 0 {
			return nil
		}
	}

	if err := s.db.AutoMigrate(&DailySnapshot{}); err != nil {
		return fmt.Errorf("failed to migrate DailySnapshot: %w", err)
	}
	if err := s.db.AutoMigrate(&NarrativeHistory{}); err != nil {
		return fmt.Errorf("failed to migrate NarrativeHistory: %w", err)
	}

	// Create composite unique index for narrative_history
	if s.db.Dialector.Name() == "postgres" {
		s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_narrative_time ON narrative_history(recorded_at, narrative)`)
	}

	return nil
}

// ============================================
// DailySnapshot CRUD
// ============================================

// SaveDailySnapshot saves or updates a daily snapshot
func (s *InsightsStore) SaveDailySnapshot(snapshot *DailySnapshot) error {
	// Normalize date to start of day
	snapshot.SnapshotDate = snapshot.SnapshotDate.UTC().Truncate(24 * time.Hour)

	// Upsert: update if exists, create if not
	result := s.db.Where("snapshot_date = ?", snapshot.SnapshotDate).
		Assign(DailySnapshot{
			TotalVolume:     snapshot.TotalVolume,
			TotalMarketCap:  snapshot.TotalMarketCap,
			ActiveTokens:    snapshot.ActiveTokens,
			NewTokens:       snapshot.NewTokens,
			ChainStats:      snapshot.ChainStats,
			NarrativeStats:  snapshot.NarrativeStats,
			TopGainers:      snapshot.TopGainers,
			TopLosers:       snapshot.TopLosers,
			AvgBuySellRatio: snapshot.AvgBuySellRatio,
			SentimentScore:  snapshot.SentimentScore,
		}).
		FirstOrCreate(snapshot)

	if result.Error != nil {
		return fmt.Errorf("failed to save daily snapshot: %w", result.Error)
	}
	return nil
}

// GetDailySnapshot gets snapshot for a specific date
func (s *InsightsStore) GetDailySnapshot(date time.Time) (*DailySnapshot, error) {
	date = date.UTC().Truncate(24 * time.Hour)
	var snapshot DailySnapshot
	err := s.db.Where("snapshot_date = ?", date).First(&snapshot).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get daily snapshot: %w", err)
	}
	return &snapshot, nil
}

// GetDailySnapshotsRange gets snapshots within a date range
func (s *InsightsStore) GetDailySnapshotsRange(start, end time.Time) ([]*DailySnapshot, error) {
	start = start.UTC().Truncate(24 * time.Hour)
	end = end.UTC().Truncate(24 * time.Hour)

	var snapshots []*DailySnapshot
	err := s.db.Where("snapshot_date >= ? AND snapshot_date <= ?", start, end).
		Order("snapshot_date ASC").
		Find(&snapshots).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get daily snapshots: %w", err)
	}
	return snapshots, nil
}

// GetLatestDailySnapshots gets the most recent N snapshots
func (s *InsightsStore) GetLatestDailySnapshots(limit int) ([]*DailySnapshot, error) {
	var snapshots []*DailySnapshot
	err := s.db.Order("snapshot_date DESC").Limit(limit).Find(&snapshots).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get latest daily snapshots: %w", err)
	}

	// Reverse to chronological order
	for i, j := 0, len(snapshots)-1; i < j; i, j = i+1, j-1 {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	}

	return snapshots, nil
}

// ============================================
// NarrativeHistory CRUD
// ============================================

// SaveNarrativeHistory saves narrative history record
func (s *InsightsStore) SaveNarrativeHistory(history *NarrativeHistory) error {
	if history.RecordedAt.IsZero() {
		history.RecordedAt = time.Now().UTC()
	}

	if err := s.db.Create(history).Error; err != nil {
		return fmt.Errorf("failed to save narrative history: %w", err)
	}
	return nil
}

// SaveNarrativeHistoryBatch saves multiple narrative history records
func (s *InsightsStore) SaveNarrativeHistoryBatch(histories []*NarrativeHistory) error {
	if len(histories) == 0 {
		return nil
	}

	now := time.Now().UTC()
	for _, h := range histories {
		if h.RecordedAt.IsZero() {
			h.RecordedAt = now
		}
	}

	if err := s.db.CreateInBatches(histories, 100).Error; err != nil {
		return fmt.Errorf("failed to save narrative history batch: %w", err)
	}
	return nil
}

// GetNarrativeHistoryRange gets narrative history within a time range
func (s *InsightsStore) GetNarrativeHistoryRange(start, end time.Time) ([]*NarrativeHistory, error) {
	var histories []*NarrativeHistory
	err := s.db.Where("recorded_at >= ? AND recorded_at <= ?", start, end).
		Order("recorded_at ASC").
		Find(&histories).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get narrative history: %w", err)
	}
	return histories, nil
}

// GetLatestNarrativeHistory gets the most recent narrative snapshot
func (s *InsightsStore) GetLatestNarrativeHistory() ([]*NarrativeHistory, error) {
	// Get the latest recorded_at timestamp
	var latestTime time.Time
	err := s.db.Model(&NarrativeHistory{}).Select("MAX(recorded_at)").Scan(&latestTime).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get latest narrative time: %w", err)
	}

	if latestTime.IsZero() {
		return nil, nil
	}

	// Get all narratives at that timestamp
	var histories []*NarrativeHistory
	err = s.db.Where("recorded_at = ?", latestTime).
		Order("total_volume DESC").
		Find(&histories).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get latest narratives: %w", err)
	}
	return histories, nil
}

// GetNarrativeTrend gets trend data for a specific narrative
func (s *InsightsStore) GetNarrativeTrend(narrative string, days int) ([]*NarrativeHistory, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days)

	var histories []*NarrativeHistory
	err := s.db.Where("narrative = ? AND recorded_at >= ?", narrative, cutoff).
		Order("recorded_at ASC").
		Find(&histories).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get narrative trend: %w", err)
	}
	return histories, nil
}

// GetTopNarratives gets top N narratives by volume
func (s *InsightsStore) GetTopNarratives(limit int) ([]*NarrativeHistory, error) {
	return s.GetLatestNarrativeHistory()
}

// GetEmergingNarratives gets narratives marked as emerging
func (s *InsightsStore) GetEmergingNarratives() ([]*NarrativeHistory, error) {
	var histories []*NarrativeHistory

	// Get latest recorded_at
	var latestTime time.Time
	err := s.db.Model(&NarrativeHistory{}).Select("MAX(recorded_at)").Scan(&latestTime).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get latest time: %w", err)
	}

	if latestTime.IsZero() {
		return nil, nil
	}

	err = s.db.Where("recorded_at = ? AND is_emerging = ?", latestTime, true).
		Order("volume_share DESC").
		Find(&histories).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get emerging narratives: %w", err)
	}
	return histories, nil
}

// ============================================
// Analytics helpers
// ============================================

// GetWeeklyStats calculates weekly statistics
type WeeklyStats struct {
	TotalVolume       float64              `json:"total_volume"`
	VolumeChange      float64              `json:"volume_change"` // percentage change from previous week
	AvgDailyVolume    float64              `json:"avg_daily_volume"`
	TotalNewTokens    int                  `json:"total_new_tokens"`
	AvgSentiment      int                  `json:"avg_sentiment"`
	TopGainers        []map[string]any     `json:"top_gainers"`
	DominantNarrative string               `json:"dominant_narrative"`
	Snapshots         []*DailySnapshot     `json:"snapshots"`
}

func (s *InsightsStore) GetWeeklyStats() (*WeeklyStats, error) {
	now := time.Now().UTC()
	weekStart := now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
	prevWeekStart := weekStart.AddDate(0, 0, -7)

	// Get this week's snapshots
	thisWeek, err := s.GetDailySnapshotsRange(weekStart, now)
	if err != nil {
		return nil, err
	}

	// Get previous week's snapshots for comparison
	prevWeek, err := s.GetDailySnapshotsRange(prevWeekStart, weekStart.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}

	stats := &WeeklyStats{
		Snapshots: thisWeek,
	}

	// Calculate this week's totals
	var totalVolume float64
	var totalNewTokens int
	var totalSentiment int
	for _, snap := range thisWeek {
		totalVolume += snap.TotalVolume
		totalNewTokens += snap.NewTokens
		totalSentiment += snap.SentimentScore
	}
	stats.TotalVolume = totalVolume
	stats.TotalNewTokens = totalNewTokens
	if len(thisWeek) > 0 {
		stats.AvgDailyVolume = totalVolume / float64(len(thisWeek))
		stats.AvgSentiment = totalSentiment / len(thisWeek)
	}

	// Calculate previous week's volume for comparison
	var prevTotalVolume float64
	for _, snap := range prevWeek {
		prevTotalVolume += snap.TotalVolume
	}
	if prevTotalVolume > 0 {
		stats.VolumeChange = ((totalVolume - prevTotalVolume) / prevTotalVolume) * 100
	}

	return stats, nil
}

// GetDailyComparison compares today vs yesterday
type DailyComparison struct {
	TodayVolume       float64 `json:"today_volume"`
	YesterdayVolume   float64 `json:"yesterday_volume"`
	VolumeChange      float64 `json:"volume_change"`
	TodaySentiment    int     `json:"today_sentiment"`
	YesterdaySentiment int    `json:"yesterday_sentiment"`
	SentimentChange   int     `json:"sentiment_change"`
	TodayActiveTokens int     `json:"today_active_tokens"`
	TodayNewTokens    int     `json:"today_new_tokens"`
}

func (s *InsightsStore) GetDailyComparison() (*DailyComparison, error) {
	now := time.Now().UTC().Truncate(24 * time.Hour)
	yesterday := now.AddDate(0, 0, -1)

	today, err := s.GetDailySnapshot(now)
	if err != nil {
		return nil, err
	}

	yesterdaySnap, err := s.GetDailySnapshot(yesterday)
	if err != nil {
		return nil, err
	}

	comparison := &DailyComparison{}

	if today != nil {
		comparison.TodayVolume = today.TotalVolume
		comparison.TodaySentiment = today.SentimentScore
		comparison.TodayActiveTokens = today.ActiveTokens
		comparison.TodayNewTokens = today.NewTokens
	}

	if yesterdaySnap != nil {
		comparison.YesterdayVolume = yesterdaySnap.TotalVolume
		comparison.YesterdaySentiment = yesterdaySnap.SentimentScore

		if yesterdaySnap.TotalVolume > 0 {
			comparison.VolumeChange = ((comparison.TodayVolume - yesterdaySnap.TotalVolume) / yesterdaySnap.TotalVolume) * 100
		}
		comparison.SentimentChange = comparison.TodaySentiment - yesterdaySnap.SentimentScore
	}

	return comparison, nil
}

// CleanOldRecords removes records older than specified days
func (s *InsightsStore) CleanOldRecords(days int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -days)

	// Clean daily snapshots
	if err := s.db.Where("snapshot_date < ?", cutoff).Delete(&DailySnapshot{}).Error; err != nil {
		return fmt.Errorf("failed to clean old daily snapshots: %w", err)
	}

	// Clean narrative history
	if err := s.db.Where("recorded_at < ?", cutoff).Delete(&NarrativeHistory{}).Error; err != nil {
		return fmt.Errorf("failed to clean old narrative history: %w", err)
	}

	return nil
}
