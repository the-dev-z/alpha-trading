import { useEffect, useState, useRef } from 'react'
import useSWR, { mutate } from 'swr'
import { api } from './lib/api'
import { ChartTabs } from './components/ChartTabs'
import { AITradersPage } from './components/AITradersPage'
import { LoginPage } from './components/LoginPage'
import { RegisterPage } from './components/RegisterPage'
import { ResetPasswordPage } from './components/ResetPasswordPage'
import { CompetitionPage } from './components/CompetitionPage'
import { LandingPage } from './pages/LandingPage'
import { FAQPage } from './pages/FAQPage'
import { StrategyStudioPage } from './pages/StrategyStudioPage'
import { DebateArenaPage } from './pages/DebateArenaPage'
import HeaderBar from './components/HeaderBar'
import { LanguageProvider, useLanguage } from './contexts/LanguageContext'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { ConfirmDialogProvider } from './components/ConfirmDialog'
import { t, type Language } from './i18n/translations'
import { confirmToast, notify } from './lib/notify'
import { useSystemConfig } from './hooks/useSystemConfig'
import { DecisionCard } from './components/DecisionCard'
import { PunkAvatar, getTraderAvatar } from './components/PunkAvatar'
import { OFFICIAL_LINKS } from './constants/branding'
import { BacktestPage } from './components/BacktestPage'
import { LogOut, Loader2, Eye, EyeOff, Copy, Check } from 'lucide-react'
import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  Exchange,
} from './types'

type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'backtest'
  | 'strategy'
  | 'debate'
  | 'faq'
  | 'login'
  | 'register'

// 获取友好的AI模型名称
function getModelDisplayName(modelId: string): string {
  switch (modelId.toLowerCase()) {
    case 'deepseek':
      return 'DeepSeek'
    case 'qwen':
      return 'Qwen'
    case 'claude':
      return 'Claude'
    default:
      return modelId.toUpperCase()
  }
}

// Helper function to get exchange display name from exchange ID (UUID)
function getExchangeDisplayNameFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId) return 'Unknown'
  const exchange = exchanges?.find((e) => e.id === exchangeId)
  if (!exchange) return exchangeId.substring(0, 8).toUpperCase() + '...'
  const typeName = exchange.exchange_type?.toUpperCase() || exchange.name
  return exchange.account_name
    ? `${typeName} - ${exchange.account_name}`
    : typeName
}

// Helper function to get exchange type from exchange ID (UUID) - for TradingView charts
function getExchangeTypeFromList(
  exchangeId: string | undefined,
  exchanges: Exchange[] | undefined
): string {
  if (!exchangeId) return 'BINANCE'
  const exchange = exchanges?.find((e) => e.id === exchangeId)
  if (!exchange) return 'BINANCE' // Default to BINANCE for charts
  return exchange.exchange_type?.toUpperCase() || 'BINANCE'
}

// Helper function to check if exchange is a perp-dex type (wallet-based)
function isPerpDexExchange(exchangeType: string | undefined): boolean {
  if (!exchangeType) return false
  const perpDexTypes = ['hyperliquid', 'lighter', 'aster']
  return perpDexTypes.includes(exchangeType.toLowerCase())
}

// Helper function to get wallet address for perp-dex exchanges
function getWalletAddress(exchange: Exchange | undefined): string | undefined {
  if (!exchange) return undefined
  const type = exchange.exchange_type?.toLowerCase()
  switch (type) {
    case 'hyperliquid':
      return exchange.hyperliquidWalletAddr
    case 'lighter':
      return exchange.lighterWalletAddr
    case 'aster':
      return exchange.asterSigner
    default:
      return undefined
  }
}

// Helper function to truncate wallet address for display
function truncateAddress(address: string, startLen = 6, endLen = 4): string {
  if (address.length <= startLen + endLen + 3) return address
  return `${address.slice(0, startLen)}...${address.slice(-endLen)}`
}

function App() {
  const { language, setLanguage } = useLanguage()
  const { user, token, logout, isLoading } = useAuth()
  const { loading: configLoading } = useSystemConfig()
  const [route, setRoute] = useState(window.location.pathname)

  // 从URL路径读取初始页面状态（支持刷新保持页面）
  const getInitialPage = (): Page => {
    const path = window.location.pathname
    const hash = window.location.hash.slice(1) // 去掉 #

    if (path === '/traders' || hash === 'traders') return 'traders'
    if (path === '/backtest' || hash === 'backtest') return 'backtest'
    if (path === '/strategy' || hash === 'strategy') return 'strategy'
    if (path === '/debate' || hash === 'debate') return 'debate'
    if (path === '/dashboard' || hash === 'trader' || hash === 'details')
      return 'trader'
    return 'competition' // 默认为竞赛页面
  }

  const [currentPage, setCurrentPage] = useState<Page>(getInitialPage())
  // 从 URL 参数读取初始 trader 标识（格式: name-id前4位）
  const [selectedTraderSlug, setSelectedTraderSlug] = useState<string | undefined>(() => {
    const params = new URLSearchParams(window.location.search)
    return params.get('trader') || undefined
  })
  const [selectedTraderId, setSelectedTraderId] = useState<string | undefined>()

  // 生成 trader URL slug（name + ID 前 4 位）
  const getTraderSlug = (trader: TraderInfo) => {
    const idPrefix = trader.trader_id.slice(0, 4)
    return `${trader.trader_name}-${idPrefix}`
  }

  // 从 slug 解析并匹配 trader
  const findTraderBySlug = (slug: string, traderList: TraderInfo[]) => {
    // slug 格式: name-xxxx (xxxx 是 ID 前 4 位)
    const lastDashIndex = slug.lastIndexOf('-')
    if (lastDashIndex === -1) {
      // 没有 dash，直接按 name 匹配
      return traderList.find(t => t.trader_name === slug)
    }
    const name = slug.slice(0, lastDashIndex)
    const idPrefix = slug.slice(lastDashIndex + 1)
    return traderList.find(t =>
      t.trader_name === name && t.trader_id.startsWith(idPrefix)
    )
  }
  const [lastUpdate, setLastUpdate] = useState<string>('--:--:--')
  const [decisionsLimit, setDecisionsLimit] = useState<number>(5)

  // 监听URL变化，同步页面状态
  useEffect(() => {
    const handleRouteChange = () => {
      const path = window.location.pathname
      const hash = window.location.hash.slice(1)
      const params = new URLSearchParams(window.location.search)
      const traderParam = params.get('trader')

      if (path === '/traders' || hash === 'traders') {
        setCurrentPage('traders')
      } else if (path === '/backtest' || hash === 'backtest') {
        setCurrentPage('backtest')
      } else if (path === '/strategy' || hash === 'strategy') {
        setCurrentPage('strategy')
      } else if (path === '/debate' || hash === 'debate') {
        setCurrentPage('debate')
      } else if (
        path === '/dashboard' ||
        hash === 'trader' ||
        hash === 'details'
      ) {
        setCurrentPage('trader')
        // 如果 URL 中有 trader 参数（slug 格式），更新选中的 trader
        if (traderParam) {
          setSelectedTraderSlug(traderParam)
        }
      } else if (
        path === '/competition' ||
        hash === 'competition' ||
        hash === ''
      ) {
        setCurrentPage('competition')
      }
      setRoute(path)
    }

    window.addEventListener('hashchange', handleRouteChange)
    window.addEventListener('popstate', handleRouteChange)
    return () => {
      window.removeEventListener('hashchange', handleRouteChange)
      window.removeEventListener('popstate', handleRouteChange)
    }
  }, [])

  // 切换页面时更新URL hash (当前通过按钮直接调用setCurrentPage，这个函数暂时保留用于未来扩展)
  // const navigateToPage = (page: Page) => {
  //   setCurrentPage(page);
  //   window.location.hash = page === 'competition' ? '' : 'trader';
  // };

  // 获取trader列表（仅在用户登录时）
  const { data: traders, error: tradersError } = useSWR<TraderInfo[]>(
    user && token ? 'traders' : null,
    api.getTraders,
    {
      refreshInterval: 10000,
      shouldRetryOnError: false, // 避免在后端未运行时无限重试
    }
  )

  // 获取exchanges列表（用于显示交易所名称）
  const { data: exchanges } = useSWR<Exchange[]>(
    user && token ? 'exchanges' : null,
    api.getExchangeConfigs,
    {
      refreshInterval: 60000, // 1分钟刷新一次
      shouldRetryOnError: false,
    }
  )

  // 当获取到traders后，根据 URL 中的 trader slug 设置选中的 trader，或默认选中第一个
  useEffect(() => {
    if (traders && traders.length > 0 && !selectedTraderId) {
      if (selectedTraderSlug) {
        // 通过 slug 找到对应的 trader
        const trader = findTraderBySlug(selectedTraderSlug, traders)
        if (trader) {
          setSelectedTraderId(trader.trader_id)
        } else {
          // 如果找不到，选中第一个
          setSelectedTraderId(traders[0].trader_id)
        }
      } else {
        setSelectedTraderId(traders[0].trader_id)
      }
    }
  }, [traders, selectedTraderId, selectedTraderSlug])

  // 如果在trader页面，获取该trader的数据
  const { data: status } = useSWR<SystemStatus>(
    currentPage === 'trader' && selectedTraderId
      ? `status-${selectedTraderId}`
      : null,
    () => api.getStatus(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: account } = useSWR<AccountInfo>(
    currentPage === 'trader' && selectedTraderId
      ? `account-${selectedTraderId}`
      : null,
    () => api.getAccount(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: positions } = useSWR<Position[]>(
    currentPage === 'trader' && selectedTraderId
      ? `positions-${selectedTraderId}`
      : null,
    () => api.getPositions(selectedTraderId),
    {
      refreshInterval: 15000, // 15秒刷新（配合后端15秒缓存）
      revalidateOnFocus: false, // 禁用聚焦时重新验证，减少请求
      dedupingInterval: 10000, // 10秒去重，防止短时间内重复请求
    }
  )

  const { data: decisions } = useSWR<DecisionRecord[]>(
    currentPage === 'trader' && selectedTraderId
      ? `decisions/latest-${selectedTraderId}-${decisionsLimit}`
      : null,
    () => api.getLatestDecisions(selectedTraderId, decisionsLimit),
    {
      refreshInterval: 30000, // 30秒刷新（决策更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  const { data: stats } = useSWR<Statistics>(
    currentPage === 'trader' && selectedTraderId
      ? `statistics-${selectedTraderId}`
      : null,
    () => api.getStatistics(selectedTraderId),
    {
      refreshInterval: 30000, // 30秒刷新（统计数据更新频率较低）
      revalidateOnFocus: false,
      dedupingInterval: 20000,
    }
  )

  useEffect(() => {
    if (account) {
      const now = new Date().toLocaleTimeString()
      setLastUpdate(now)
    }
  }, [account])

  const selectedTrader = traders?.find((t) => t.trader_id === selectedTraderId)

  // Handle routing
  useEffect(() => {
    const handlePopState = () => {
      setRoute(window.location.pathname)
    }
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  // Set current page based on route for consistent navigation state
  useEffect(() => {
    if (route === '/competition') {
      setCurrentPage('competition')
    } else if (route === '/traders') {
      setCurrentPage('traders')
    } else if (route === '/dashboard') {
      setCurrentPage('trader')
    }
  }, [route])

  // Show loading spinner while checking auth or config
  if (isLoading || configLoading) {
    return (
      <div
        className="min-h-screen flex items-center justify-center"
        style={{ background: '#0B0E11' }}
      >
        <div className="text-center">
          <img
            src="/icons/alpha.svg"
            alt="Alpha Trading Logo"
            className="w-16 h-16 mx-auto mb-4 animate-pulse"
          />
          <p style={{ color: '#EAECEF' }}>{t('loading', language)}</p>
        </div>
      </div>
    )
  }

  // Handle specific routes regardless of authentication
  if (route === '/login') {
    return <LoginPage />
  }
  if (route === '/register') {
    return <RegisterPage />
  }
  if (route === '/faq') {
    return (
      <div
        className="min-h-screen"
        style={{ background: '#0B0E11', color: '#EAECEF' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="faq"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onPageChange={(page: Page) => {
            if (page === 'competition') {
              window.history.pushState({}, '', '/competition')
              setRoute('/competition')
              setCurrentPage('competition')
            } else if (page === 'traders') {
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            } else if (page === 'trader') {
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            } else if (page === 'faq') {
              window.history.pushState({}, '', '/faq')
              setRoute('/faq')
            } else if (page === 'backtest') {
              window.history.pushState({}, '', '/backtest')
              setRoute('/backtest')
              setCurrentPage('backtest')
            } else if (page === 'strategy') {
              window.history.pushState({}, '', '/strategy')
              setRoute('/strategy')
              setCurrentPage('strategy')
            } else if (page === 'debate') {
              window.history.pushState({}, '', '/debate')
              setRoute('/debate')
              setCurrentPage('debate')
            }
          }}
        />
        <FAQPage />
      </div>
    )
  }
  if (route === '/reset-password') {
    return <ResetPasswordPage />
  }
  if (route === '/competition') {
    return (
      <div
        className="min-h-screen"
        style={{ background: '#000000', color: '#EAECEF' }}
      >
        <HeaderBar
          isLoggedIn={!!user}
          currentPage="competition"
          language={language}
          onLanguageChange={setLanguage}
          user={user}
          onLogout={logout}
          onPageChange={(page: Page) => {
            console.log('Competition page onPageChange called with:', page)
            console.log('Current route:', route, 'Current page:', currentPage)

            if (page === 'competition') {
              console.log('Navigating to competition')
              window.history.pushState({}, '', '/competition')
              setRoute('/competition')
              setCurrentPage('competition')
            } else if (page === 'traders') {
              console.log('Navigating to traders')
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            } else if (page === 'trader') {
              console.log('Navigating to trader/dashboard')
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            } else if (page === 'faq') {
              console.log('Navigating to faq')
              window.history.pushState({}, '', '/faq')
              setRoute('/faq')
            } else if (page === 'backtest') {
              console.log('Navigating to backtest')
              window.history.pushState({}, '', '/backtest')
              setRoute('/backtest')
              setCurrentPage('backtest')
            } else if (page === 'strategy') {
              console.log('Navigating to strategy')
              window.history.pushState({}, '', '/strategy')
              setRoute('/strategy')
              setCurrentPage('strategy')
            } else if (page === 'debate') {
              console.log('Navigating to debate')
              window.history.pushState({}, '', '/debate')
              setRoute('/debate')
              setCurrentPage('debate')
            }

            console.log(
              'After navigation - route:',
              route,
              'currentPage:',
              currentPage
            )
          }}
        />
        <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
          <CompetitionPage />
        </main>
      </div>
    )
  }

  // Show landing page for root route
  if (route === '/' || route === '') {
    return <LandingPage />
  }

  // Allow unauthenticated users to open backtest page directly (others仍展示 Landing)
  if (!user || !token) {
    if (route === '/backtest' || currentPage === 'backtest') {
      return (
        <div
          className="min-h-screen"
          style={{ background: '#0B0E11', color: '#EAECEF' }}
        >
          <HeaderBar
            isLoggedIn={false}
            currentPage="backtest"
            language={language}
            onLanguageChange={setLanguage}
            onPageChange={(page: Page) => {
              if (page === 'competition') {
                window.history.pushState({}, '', '/competition')
                setRoute('/competition')
                setCurrentPage('competition')
              } else if (page === 'traders') {
                window.history.pushState({}, '', '/traders')
                setRoute('/traders')
                setCurrentPage('traders')
              }
            }}
          />
          <main className="max-w-[1920px] mx-auto px-6 py-6 pt-24">
            <BacktestPage />
          </main>
        </div>
      )
    }
    return <LandingPage />
  }

  // Show main app for authenticated users on other routes
  if (!user || !token) {
    // Default to landing page when not authenticated and no specific route
    return <LandingPage />
  }

  return (
    <div
      className="min-h-screen"
      style={{ background: '#0B0E11', color: '#EAECEF' }}
    >
      <HeaderBar
        isLoggedIn={!!user}
        currentPage={currentPage}
        language={language}
        onLanguageChange={setLanguage}
        user={user}
        onLogout={logout}
        onPageChange={(page: Page) => {
          console.log('Main app onPageChange called with:', page)

          if (page === 'competition') {
            window.history.pushState({}, '', '/competition')
            setRoute('/competition')
            setCurrentPage('competition')
          } else if (page === 'traders') {
            window.history.pushState({}, '', '/traders')
            setRoute('/traders')
            setCurrentPage('traders')
          } else if (page === 'trader') {
            window.history.pushState({}, '', '/dashboard')
            setRoute('/dashboard')
            setCurrentPage('trader')
          } else if (page === 'backtest') {
            window.history.pushState({}, '', '/backtest')
            setRoute('/backtest')
            setCurrentPage('backtest')
          } else if (page === 'strategy') {
            window.history.pushState({}, '', '/strategy')
            setRoute('/strategy')
            setCurrentPage('strategy')
          } else if (page === 'faq') {
            window.history.pushState({}, '', '/faq')
            setRoute('/faq')
          } else if (page === 'debate') {
            window.history.pushState({}, '', '/debate')
            setRoute('/debate')
            setCurrentPage('debate')
          }
        }}
      />

      {/* Main Content */}
      <main
        className={
          currentPage === 'debate'
            ? 'h-[calc(100vh-64px)] mt-16'
            : 'max-w-[1920px] mx-auto px-6 py-6 pt-24'
        }
      >
        {currentPage === 'competition' ? (
          <CompetitionPage />
        ) : currentPage === 'traders' ? (
          <AITradersPage
            onTraderSelect={(traderId) => {
              setSelectedTraderId(traderId)
              window.history.pushState({}, '', '/dashboard')
              setRoute('/dashboard')
              setCurrentPage('trader')
            }}
          />
        ) : currentPage === 'backtest' ? (
          <BacktestPage />
        ) : currentPage === 'strategy' ? (
          <StrategyStudioPage />
        ) : currentPage === 'debate' ? (
          <DebateArenaPage />
        ) : (
          <TraderDetailsPage
            selectedTrader={selectedTrader}
            status={status}
            account={account}
            positions={positions}
            decisions={decisions}
            decisionsLimit={decisionsLimit}
            onDecisionsLimitChange={setDecisionsLimit}
            stats={stats}
            lastUpdate={lastUpdate}
            language={language}
            traders={traders}
            tradersError={tradersError}
            selectedTraderId={selectedTraderId}
            onTraderSelect={(traderId) => {
              setSelectedTraderId(traderId)
              // 更新 URL 参数（使用 slug: name-id前4位）
              const trader = traders?.find(t => t.trader_id === traderId)
              if (trader) {
                const url = new URL(window.location.href)
                url.searchParams.set('trader', getTraderSlug(trader))
                window.history.replaceState({}, '', url.toString())
              }
            }}
            onNavigateToTraders={() => {
              window.history.pushState({}, '', '/traders')
              setRoute('/traders')
              setCurrentPage('traders')
            }}
            exchanges={exchanges}
          />
        )}
      </main>

      {/* Footer - Hidden on debate page */}
      {currentPage !== 'debate' && (
        <footer
          className="mt-16"
          style={{ borderTop: '1px solid #2B3139', background: '#181A20' }}
        >
          <div
            className="max-w-[1920px] mx-auto px-6 py-6 text-center text-sm"
            style={{ color: '#5E6673' }}
          >
            <p>{t('footerTitle', language)}</p>
            <p className="mt-1">{t('footerWarning', language)}</p>
            <div className="mt-4 flex items-center justify-center gap-3 flex-wrap">
              {/* GitHub */}
              <a
                href={OFFICIAL_LINKS.github}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#1E2329',
                  color: '#848E9C',
                  border: '1px solid #2B3139',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = '#2B3139'
                  e.currentTarget.style.color = '#EAECEF'
                  e.currentTarget.style.borderColor = '#F0B90B'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = '#1E2329'
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.borderColor = '#2B3139'
                }}
              >
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 16 16"
                  fill="currentColor"
                >
                  <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                </svg>
                GitHub
              </a>
              {/* Twitter/X */}
              <a
                href={OFFICIAL_LINKS.twitter}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#1E2329',
                  color: '#848E9C',
                  border: '1px solid #2B3139',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = '#2B3139'
                  e.currentTarget.style.color = '#EAECEF'
                  e.currentTarget.style.borderColor = '#1DA1F2'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = '#1E2329'
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.borderColor = '#2B3139'
                }}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
                </svg>
                Twitter
              </a>
              {/* Telegram */}
              <a
                href={OFFICIAL_LINKS.telegram}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{
                  background: '#1E2329',
                  color: '#848E9C',
                  border: '1px solid #2B3139',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.background = '#2B3139'
                  e.currentTarget.style.color = '#EAECEF'
                  e.currentTarget.style.borderColor = '#0088cc'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.background = '#1E2329'
                  e.currentTarget.style.color = '#848E9C'
                  e.currentTarget.style.borderColor = '#2B3139'
                }}
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z" />
                </svg>
                Telegram
              </a>
            </div>
          </div>
        </footer>
      )}
    </div>
  )
}

// Trader Details Page Component
function TraderDetailsPage({
  selectedTrader,
  status,
  account,
  positions,
  decisions,
  decisionsLimit,
  onDecisionsLimitChange,
  lastUpdate,
  language,
  traders,
  tradersError,
  selectedTraderId,
  onTraderSelect,
  onNavigateToTraders,
  exchanges,
}: {
  selectedTrader?: TraderInfo
  traders?: TraderInfo[]
  tradersError?: Error
  selectedTraderId?: string
  onTraderSelect: (traderId: string) => void
  onNavigateToTraders: () => void
  status?: SystemStatus
  account?: AccountInfo
  positions?: Position[]
  decisions?: DecisionRecord[]
  decisionsLimit: number
  onDecisionsLimitChange: (limit: number) => void
  stats?: Statistics
  lastUpdate: string
  language: Language
  exchanges?: Exchange[]
}) {
  const [closingPosition, setClosingPosition] = useState<string | null>(null)
  const [selectedChartSymbol, setSelectedChartSymbol] = useState<
    string | undefined
  >(undefined)
  const [chartUpdateKey, setChartUpdateKey] = useState<number>(0)
  const chartSectionRef = useRef<HTMLDivElement>(null)
  const [showWalletAddress, setShowWalletAddress] = useState<boolean>(false)
  const [copiedAddress, setCopiedAddress] = useState<boolean>(false)

  // Get current exchange info for perp-dex wallet display
  const currentExchange = exchanges?.find(
    (e) => e.id === selectedTrader?.exchange_id
  )
  const walletAddress = getWalletAddress(currentExchange)
  const isPerpDex = isPerpDexExchange(currentExchange?.exchange_type)

  // Copy wallet address to clipboard
  const handleCopyAddress = async () => {
    if (!walletAddress) return
    try {
      await navigator.clipboard.writeText(walletAddress)
      setCopiedAddress(true)
      setTimeout(() => setCopiedAddress(false), 2000)
    } catch (err) {
      console.error('Failed to copy address:', err)
    }
  }

  // 平仓操作
  const handleClosePosition = async (symbol: string, side: string) => {
    if (!selectedTraderId) return

    const confirmMsg =
      language === 'zh'
        ? `确定要平仓 ${symbol} ${side === 'LONG' ? '多仓' : '空仓'} 吗？`
        : `Are you sure you want to close ${symbol} ${side === 'LONG' ? 'LONG' : 'SHORT'} position?`

    const confirmed = await confirmToast(confirmMsg, {
      title: language === 'zh' ? '确认平仓' : 'Confirm Close',
      okText: language === 'zh' ? '确认' : 'Confirm',
      cancelText: language === 'zh' ? '取消' : 'Cancel',
    })

    if (!confirmed) return

    setClosingPosition(symbol)
    try {
      await api.closePosition(selectedTraderId, symbol, side)
      notify.success(
        language === 'zh' ? '平仓成功' : 'Position closed successfully'
      )
      // 使用 SWR mutate 刷新数据而非重新加载页面
      await Promise.all([
        mutate(`positions-${selectedTraderId}`),
        mutate(`account-${selectedTraderId}`),
      ])
    } catch (err: unknown) {
      const errorMsg =
        err instanceof Error
          ? err.message
          : language === 'zh'
            ? '平仓失败'
            : 'Failed to close position'
      notify.error(errorMsg)
    } finally {
      setClosingPosition(null)
    }
  }
  // If API failed with error, show empty state (likely backend not running)
  if (tradersError) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              border: '2px solid rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#F0B90B' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is loaded and empty, show empty state
  if (traders && traders.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div className="text-center max-w-md mx-auto px-6">
          {/* Icon */}
          <div
            className="w-24 h-24 mx-auto mb-6 rounded-full flex items-center justify-center"
            style={{
              background: 'rgba(240, 185, 11, 0.1)',
              border: '2px solid rgba(240, 185, 11, 0.3)',
            }}
          >
            <svg
              className="w-12 h-12"
              style={{ color: '#F0B90B' }}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
          </div>

          {/* Title */}
          <h2 className="text-2xl font-bold mb-3" style={{ color: '#EAECEF' }}>
            {t('dashboardEmptyTitle', language)}
          </h2>

          {/* Description */}
          <p className="text-base mb-6" style={{ color: '#848E9C' }}>
            {t('dashboardEmptyDescription', language)}
          </p>

          {/* CTA Button */}
          <button
            onClick={onNavigateToTraders}
            className="px-6 py-3 rounded-lg font-semibold transition-all hover:scale-105 active:scale-95"
            style={{
              background: 'linear-gradient(135deg, #F0B90B 0%, #FCD535 100%)',
              color: '#0B0E11',
              boxShadow: '0 4px 12px rgba(240, 185, 11, 0.3)',
            }}
          >
            {t('goToTradersPage', language)}
          </button>
        </div>
      </div>
    )
  }

  // If traders is still loading or selectedTrader is not ready, show skeleton
  if (!selectedTrader) {
    return (
      <div className="space-y-6">
        {/* Loading Skeleton - Binance Style */}
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-8 w-48 mb-3"></div>
          <div className="flex gap-4">
            <div className="skeleton h-4 w-32"></div>
            <div className="skeleton h-4 w-24"></div>
            <div className="skeleton h-4 w-28"></div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="binance-card p-5 animate-pulse">
              <div className="skeleton h-4 w-24 mb-3"></div>
              <div className="skeleton h-8 w-32"></div>
            </div>
          ))}
        </div>
        <div className="binance-card p-6 animate-pulse">
          <div className="skeleton h-6 w-40 mb-4"></div>
          <div className="skeleton h-64 w-full"></div>
        </div>
      </div>
    )
  }

  return (
    <div>
      {/* Trader Header */}
      <div
        className="mb-6 rounded p-6 animate-scale-in"
        style={{
          background:
            'linear-gradient(135deg, rgba(240, 185, 11, 0.15) 0%, rgba(252, 213, 53, 0.05) 100%)',
          border: '1px solid rgba(240, 185, 11, 0.2)',
          boxShadow: '0 0 30px rgba(240, 185, 11, 0.15)',
        }}
      >
        <div className="flex items-start justify-between mb-3">
          <h2
            className="text-2xl font-bold flex items-center gap-3"
            style={{ color: '#EAECEF' }}
          >
            <PunkAvatar
              seed={getTraderAvatar(
                selectedTrader.trader_id,
                selectedTrader.trader_name
              )}
              size={48}
              className="rounded-lg"
            />
            {selectedTrader.trader_name}
          </h2>

          <div className="flex items-center gap-4">
            {/* Trader Selector */}
            {traders && traders.length > 0 && (
              <div className="flex items-center gap-2">
                <span className="text-sm" style={{ color: '#848E9C' }}>
                  {t('switchTrader', language)}:
                </span>
                <select
                  value={selectedTraderId}
                  onChange={(e) => onTraderSelect(e.target.value)}
                  className="rounded px-3 py-2 text-sm font-medium cursor-pointer transition-colors"
                  style={{
                    background: '#1E2329',
                    border: '1px solid #2B3139',
                    color: '#EAECEF',
                  }}
                >
                  {traders.map((trader) => (
                    <option key={trader.trader_id} value={trader.trader_id}>
                      {trader.trader_name}
                    </option>
                  ))}
                </select>
              </div>
            )}

            {/* Wallet Address Display for Perp-DEX */}
            {exchanges && isPerpDex && (
              <div
                className="flex items-center gap-2 px-3 py-2 rounded"
                style={{
                  background: 'rgba(240, 185, 11, 0.1)',
                  border: '1px solid rgba(240, 185, 11, 0.3)',
                }}
              >
                {walletAddress ? (
                  <>
                    <span className="text-xs font-mono" style={{ color: '#F0B90B' }}>
                      {showWalletAddress
                        ? walletAddress
                        : truncateAddress(walletAddress)}
                    </span>
                    <button
                      type="button"
                      onClick={() => setShowWalletAddress(!showWalletAddress)}
                      className="p-1 rounded hover:bg-gray-700 transition-colors"
                      title={showWalletAddress ? (language === 'zh' ? '隐藏地址' : 'Hide address') : (language === 'zh' ? '显示完整地址' : 'Show full address')}
                    >
                      {showWalletAddress ? (
                        <EyeOff className="w-3.5 h-3.5" style={{ color: '#848E9C' }} />
                      ) : (
                        <Eye className="w-3.5 h-3.5" style={{ color: '#848E9C' }} />
                      )}
                    </button>
                    <button
                      type="button"
                      onClick={handleCopyAddress}
                      className="p-1 rounded hover:bg-gray-700 transition-colors"
                      title={language === 'zh' ? '复制地址' : 'Copy address'}
                    >
                      {copiedAddress ? (
                        <Check className="w-3.5 h-3.5" style={{ color: '#0ECB81' }} />
                      ) : (
                        <Copy className="w-3.5 h-3.5" style={{ color: '#848E9C' }} />
                      )}
                    </button>
                  </>
                ) : (
                  <span className="text-xs" style={{ color: '#848E9C' }}>
                    {language === 'zh' ? '未配置地址' : 'No address configured'}
                  </span>
                )}
              </div>
            )}
          </div>
        </div>
        <div
          className="flex items-center gap-4 text-sm flex-wrap"
          style={{ color: '#848E9C' }}
        >
          <span>
            AI Model:{' '}
            <span
              className="font-semibold"
              style={{
                color: selectedTrader.ai_model.includes('qwen')
                  ? '#c084fc'
                  : '#60a5fa',
              }}
            >
              {getModelDisplayName(
                selectedTrader.ai_model.split('_').pop() ||
                  selectedTrader.ai_model
              )}
            </span>
          </span>
          <span>•</span>
          <span>
            Exchange:{' '}
            <span className="font-semibold" style={{ color: '#EAECEF' }}>
              {getExchangeDisplayNameFromList(
                selectedTrader.exchange_id,
                exchanges
              )}
            </span>
          </span>
          <span>•</span>
          <span>
            Strategy:{' '}
            <span className="font-semibold" style={{ color: '#F0B90B' }}>
              {selectedTrader.strategy_name || 'No Strategy'}
            </span>
          </span>
          {status && (
            <>
              <span>•</span>
              <span>Cycles: {status.call_count}</span>
              <span>•</span>
              <span>Runtime: {status.runtime_minutes} min</span>
            </>
          )}
        </div>
      </div>

      {/* Debug Info */}
      {account && (
        <div
          className="mb-4 p-3 rounded text-xs font-mono"
          style={{ background: '#1E2329', border: '1px solid #2B3139' }}
        >
          <div style={{ color: '#848E9C' }}>
            🔄 Last Update: {lastUpdate} | Total Equity:{' '}
            {account?.total_equity?.toFixed(2) || '0.00'} | Available:{' '}
            {account?.available_balance?.toFixed(2) || '0.00'} | P&L:{' '}
            {account?.total_pnl?.toFixed(2) || '0.00'} (
            {account?.total_pnl_pct?.toFixed(2) || '0.00'}%)
          </div>
        </div>
      )}

      {/* Account Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
        <StatCard
          title={t('totalEquity', language)}
          value={`${account?.total_equity?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) > 0}
        />
        <StatCard
          title={t('availableBalance', language)}
          value={`${account?.available_balance?.toFixed(2) || '0.00'} USDT`}
          subtitle={`${account?.available_balance && account?.total_equity ? ((account.available_balance / account.total_equity) * 100).toFixed(1) : '0.0'}% ${t('free', language)}`}
        />
        <StatCard
          title={t('totalPnL', language)}
          value={`${account?.total_pnl !== undefined && account.total_pnl >= 0 ? '+' : ''}${account?.total_pnl?.toFixed(2) || '0.00'} USDT`}
          change={account?.total_pnl_pct || 0}
          positive={(account?.total_pnl ?? 0) >= 0}
        />
        <StatCard
          title={t('positions', language)}
          value={`${account?.position_count || 0}`}
          subtitle={`${t('margin', language)}: ${account?.margin_used_pct?.toFixed(1) || '0.0'}%`}
        />
      </div>

      {/* 主要内容区：左右分屏 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* 左侧：图表 + 持仓 */}
        <div className="space-y-6">
          {/* Chart Tabs (Equity / K-line) */}
          <div
            ref={chartSectionRef}
            className="chart-container animate-slide-in scroll-mt-32"
            style={{ animationDelay: '0.1s' }}
          >
            <ChartTabs
              traderId={selectedTrader.trader_id}
              selectedSymbol={selectedChartSymbol}
              updateKey={chartUpdateKey}
              exchangeId={getExchangeTypeFromList(
                selectedTrader.exchange_id,
                exchanges
              )}
            />
          </div>

          {/* Current Positions */}
          <div
            className="binance-card p-6 animate-slide-in"
            style={{ animationDelay: '0.15s' }}
          >
            <div className="flex items-center justify-between mb-5">
              <h2
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                📈 {t('currentPositions', language)}
              </h2>
              {positions && positions.length > 0 && (
                <div
                  className="text-xs px-3 py-1 rounded"
                  style={{
                    background: 'rgba(240, 185, 11, 0.1)',
                    color: '#F0B90B',
                    border: '1px solid rgba(240, 185, 11, 0.2)',
                  }}
                >
                  {positions.length} {t('active', language)}
                </div>
              )}
            </div>
            {positions && positions.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead className="text-left border-b border-gray-800">
                    <tr>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-left">
                        {t('symbol', language)}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                        {t('side', language)}
                      </th>
                      <th className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center">
                        {language === 'zh' ? '操作' : 'Action'}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('entryPrice', language)}
                      >
                        {language === 'zh' ? '入场价' : 'Entry'}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('markPrice', language)}
                      >
                        {language === 'zh' ? '标记价' : 'Mark'}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('quantity', language)}
                      >
                        {language === 'zh' ? '数量' : 'Qty'}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('positionValue', language)}
                      >
                        {language === 'zh' ? '价值' : 'Value'}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-center"
                        title={t('leverage', language)}
                      >
                        {language === 'zh' ? '杠杆' : 'Lev.'}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('unrealizedPnL', language)}
                      >
                        {language === 'zh' ? '未实现盈亏' : 'uPnL'}
                      </th>
                      <th
                        className="px-1 pb-3 font-semibold text-gray-400 whitespace-nowrap text-right"
                        title={t('liqPrice', language)}
                      >
                        {language === 'zh' ? '强平价' : 'Liq.'}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {positions.map((pos, i) => (
                      <tr
                        key={i}
                        className="border-b border-gray-800 last:border-0 transition-colors hover:bg-opacity-10 hover:bg-yellow-500 cursor-pointer"
                        onClick={() => {
                          setSelectedChartSymbol(pos.symbol)
                          setChartUpdateKey(Date.now())
                          // Smooth scroll to chart with ref
                          if (chartSectionRef.current) {
                            chartSectionRef.current.scrollIntoView({
                              behavior: 'smooth',
                              block: 'start',
                            })
                          }
                        }}
                      >
                        <td className="px-1 py-3 font-mono font-semibold whitespace-nowrap text-left">
                          {pos.symbol}
                        </td>
                        <td className="px-1 py-3 whitespace-nowrap text-center">
                          <span
                            className="px-1.5 py-0.5 rounded text-[10px] font-bold"
                            style={
                              pos.side === 'long'
                                ? {
                                    background: 'rgba(14, 203, 129, 0.1)',
                                    color: '#0ECB81',
                                  }
                                : {
                                    background: 'rgba(246, 70, 93, 0.1)',
                                    color: '#F6465D',
                                  }
                            }
                          >
                            {t(
                              pos.side === 'long' ? 'long' : 'short',
                              language
                            )}
                          </span>
                        </td>
                        <td className="px-1 py-3 whitespace-nowrap text-center">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation() // Prevent row click
                              handleClosePosition(
                                pos.symbol,
                                pos.side.toUpperCase()
                              )
                            }}
                            disabled={closingPosition === pos.symbol}
                            className="btn-danger inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-semibold transition-all hover:scale-105 disabled:opacity-50 disabled:cursor-not-allowed mx-auto"
                            title={
                              language === 'zh' ? '平仓' : 'Close Position'
                            }
                          >
                            {closingPosition === pos.symbol ? (
                              <Loader2 className="w-3 h-3 animate-spin" />
                            ) : (
                              <LogOut className="w-3 h-3" />
                            )}
                            {language === 'zh' ? '平仓' : 'Close'}
                          </button>
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.entry_price.toFixed(4)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.mark_price.toFixed(4)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {pos.quantity.toFixed(4)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono font-bold whitespace-nowrap text-right"
                          style={{ color: '#EAECEF' }}
                        >
                          {(pos.quantity * pos.mark_price).toFixed(2)}
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-center"
                          style={{ color: '#F0B90B' }}
                        >
                          {pos.leverage}x
                        </td>
                        <td className="px-1 py-3 font-mono whitespace-nowrap text-right">
                          <span
                            style={{
                              color:
                                pos.unrealized_pnl >= 0 ? '#0ECB81' : '#F6465D',
                              fontWeight: 'bold',
                            }}
                          >
                            {pos.unrealized_pnl >= 0 ? '+' : ''}
                            {pos.unrealized_pnl.toFixed(2)}
                          </span>
                        </td>
                        <td
                          className="px-1 py-3 font-mono whitespace-nowrap text-right"
                          style={{ color: '#848E9C' }}
                        >
                          {pos.liquidation_price.toFixed(4)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="text-center py-16" style={{ color: '#848E9C' }}>
                <div className="text-6xl mb-4 opacity-50">📊</div>
                <div className="text-lg font-semibold mb-2">
                  {t('noPositions', language)}
                </div>
                <div className="text-sm">
                  {t('noActivePositions', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 左侧结束 */}

        {/* 右侧：Recent Decisions - 卡片容器 */}
        <div
          className="binance-card p-6 animate-slide-in h-fit lg:sticky lg:top-24 lg:max-h-[calc(100vh-120px)]"
          style={{ animationDelay: '0.2s' }}
        >
          {/* 标题 */}
          <div
            className="flex items-center gap-3 mb-5 pb-4 border-b"
            style={{ borderColor: '#2B3139' }}
          >
            <div
              className="w-10 h-10 rounded-xl flex items-center justify-center text-xl"
              style={{
                background: 'linear-gradient(135deg, #6366F1 0%, #8B5CF6 100%)',
                boxShadow: '0 4px 14px rgba(99, 102, 241, 0.4)',
              }}
            >
              🧠
            </div>
            <div className="flex-1">
              <h2 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
                {t('recentDecisions', language)}
              </h2>
              {decisions && decisions.length > 0 && (
                <div className="text-xs" style={{ color: '#848E9C' }}>
                  {t('lastCycles', language, { count: decisions.length })}
                </div>
              )}
            </div>
            {/* 数量选择器 */}
            <select
              value={decisionsLimit}
              onChange={(e) => onDecisionsLimitChange(Number(e.target.value))}
              className="px-3 py-1.5 rounded-lg text-sm font-medium cursor-pointer transition-all"
              style={{
                background: '#2B3139',
                color: '#EAECEF',
                border: '1px solid #3C4043',
              }}
            >
              <option value={5}>5</option>
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
          </div>

          {/* 决策列表 - 可滚动 */}
          <div
            className="space-y-4 overflow-y-auto pr-2"
            style={{ maxHeight: 'calc(100vh - 280px)' }}
          >
            {decisions && decisions.length > 0 ? (
              decisions.map((decision, i) => (
                <DecisionCard key={i} decision={decision} language={language} />
              ))
            ) : (
              <div className="py-16 text-center">
                <div className="text-6xl mb-4 opacity-30">🧠</div>
                <div
                  className="text-lg font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('noDecisionsYet', language)}
                </div>
                <div className="text-sm" style={{ color: '#848E9C' }}>
                  {t('aiDecisionsWillAppear', language)}
                </div>
              </div>
            )}
          </div>
        </div>
        {/* 右侧结束 */}
      </div>
    </div>
  )
}

// Stat Card Component - Binance Style Enhanced
function StatCard({
  title,
  value,
  change,
  positive,
  subtitle,
}: {
  title: string
  value: string
  change?: number
  positive?: boolean
  subtitle?: string
}) {
  return (
    <div className="stat-card animate-fade-in">
      <div
        className="text-xs mb-2 mono uppercase tracking-wider"
        style={{ color: '#848E9C' }}
      >
        {title}
      </div>
      <div
        className="text-2xl font-bold mb-1 mono"
        style={{ color: '#EAECEF' }}
      >
        {value}
      </div>
      {change !== undefined && (
        <div className="flex items-center gap-1">
          <div
            className="text-sm mono font-bold"
            style={{ color: positive ? '#0ECB81' : '#F6465D' }}
          >
            {positive ? '▲' : '▼'} {positive ? '+' : ''}
            {change.toFixed(2)}%
          </div>
        </div>
      )}
      {subtitle && (
        <div className="text-xs mt-2 mono" style={{ color: '#848E9C' }}>
          {subtitle}
        </div>
      )}
    </div>
  )
}

// Wrap App with providers
export default function AppWithProviders() {
  return (
    <LanguageProvider>
      <AuthProvider>
        <ConfirmDialogProvider>
          <App />
        </ConfirmDialogProvider>
      </AuthProvider>
    </LanguageProvider>
  )
}
