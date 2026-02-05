import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import { DeepVoidBackground } from '../components/DeepVoidBackground'
import type {
  WeeklyInsightsResponse,
  DailyInsightsResponse,
  WatchlistResponse,
  NarrativeInfo,
  TokenInfo,
} from '../types'
import {
  TrendingUp,
  TrendingDown,
  Calendar,
  BarChart3,
  Flame,
  RefreshCw,
  Clock,
  Sparkles,
  ArrowUpRight,
  ArrowDownRight,
  Minus,
} from 'lucide-react'

// Translations
const T: Record<string, Record<string, string>> = {
  pageTitle: { zh: 'AI 深度敘事', en: 'AI Deep Insights' },
  pageSubtitle: { zh: '市場回顧與趨勢分析', en: 'Market Review & Trend Analysis' },
  weeklyReview: { zh: '本週回顧', en: 'Weekly Review' },
  dailyComparison: { zh: '今日 vs 昨日', en: 'Today vs Yesterday' },
  watchlist: { zh: '值得關注', en: 'Worth Watching' },
  totalVolume: { zh: '總交易量', en: 'Total Volume' },
  volumeChange: { zh: '交易量變化', en: 'Volume Change' },
  newTokens: { zh: '新上線代幣', en: 'New Tokens' },
  sentiment: { zh: '市場情緒', en: 'Market Sentiment' },
  topGainers: { zh: '週漲幅王', en: 'Top Gainers' },
  topLosers: { zh: '週跌幅王', en: 'Top Losers' },
  dominantNarrative: { zh: '主導敘事', en: 'Dominant Narrative' },
  narrativeStats: { zh: '敘事統計', en: 'Narrative Stats' },
  consecutiveGainers: { zh: '連續上漲', en: 'Consecutive Gainers' },
  volumeSpikes: { zh: '交易量突增', en: 'Volume Spikes' },
  emergingNarratives: { zh: '新興敘事', en: 'Emerging Narratives' },
  lastUpdated: { zh: '最後更新', en: 'Last Updated' },
  noData: { zh: '暫無數據', en: 'No data available' },
  loading: { zh: '載入中...', en: 'Loading...' },
  days: { zh: '天', en: 'days' },
  tokens: { zh: '代幣', en: 'tokens' },
  volume: { zh: '交易量', en: 'Volume' },
  share: { zh: '佔比', en: 'Share' },
  avgChange: { zh: '平均漲跌', en: 'Avg Change' },
  emerging: { zh: '新興', en: 'Emerging' },
}

const t = (key: string, lang: string) => T[key]?.[lang] || T[key]?.en || key

// Format large numbers
function formatVolume(value: number): string {
  if (value >= 1e9) return `$${(value / 1e9).toFixed(2)}B`
  if (value >= 1e6) return `$${(value / 1e6).toFixed(2)}M`
  if (value >= 1e3) return `$${(value / 1e3).toFixed(2)}K`
  return `$${value.toFixed(2)}`
}

// Format percentage
function formatPercent(value: number): string {
  const sign = value >= 0 ? '+' : ''
  return `${sign}${value.toFixed(2)}%`
}

// Get change color
function getChangeColor(value: number): string {
  if (value > 0) return 'text-green-400'
  if (value < 0) return 'text-red-400'
  return 'text-zinc-400'
}

// Get change icon
function getChangeIcon(value: number) {
  if (value > 0) return <ArrowUpRight className="w-4 h-4" />
  if (value < 0) return <ArrowDownRight className="w-4 h-4" />
  return <Minus className="w-4 h-4" />
}

// Sentiment color
function getSentimentColor(label: string): string {
  if (label.includes('貪婪') || label.includes('Greed')) return 'text-green-400'
  if (label.includes('恐懼') || label.includes('Fear')) return 'text-red-400'
  return 'text-yellow-400'
}

// Card component
function InsightCard({
  title,
  icon,
  children,
  className = '',
}: {
  title: string
  icon: React.ReactNode
  children: React.ReactNode
  className?: string
}) {
  return (
    <div
      className={`bg-black/40 border border-white/10 rounded-xl p-6 backdrop-blur-md ${className}`}
    >
      <div className="flex items-center gap-3 mb-4">
        <div className="w-10 h-10 rounded-lg bg-nofx-gold/10 border border-nofx-gold/20 flex items-center justify-center">
          {icon}
        </div>
        <h2 className="text-lg font-semibold text-white">{title}</h2>
      </div>
      {children}
    </div>
  )
}

// Stat item component
function StatItem({
  label,
  value,
  change,
  changeLabel,
}: {
  label: string
  value: string
  change?: number
  changeLabel?: string
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-zinc-500">{label}</span>
      <span className="text-xl font-bold text-white">{value}</span>
      {change !== undefined && (
        <div className={`flex items-center gap-1 text-sm ${getChangeColor(change)}`}>
          {getChangeIcon(change)}
          <span>{formatPercent(change)}</span>
          {changeLabel && <span className="text-zinc-500 text-xs">{changeLabel}</span>}
        </div>
      )}
    </div>
  )
}

// Token list component
function TokenList({
  tokens,
  title,
  isGainer,
  language,
}: {
  tokens: TokenInfo[]
  title: string
  isGainer: boolean
  language: string
}) {
  if (!tokens || tokens.length === 0) {
    return (
      <div className="text-center py-4 text-zinc-500">
        {t('noData', language)}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium text-zinc-400">{title}</h3>
      {tokens.slice(0, 5).map((token, i) => (
        <div
          key={`${token.symbol}-${i}`}
          className="flex items-center justify-between p-3 rounded-lg bg-white/5 hover:bg-white/10 transition-colors"
        >
          <div className="flex items-center gap-3">
            <span className="text-lg font-bold text-white">#{i + 1}</span>
            <div>
              <span className="font-semibold text-white">${token.symbol}</span>
              {token.narrative && (
                <span className="ml-2 text-xs px-2 py-0.5 rounded-full bg-white/10 text-zinc-400">
                  {token.narrative}
                </span>
              )}
            </div>
          </div>
          <div className={`flex items-center gap-1 font-mono ${isGainer ? 'text-green-400' : 'text-red-400'}`}>
            {isGainer ? <TrendingUp className="w-4 h-4" /> : <TrendingDown className="w-4 h-4" />}
            {formatPercent(token.price_change)}
          </div>
        </div>
      ))}
    </div>
  )
}

// Narrative list component
function NarrativeList({
  narratives,
  language,
}: {
  narratives: NarrativeInfo[]
  language: string
}) {
  if (!narratives || narratives.length === 0) {
    return (
      <div className="text-center py-4 text-zinc-500">
        {t('noData', language)}
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {narratives.slice(0, 6).map((narrative, i) => (
        <div
          key={narrative.name}
          className="flex items-center justify-between p-3 rounded-lg bg-white/5 hover:bg-white/10 transition-colors"
        >
          <div className="flex items-center gap-3">
            <span className="text-sm font-medium text-zinc-400">#{i + 1}</span>
            <div>
              <span className="font-semibold text-white">{narrative.name}</span>
              {narrative.is_emerging && (
                <span className="ml-2 text-xs px-2 py-0.5 rounded-full bg-nofx-gold/20 text-nofx-gold border border-nofx-gold/30">
                  {t('emerging', language)}
                </span>
              )}
            </div>
          </div>
          <div className="flex items-center gap-4 text-sm">
            <div className="text-right">
              <div className="text-zinc-400">{narrative.token_count} {t('tokens', language)}</div>
              <div className="text-zinc-500">{(narrative.volume_share * 100).toFixed(1)}% {t('share', language)}</div>
            </div>
            <div className={`font-mono ${getChangeColor(narrative.avg_price_change)}`}>
              {formatPercent(narrative.avg_price_change)}
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}

export function InsightsPage() {
  const { language } = useLanguage()

  // Fetch insights data
  const { data: weekly, isLoading: weeklyLoading } = useSWR<WeeklyInsightsResponse>(
    'insights-weekly',
    api.getInsightsWeekly,
    {
      refreshInterval: 12 * 60 * 60 * 1000, // 12 hours
      revalidateOnFocus: false,
    }
  )

  const { data: daily, isLoading: dailyLoading } = useSWR<DailyInsightsResponse>(
    'insights-daily',
    api.getInsightsDaily,
    {
      refreshInterval: 30 * 60 * 1000, // 30 minutes
      revalidateOnFocus: false,
    }
  )

  const { data: watchlist, isLoading: watchlistLoading } = useSWR<WatchlistResponse>(
    'insights-watchlist',
    api.getInsightsWatchlist,
    {
      refreshInterval: 60 * 60 * 1000, // 1 hour
      revalidateOnFocus: false,
    }
  )

  const isLoading = weeklyLoading || dailyLoading || watchlistLoading

  return (
    <DeepVoidBackground className="py-8 min-h-screen" disableAnimation>
      <div className="container mx-auto max-w-7xl px-4 md:px-8 space-y-8">
        {/* Page Header */}
        <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl flex items-center justify-center bg-black/60 border border-nofx-gold/30 shadow-[0_0_15px_rgba(240,185,11,0.2)]">
              <Sparkles className="w-7 h-7 text-nofx-gold" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-white flex items-center gap-2">
                {t('pageTitle', language)}
                <span className="text-xs font-normal px-2 py-1 rounded bg-nofx-gold/10 text-nofx-gold border border-nofx-gold/20">
                  Beta
                </span>
              </h1>
              <p className="text-sm text-zinc-400">{t('pageSubtitle', language)}</p>
            </div>
          </div>
          {weekly?.cached_at && (
            <div className="flex items-center gap-2 text-xs text-zinc-500">
              <Clock className="w-3 h-3" />
              {t('lastUpdated', language)}: {new Date(weekly.cached_at).toLocaleString()}
            </div>
          )}
        </div>

        {isLoading ? (
          <div className="flex items-center justify-center py-20">
            <RefreshCw className="w-8 h-8 text-nofx-gold animate-spin" />
            <span className="ml-3 text-zinc-400">{t('loading', language)}</span>
          </div>
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Weekly Review */}
            <InsightCard
              title={t('weeklyReview', language)}
              icon={<Calendar className="w-5 h-5 text-nofx-gold" />}
              className="lg:col-span-2"
            >
              <div className="grid grid-cols-2 md:grid-cols-4 gap-6 mb-6">
                <StatItem
                  label={t('totalVolume', language)}
                  value={formatVolume(weekly?.total_volume || 0)}
                  change={weekly?.volume_change}
                />
                <StatItem
                  label={t('newTokens', language)}
                  value={`${weekly?.total_new_tokens || 0}`}
                />
                <StatItem
                  label={t('sentiment', language)}
                  value={weekly?.sentiment_label || '-'}
                />
                <StatItem
                  label={t('dominantNarrative', language)}
                  value={weekly?.dominant_narrative?.name || '-'}
                />
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <TokenList
                  tokens={weekly?.top_gainers || []}
                  title={t('topGainers', language)}
                  isGainer={true}
                  language={language}
                />
                <TokenList
                  tokens={weekly?.top_losers || []}
                  title={t('topLosers', language)}
                  isGainer={false}
                  language={language}
                />
              </div>
            </InsightCard>

            {/* Daily Comparison */}
            <InsightCard
              title={t('dailyComparison', language)}
              icon={<BarChart3 className="w-5 h-5 text-nofx-gold" />}
            >
              <div className="space-y-6">
                <div className="grid grid-cols-2 gap-4">
                  <StatItem
                    label={t('totalVolume', language)}
                    value={formatVolume(daily?.today_volume || 0)}
                    change={daily?.volume_change}
                  />
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-zinc-500">{t('sentiment', language)}</span>
                    <span className={`text-xl font-bold ${getSentimentColor(daily?.sentiment_label || '')}`}>
                      {daily?.sentiment_label || '-'}
                    </span>
                    {daily?.sentiment_change !== undefined && daily.sentiment_change !== 0 && (
                      <div className={`flex items-center gap-1 text-sm ${getChangeColor(daily.sentiment_change)}`}>
                        {getChangeIcon(daily.sentiment_change)}
                        <span>{daily.sentiment_change > 0 ? '+' : ''}{daily.sentiment_change}</span>
                      </div>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div className="p-3 rounded-lg bg-white/5">
                    <div className="text-xs text-zinc-500 mb-1">{t('newTokens', language)}</div>
                    <div className="text-lg font-bold text-white">{daily?.today_new_tokens || 0}</div>
                  </div>
                  <div className="p-3 rounded-lg bg-white/5">
                    <div className="text-xs text-zinc-500 mb-1">{t('tokens', language)}</div>
                    <div className="text-lg font-bold text-white">{daily?.today_active_tokens || 0}</div>
                  </div>
                </div>
              </div>
            </InsightCard>

            {/* Watchlist */}
            <InsightCard
              title={t('watchlist', language)}
              icon={<Flame className="w-5 h-5 text-nofx-gold" />}
            >
              <div className="space-y-4">
                {/* Consecutive Gainers */}
                {watchlist?.consecutive_gainers && watchlist.consecutive_gainers.length > 0 && (
                  <div>
                    <h3 className="text-sm font-medium text-zinc-400 mb-2 flex items-center gap-2">
                      <TrendingUp className="w-4 h-4 text-green-400" />
                      {t('consecutiveGainers', language)}
                    </h3>
                    <div className="space-y-2">
                      {watchlist.consecutive_gainers.slice(0, 3).map((token, i) => (
                        <div
                          key={`gain-${token.symbol}-${i}`}
                          className="flex items-center justify-between p-2 rounded-lg bg-white/5"
                        >
                          <span className="font-semibold text-white">${token.symbol}</span>
                          <div className="text-right">
                            <div className="text-green-400 font-mono text-sm">
                              {token.consecutive_days} {t('days', language)}
                            </div>
                            <div className="text-xs text-zinc-500">
                              {formatPercent(token.total_change || 0)}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Emerging Narratives */}
                {watchlist?.emerging_narratives && watchlist.emerging_narratives.length > 0 && (
                  <div>
                    <h3 className="text-sm font-medium text-zinc-400 mb-2 flex items-center gap-2">
                      <Sparkles className="w-4 h-4 text-nofx-gold" />
                      {t('emergingNarratives', language)}
                    </h3>
                    <div className="space-y-2">
                      {watchlist.emerging_narratives.slice(0, 3).map((narrative, i) => (
                        <div
                          key={`emerging-${narrative.name}-${i}`}
                          className="flex items-center justify-between p-2 rounded-lg bg-white/5"
                        >
                          <span className="font-semibold text-white">{narrative.name}</span>
                          <div className="text-right">
                            <div className="text-nofx-gold font-mono text-sm">
                              {narrative.token_count} {t('tokens', language)}
                            </div>
                            <div className={`text-xs ${getChangeColor(narrative.avg_price_change)}`}>
                              {formatPercent(narrative.avg_price_change)}
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Empty state */}
                {(!watchlist?.consecutive_gainers?.length &&
                  !watchlist?.volume_spikes?.length &&
                  !watchlist?.emerging_narratives?.length) && (
                  <div className="text-center py-8 text-zinc-500">
                    {t('noData', language)}
                  </div>
                )}
              </div>
            </InsightCard>

            {/* Narrative Stats */}
            <InsightCard
              title={t('narrativeStats', language)}
              icon={<BarChart3 className="w-5 h-5 text-nofx-gold" />}
              className="lg:col-span-2"
            >
              <NarrativeList
                narratives={weekly?.narrative_stats || []}
                language={language}
              />
            </InsightCard>
          </div>
        )}
      </div>
    </DeepVoidBackground>
  )
}
