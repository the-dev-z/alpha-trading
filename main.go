package main

import (
	"nofx/api"
	"nofx/auth"
	"nofx/backtest"
	"nofx/config"
	"nofx/crypto"
	"nofx/logger"
	"nofx/manager"
	"nofx/mcp"
	"nofx/store"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env environment variables
	_ = godotenv.Load()

	// Initialize logger
	logger.Init(nil)

	logger.Info("╔════════════════════════════════════════════════════════════╗")
	logger.Info("║           🚀 NOFX - AI-Powered Trading System              ║")
	logger.Info("╚════════════════════════════════════════════════════════════╝")

	// Initialize global configuration (loaded from .env)
	config.Init()
	cfg := config.Get()
	logger.Info("✅ Configuration loaded")

	// Initialize database
	// Default path is data/data.db to work with Docker volume mount (/app/data)
	dbPath := "data/data.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	// Ensure data directory exists
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Errorf("Failed to create data directory: %v", err)
		}
	}

	logger.Infof("📋 Initializing database: %s", dbPath)
	st, err := store.New(dbPath)
	if err != nil {
		logger.Fatalf("❌ Failed to initialize database: %v", err)
	}
	defer st.Close()
	backtest.UseDatabase(st.DB())


	// Initialize encryption service
	logger.Info("🔐 Initializing encryption service...")
	cryptoService, err := crypto.NewCryptoService()
	if err != nil {
		logger.Fatalf("❌ Failed to initialize encryption service: %v", err)
	}
	encryptFunc := func(plaintext string) string {
		if plaintext == "" {
			return plaintext
		}
		encrypted, err := cryptoService.EncryptForStorage(plaintext)
		if err != nil {
			logger.Warnf("⚠️ Encryption failed: %v", err)
			return plaintext
		}
		return encrypted
	}
	decryptFunc := func(encrypted string) string {
		if encrypted == "" {
			return encrypted
		}
		if !cryptoService.IsEncryptedStorageValue(encrypted) {
			return encrypted
		}
		decrypted, err := cryptoService.DecryptFromStorage(encrypted)
		if err != nil {
			logger.Warnf("⚠️ Decryption failed: %v", err)
			return encrypted
		}
		return decrypted
	}
	st.SetCryptoFuncs(encryptFunc, decryptFunc)
	logger.Info("✅ Encryption service initialized successfully")

	// Set JWT secret
	auth.SetJWTSecret(cfg.JWTSecret)
	logger.Info("🔑 JWT secret configured")

	// WebSocket market monitor is NO LONGER USED
	// All K-line data now comes from CoinAnk API instead of Binance WebSocket cache
	// Commented out to reduce unnecessary connections:
	// go market.NewWSMonitor(150).Start(nil)
	// logger.Info("📊 WebSocket market monitor started")
	// time.Sleep(500 * time.Millisecond)
	logger.Info("📊 Using CoinAnk API for all market data (WebSocket cache disabled)")

	// Create TraderManager and BacktestManager
	traderManager := manager.NewTraderManager()
	mcpClient := newSharedMCPClient()
	backtestManager := backtest.NewManager(mcpClient)
	if err := backtestManager.RestoreRuns(); err != nil {
		logger.Warnf("⚠️ Failed to restore backtest history: %v", err)
	}

	// Load all traders from database to memory (may auto-start traders with IsRunning=true)
	if err := traderManager.LoadTradersFromStore(st); err != nil {
		logger.Fatalf("❌ Failed to load traders: %v", err)
	}

	// Display loaded trader information
	traders, err := st.Trader().List("default")
	if err != nil {
		logger.Fatalf("❌ Failed to get trader list: %v", err)
	}

	logger.Info("🤖 AI Trader Configurations in Database:")
	if len(traders) == 0 {
		logger.Info("  (No trader configurations, please create via Web interface)")
	} else {
		for _, t := range traders {
			status := "❌ Stopped"
			if t.IsRunning {
				status = "✅ Running"
			}
			logger.Infof("  • %s [%s] %s - AI Model: %s, Exchange: %s",
				t.Name, t.ID[:8], status, t.AIModelID, t.ExchangeID)
		}
	}

	// Start API server
	server := api.NewServer(traderManager, st, cryptoService, backtestManager, cfg.APIServerPort)
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatalf("❌ Failed to start API server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("✅ System started successfully, waiting for trading commands...")
	logger.Info("📌 Tip: Use Ctrl+C to stop the system")

	<-quit
	logger.Info("📴 Shutdown signal received, closing system...")

	// Stop all traders
	traderManager.StopAll()
	logger.Info("✅ System shut down safely")
}

// newSharedMCPClient creates a shared MCP AI client (for backtesting)
func newSharedMCPClient() mcp.AIClient {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		logger.Warn("⚠️ DEEPSEEK_API_KEY not set, AI features will be unavailable")
		return nil
	}
	return mcp.NewDeepSeekClient()
}

