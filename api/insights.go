package api

import (
	"encoding/json"
	"net/http"
	"time"

	"nofx/store"

	"github.com/gin-gonic/gin"
)

// InsightsHandler handles market insights API endpoints
type InsightsHandler struct {
	store *store.Store
}

// NewInsightsHandler creates a new insights handler
func NewInsightsHandler(st *store.Store) *InsightsHandler {
	return &InsightsHandler{store: st}
}

// WeeklyInsightsResponse represents the weekly insights API response
type WeeklyInsightsResponse struct {
	TotalVolume       float64                  `json:"total_volume"`
	VolumeChange      float64                  `json:"volume_change"`
	AvgDailyVolume    float64                  `json:"avg_daily_volume"`
	TotalNewTokens    int                      `json:"total_new_tokens"`
	AvgSentiment      int                      `json:"avg_sentiment"`
	SentimentLabel    string                   `json:"sentiment_label"`
	TopGainers        []TokenInfo              `json:"top_gainers"`
	TopLosers         []TokenInfo              `json:"top_losers"`
	DominantNarrative *NarrativeInfo           `json:"dominant_narrative"`
	NarrativeStats    []NarrativeInfo          `json:"narrative_stats"`
	ChainStats        []ChainInfo              `json:"chain_stats"`
	DailySnapshots    []DailySnapshotInfo      `json:"daily_snapshots"`
	AISummary         *AISummary               `json:"ai_summary,omitempty"`
	CachedAt          time.Time                `json:"cached_at"`
}

// DailyInsightsResponse represents today vs yesterday comparison
type DailyInsightsResponse struct {
	TodayVolume        float64         `json:"today_volume"`
	YesterdayVolume    float64         `json:"yesterday_volume"`
	VolumeChange       float64         `json:"volume_change"`
	TodaySentiment     int             `json:"today_sentiment"`
	YesterdaySentiment int             `json:"yesterday_sentiment"`
	SentimentChange    int             `json:"sentiment_change"`
	SentimentLabel     string          `json:"sentiment_label"`
	TodayActiveTokens  int             `json:"today_active_tokens"`
	TodayNewTokens     int             `json:"today_new_tokens"`
	NarrativeChanges   []NarrativeChange `json:"narrative_changes"`
	CachedAt           time.Time       `json:"cached_at"`
}

// WatchlistResponse represents tokens worth watching
type WatchlistResponse struct {
	ConsecutiveGainers []WatchlistToken `json:"consecutive_gainers"`
	VolumeSpikes       []WatchlistToken `json:"volume_spikes"`
	EmergingNarratives []NarrativeInfo  `json:"emerging_narratives"`
	CachedAt           time.Time        `json:"cached_at"`
}

// TokenInfo represents basic token information
type TokenInfo struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	PriceChange  float64 `json:"price_change"`
	Volume       float64 `json:"volume"`
	Chain        string  `json:"chain"`
	Narrative    string  `json:"narrative"`
}

// NarrativeInfo represents narrative statistics
type NarrativeInfo struct {
	Name           string  `json:"name"`
	TokenCount     int     `json:"token_count"`
	TotalVolume    float64 `json:"total_volume"`
	VolumeShare    float64 `json:"volume_share"`
	AvgPriceChange float64 `json:"avg_price_change"`
	IsEmerging     bool    `json:"is_emerging"`
}

// NarrativeChange represents change in narrative ranking
type NarrativeChange struct {
	Name         string  `json:"name"`
	TodayRank    int     `json:"today_rank"`
	YesterdayRank int    `json:"yesterday_rank"`
	RankChange   int     `json:"rank_change"`
	VolumeChange float64 `json:"volume_change"`
}

// ChainInfo represents chain statistics
type ChainInfo struct {
	Name         string  `json:"name"`
	Volume       float64 `json:"volume"`
	VolumeShare  float64 `json:"volume_share"`
	ActiveTokens int     `json:"active_tokens"`
	NewTokens    int     `json:"new_tokens"`
}

// DailySnapshotInfo represents daily snapshot for charts
type DailySnapshotInfo struct {
	Date           string  `json:"date"`
	Volume         float64 `json:"volume"`
	ActiveTokens   int     `json:"active_tokens"`
	SentimentScore int     `json:"sentiment_score"`
}

// WatchlistToken represents a token in the watchlist
type WatchlistToken struct {
	Symbol          string  `json:"symbol"`
	Name            string  `json:"name"`
	Chain           string  `json:"chain"`
	ConsecutiveDays int     `json:"consecutive_days,omitempty"`
	TotalChange     float64 `json:"total_change,omitempty"`
	VolumeMultiple  float64 `json:"volume_multiple,omitempty"`
	CurrentVolume   float64 `json:"current_volume"`
	Narrative       string  `json:"narrative"`
}

// AISummary represents AI-generated analysis
type AISummary struct {
	WeeklySummary     string   `json:"weekly_summary"`
	DominantNarrative string   `json:"dominant_narrative"`
	CapitalFlow       string   `json:"capital_flow"`
	Outlook           string   `json:"outlook"`
	KeyTokens         []string `json:"key_tokens"`
	GeneratedAt       time.Time `json:"generated_at"`
}

// HandleGetWeeklyInsights returns weekly market insights
// GET /api/insights/weekly
func (h *InsightsHandler) HandleGetWeeklyInsights(c *gin.Context) {
	// Get weekly stats from store
	weeklyStats, err := h.store.Insights().GetWeeklyStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get weekly stats"})
		return
	}

	// Get latest narrative data
	narratives, err := h.store.Insights().GetLatestNarrativeHistory()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get narrative data"})
		return
	}

	response := WeeklyInsightsResponse{
		TotalVolume:    weeklyStats.TotalVolume,
		VolumeChange:   weeklyStats.VolumeChange,
		AvgDailyVolume: weeklyStats.AvgDailyVolume,
		TotalNewTokens: weeklyStats.TotalNewTokens,
		AvgSentiment:   weeklyStats.AvgSentiment,
		SentimentLabel: getSentimentLabel(weeklyStats.AvgSentiment),
		CachedAt:       time.Now().UTC(),
	}

	// Parse snapshots for daily chart data
	for _, snap := range weeklyStats.Snapshots {
		response.DailySnapshots = append(response.DailySnapshots, DailySnapshotInfo{
			Date:           snap.SnapshotDate.Format("2006-01-02"),
			Volume:         snap.TotalVolume,
			ActiveTokens:   snap.ActiveTokens,
			SentimentScore: snap.SentimentScore,
		})

		// Parse top gainers/losers from latest snapshot
		if len(response.TopGainers) == 0 {
			response.TopGainers = parseTokenInfo(snap.TopGainers)
			response.TopLosers = parseTokenInfo(snap.TopLosers)
			response.ChainStats = parseChainInfo(snap.ChainStats)
		}
	}

	// Convert narrative history to response format
	var narrativeStats []NarrativeInfo
	for _, n := range narratives {
		info := NarrativeInfo{
			Name:           n.Narrative,
			TokenCount:     n.TokenCount,
			TotalVolume:    n.TotalVolume,
			VolumeShare:    n.VolumeShare,
			AvgPriceChange: n.AvgPriceChange,
			IsEmerging:     n.IsEmerging,
		}
		narrativeStats = append(narrativeStats, info)

		// First narrative is dominant (sorted by volume)
		if response.DominantNarrative == nil {
			response.DominantNarrative = &info
		}
	}
	response.NarrativeStats = narrativeStats

	c.JSON(http.StatusOK, response)
}

// HandleGetDailyInsights returns today vs yesterday comparison
// GET /api/insights/daily
func (h *InsightsHandler) HandleGetDailyInsights(c *gin.Context) {
	comparison, err := h.store.Insights().GetDailyComparison()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get daily comparison"})
		return
	}

	response := DailyInsightsResponse{
		TodayVolume:        comparison.TodayVolume,
		YesterdayVolume:    comparison.YesterdayVolume,
		VolumeChange:       comparison.VolumeChange,
		TodaySentiment:     comparison.TodaySentiment,
		YesterdaySentiment: comparison.YesterdaySentiment,
		SentimentChange:    comparison.SentimentChange,
		SentimentLabel:     getSentimentLabel(comparison.TodaySentiment),
		TodayActiveTokens:  comparison.TodayActiveTokens,
		TodayNewTokens:     comparison.TodayNewTokens,
		CachedAt:           time.Now().UTC(),
	}

	// TODO: Calculate narrative ranking changes
	// This would compare narrative positions between today and yesterday

	c.JSON(http.StatusOK, response)
}

// HandleGetWatchlist returns tokens worth watching
// GET /api/insights/watchlist
func (h *InsightsHandler) HandleGetWatchlist(c *gin.Context) {
	// Get emerging narratives
	emerging, err := h.store.Insights().GetEmergingNarratives()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get emerging narratives"})
		return
	}

	response := WatchlistResponse{
		ConsecutiveGainers: []WatchlistToken{}, // Would be populated from token price history
		VolumeSpikes:       []WatchlistToken{}, // Would be populated from volume analysis
		CachedAt:           time.Now().UTC(),
	}

	// Convert emerging narratives
	for _, n := range emerging {
		response.EmergingNarratives = append(response.EmergingNarratives, NarrativeInfo{
			Name:           n.Narrative,
			TokenCount:     n.TokenCount,
			TotalVolume:    n.TotalVolume,
			VolumeShare:    n.VolumeShare,
			AvgPriceChange: n.AvgPriceChange,
			IsEmerging:     true,
		})
	}

	c.JSON(http.StatusOK, response)
}

// Helper functions

func getSentimentLabel(score int) string {
	switch {
	case score >= 80:
		return "極度貪婪"
	case score >= 60:
		return "貪婪"
	case score >= 40:
		return "中性"
	case score >= 20:
		return "恐懼"
	default:
		return "極度恐懼"
	}
}

func parseTokenInfo(data []byte) []TokenInfo {
	var tokens []TokenInfo
	if len(data) == 0 {
		return tokens
	}
	_ = json.Unmarshal(data, &tokens)
	return tokens
}

func parseChainInfo(data []byte) []ChainInfo {
	var chains []ChainInfo
	if len(data) == 0 {
		return chains
	}
	_ = json.Unmarshal(data, &chains)
	return chains
}
