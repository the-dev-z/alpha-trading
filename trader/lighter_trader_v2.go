package trader

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strings"
	"sync"
	"time"

	lighterClient "github.com/elliottech/lighter-go/client"
	lighterHTTP "github.com/elliottech/lighter-go/client/http"
	"github.com/elliottech/lighter-go/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

// AccountInfo LIGHTER account information
type AccountInfo struct {
	AccountIndex int64  `json:"account_index"`
	L1Address    string `json:"l1_address"`
	// Other fields can be added based on actual API response
}

// LighterTraderV2 New implementation using official lighter-go SDK
type LighterTraderV2 struct {
	ctx        context.Context
	privateKey *ecdsa.PrivateKey // L1 wallet private key (for account identification)
	walletAddr string            // Ethereum wallet address

	client  *http.Client
	baseURL string
	testnet bool
	chainID uint32

	// SDK clients
	httpClient lighterClient.MinimalHTTPClient
	txClient   *lighterClient.TxClient

	// API Key management
	apiKeyPrivateKey string // 40-byte API Key private key (for signing transactions)
	apiKeyIndex      uint8  // API Key index (default 0)
	accountIndex     int64  // Account index

	// Authentication token
	authToken    string
	tokenExpiry  time.Time
	accountMutex sync.RWMutex

	// Market info cache
	symbolPrecision map[string]SymbolPrecision
	precisionMutex  sync.RWMutex

	// Market index cache
	marketIndexMap map[string]int16 // symbol -> market_id
	marketMutex    sync.RWMutex

	// Runtime settings cache (per-symbol)
	settingsMutex   sync.RWMutex
	leverageCache   map[string]int
	marginModeCache map[string]uint8

	// Client order index generator (avoid collisions within a millisecond)
	clientOrderMutex     sync.Mutex
	lastClientOrderIndex int64
}

// NewLighterTraderV2 Create new LIGHTER trader (using official SDK)
// Parameters:
//   - l1PrivateKeyHex: L1 wallet private key (32 bytes, optional - only needed for GenerateAndRegisterAPIKey)
//   - walletAddr: Ethereum wallet address (required if l1PrivateKeyHex is empty)
//   - apiKeyPrivateKeyHex: API Key private key (40 bytes, for signing transactions) - required
//   - testnet: Whether to use testnet
func NewLighterTraderV2(l1PrivateKeyHex, walletAddr, apiKeyPrivateKeyHex string, testnet bool) (*LighterTraderV2, error) {
	var l1PrivateKey *ecdsa.PrivateKey

	// 1. Parse L1 private key (optional - only needed for GenerateAndRegisterAPIKey)
	if l1PrivateKeyHex != "" {
		l1PrivateKeyHex = strings.TrimPrefix(strings.ToLower(l1PrivateKeyHex), "0x")
		var err error
		l1PrivateKey, err = crypto.HexToECDSA(l1PrivateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("invalid L1 private key: %w", err)
		}

		// If wallet address not provided, derive from private key
		if walletAddr == "" {
			walletAddr = crypto.PubkeyToAddress(*l1PrivateKey.Public().(*ecdsa.PublicKey)).Hex()
			logger.Infof("✓ Derived wallet address from private key: %s", walletAddr)
		}
	}

	// 2. Wallet address is required
	if walletAddr == "" {
		return nil, fmt.Errorf("wallet address is required (either provide directly or via L1 private key)")
	}

	// 3. Determine API URL and Chain ID
	baseURL := "https://mainnet.zklighter.elliot.ai"
	chainID := uint32(42766) // Mainnet Chain ID
	if testnet {
		baseURL = "https://testnet.zklighter.elliot.ai"
		chainID = uint32(42069) // Testnet Chain ID
	}

	// 4. Create HTTP client
	httpClient := lighterHTTP.NewClient(baseURL)

	trader := &LighterTraderV2{
		ctx:              context.Background(),
		privateKey:       l1PrivateKey,
		walletAddr:       walletAddr,
		client:           &http.Client{Timeout: 30 * time.Second},
		baseURL:          baseURL,
		testnet:          testnet,
		chainID:          chainID,
		httpClient:       httpClient,
		apiKeyPrivateKey: apiKeyPrivateKeyHex,
		apiKeyIndex:      0, // Default to index 0
		symbolPrecision:  make(map[string]SymbolPrecision),
		marketIndexMap:   make(map[string]int16),
		leverageCache:    make(map[string]int),
		marginModeCache:  make(map[string]uint8),
	}

	// 5. Initialize account (get account index)
	if err := trader.initializeAccount(); err != nil {
		return nil, fmt.Errorf("failed to initialize account: %w", err)
	}

	// 6. If no API Key, prompt user to generate one
	if apiKeyPrivateKeyHex == "" {
		logger.Infof("⚠️  No API Key private key provided, please call GenerateAndRegisterAPIKey() to generate")
		logger.Infof("   Or get an existing API Key from LIGHTER website")
		return trader, nil
	}

	// 7. Create TxClient (for signing transactions)
	txClient, err := lighterClient.NewTxClient(
		httpClient,
		apiKeyPrivateKeyHex,
		trader.accountIndex,
		trader.apiKeyIndex,
		trader.chainID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create TxClient: %w", err)
	}

	trader.txClient = txClient

	// 8. Verify API Key is correct
	if err := trader.checkClient(); err != nil {
		logger.Infof("⚠️  API Key verification failed: %v", err)
		logger.Infof("   You may need to regenerate API Key or check configuration")
		return trader, err
	}

	logger.Infof("✓ LIGHTER trader initialized successfully (account=%d, apiKey=%d, testnet=%v)",
		trader.accountIndex, trader.apiKeyIndex, testnet)

	return trader, nil
}

// initializeAccount Initialize account information (get account index)
func (t *LighterTraderV2) initializeAccount() error {
	// Get account info by L1 address
	accountInfo, err := t.getAccountByL1Address()
	if err != nil {
		return fmt.Errorf("failed to get account info: %w", err)
	}

	t.accountMutex.Lock()
	t.accountIndex = accountInfo.AccountIndex
	t.accountMutex.Unlock()

	logger.Infof("✓ Account index: %d", t.accountIndex)
	return nil
}

// getAccountByL1Address Get LIGHTER account info by L1 wallet address
func (t *LighterTraderV2) getAccountByL1Address() (*AccountInfo, error) {
	endpoints := []string{
		fmt.Sprintf("%s/api/v1/account?by=l1_address&value=%s", t.baseURL, t.walletAddr),
		fmt.Sprintf("%s/api/v1/account/by/l1/%s", t.baseURL, t.walletAddr), // legacy
	}

	var lastErr error
	for _, endpoint := range endpoints {
		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		resp, err := t.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			continue
		}

		info, err := parseAccountInfo(body)
		if err != nil {
			lastErr = err
			continue
		}
		if info.AccountIndex == 0 {
			lastErr = fmt.Errorf("missing account_index in response: %s", string(body))
			continue
		}
		return info, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown error")
	}
	return nil, fmt.Errorf("failed to get account: %w", lastErr)
}

// checkClient Verify if API Key is correct
func (t *LighterTraderV2) checkClient() error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get API Key public key registered on server
	publicKey, err := t.httpClient.GetApiKey(t.accountIndex, t.apiKeyIndex)
	if err != nil {
		return fmt.Errorf("failed to get API Key: %w", err)
	}

	// Get local API Key public key
	pubKeyBytes := t.txClient.GetKeyManager().PubKeyBytes()
	localPubKey := hexutil.Encode(pubKeyBytes[:])
	localPubKey = strings.Replace(localPubKey, "0x", "", 1)

	// Compare public keys
	if publicKey != localPubKey {
		return fmt.Errorf("API Key mismatch: local=%s, server=%s", localPubKey, publicKey)
	}

	logger.Infof("✓ API Key verification passed")
	return nil
}

// GenerateAndRegisterAPIKey Generate new API Key and register to LIGHTER
// Note: This requires L1 private key signature, so must be called with L1 private key available
func (t *LighterTraderV2) GenerateAndRegisterAPIKey(seed string) (privateKey, publicKey string, err error) {
	if t.privateKey == nil {
		return "", "", fmt.Errorf("L1 private key not initialized")
	}

	// Ensure we know the account index.
	if t.accountIndex == 0 {
		if err := t.initializeAccount(); err != nil {
			return "", "", fmt.Errorf("failed to initialize account: %w", err)
		}
	}

	if t.httpClient == nil {
		t.httpClient = lighterHTTP.NewClient(t.baseURL)
	}
	if t.httpClient == nil {
		return "", "", fmt.Errorf("http client not initialized")
	}

	apiKeyBytes, err := generateAPIKeyBytes(seed)
	if err != nil {
		return "", "", err
	}
	apiKeyHex := hexutil.Encode(apiKeyBytes)

	// Create a temporary TxClient for this new API key.
	txClient, err := lighterClient.NewTxClient(
		t.httpClient,
		apiKeyHex,
		t.accountIndex,
		t.apiKeyIndex,
		t.chainID,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to create TxClient: %w", err)
	}

	pubKeyBytes := txClient.GetKeyManager().PubKeyBytes()
	pubKeyHex := hexutil.Encode(pubKeyBytes[:])

	// Sign the ChangePubKey tx (and embed L1 signature) to register this API key on-chain/server-side.
	nonce := int64(-1) // auto-fetch
	txInfo, err := txClient.GetChangePubKeyTransaction(&types.ChangePubKeyReq{
		PubKey: pubKeyBytes,
	}, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to sign change pubkey tx: %w", err)
	}

	l1Sig, err := signPersonalMessage(t.privateKey, txInfo.GetL1SignatureBody())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign L1 message: %w", err)
	}
	txInfo.L1Sig = l1Sig

	if _, err := t.submitTx(txInfo, false); err != nil {
		return "", "", fmt.Errorf("failed to submit change pubkey tx: %w", err)
	}

	// Activate this API key locally.
	t.apiKeyPrivateKey = apiKeyHex
	t.txClient = txClient
	_ = t.refreshAuthToken()

	logger.Infof("✓ LIGHTER API key registered (account=%d apiKey=%d)", t.accountIndex, t.apiKeyIndex)

	return apiKeyHex, pubKeyHex, nil
}

func signPersonalMessage(privateKey *ecdsa.PrivateKey, message string) (string, error) {
	if privateKey == nil {
		return "", fmt.Errorf("nil private key")
	}

	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(message))
	prefixedMessage := append([]byte(prefix), []byte(message)...)

	hash := crypto.Keccak256Hash(prefixedMessage)
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", err
	}

	// Adjust v value (Ethereum format)
	if signature[64] < 27 {
		signature[64] += 27
	}

	return hexutil.Encode(signature), nil
}

func generateAPIKeyBytes(seed string) ([]byte, error) {
	apiKeyBytes := make([]byte, 40)
	if strings.TrimSpace(seed) == "" {
		if _, err := rand.Read(apiKeyBytes); err != nil {
			return nil, err
		}
		return apiKeyBytes, nil
	}

	sum := sha512.Sum512([]byte(seed))
	copy(apiKeyBytes, sum[:40])
	// Avoid the all-zero key.
	zero := true
	for _, b := range apiKeyBytes {
		if b != 0 {
			zero = false
			break
		}
	}
	if zero {
		apiKeyBytes[0] = 1
	}

	return apiKeyBytes, nil
}

func parseAccountInfo(body []byte) (*AccountInfo, error) {
	// 1) Plain object
	var direct AccountInfo
	if err := json.Unmarshal(body, &direct); err == nil && direct.AccountIndex != 0 {
		return &direct, nil
	}

	// 2) Wrapped object: { code, message, data }
	var wrapped struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil && wrapped.Data != nil {
		if wrapped.Code != 0 && wrapped.Code != 200 {
			return nil, fmt.Errorf("api error (code %d): %s", wrapped.Code, wrapped.Message)
		}

		var inner AccountInfo
		if err := json.Unmarshal(wrapped.Data, &inner); err == nil && inner.AccountIndex != 0 {
			return &inner, nil
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(wrapped.Data, &raw); err == nil {
			if idx, ok := getAccountIndexFromMap(raw); ok {
				return &AccountInfo{AccountIndex: idx}, nil
			}
		}
	}

	// 3) Map fallback (legacy: index)
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse account response: %w", err)
	}
	if idx, ok := getAccountIndexFromMap(raw); ok {
		return &AccountInfo{AccountIndex: idx}, nil
	}

	return nil, fmt.Errorf("failed to extract account_index from response: %s", string(body))
}

func getAccountIndexFromMap(raw map[string]interface{}) (int64, bool) {
	for _, key := range []string{"account_index", "accountIndex", "index"} {
		v, ok := raw[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case float64:
			return int64(val), true
		case int:
			return int64(val), true
		case int64:
			return val, true
		case string:
			var parsed int64
			if _, err := fmt.Sscanf(val, "%d", &parsed); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

// refreshAuthToken Refresh authentication token (using official SDK)
func (t *LighterTraderV2) refreshAuthToken() error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized, please set API Key first")
	}

	// Generate auth token using official SDK (valid for 7 hours)
	deadline := time.Now().Add(7 * time.Hour)
	authToken, err := t.txClient.GetAuthToken(deadline)
	if err != nil {
		return fmt.Errorf("failed to generate auth token: %w", err)
	}

	t.accountMutex.Lock()
	t.authToken = authToken
	t.tokenExpiry = deadline
	t.accountMutex.Unlock()

	logger.Infof("✓ Auth token generated (valid until: %s)", t.tokenExpiry.Format(time.RFC3339))
	return nil
}

// ensureAuthToken Ensure authentication token is valid
func (t *LighterTraderV2) ensureAuthToken() error {
	t.accountMutex.RLock()
	expired := time.Now().After(t.tokenExpiry.Add(-30 * time.Minute)) // Refresh 30 minutes early
	t.accountMutex.RUnlock()

	if expired {
		logger.Info("🔄 Auth token about to expire, refreshing...")
		return t.refreshAuthToken()
	}

	return nil
}

// GetExchangeType Get exchange type
func (t *LighterTraderV2) GetExchangeType() string {
	return "lighter"
}

// Cleanup Clean up resources
func (t *LighterTraderV2) Cleanup() error {
	logger.Info("⏹  LIGHTER trader cleanup completed")
	return nil
}

// GetClosedPnL gets closed position PnL records from exchange
// LIGHTER does not have a direct closed PnL API, returns empty slice
func (t *LighterTraderV2) GetClosedPnL(startTime time.Time, limit int) ([]ClosedPnLRecord, error) {
	trades, err := t.GetTrades(startTime, limit)
	if err != nil {
		return nil, err
	}

	// Filter only closing trades (realizedPnl != 0)
	var records []ClosedPnLRecord
	for _, trade := range trades {
		if trade.RealizedPnL == 0 {
			continue
		}

		side := "long"
		if trade.Side == "SELL" || trade.Side == "Sell" {
			side = "long"
		} else {
			side = "short"
		}

		var entryPrice float64
		if trade.Quantity > 0 {
			if side == "long" {
				entryPrice = trade.Price - trade.RealizedPnL/trade.Quantity
			} else {
				entryPrice = trade.Price + trade.RealizedPnL/trade.Quantity
			}
		}

		records = append(records, ClosedPnLRecord{
			Symbol:      trade.Symbol,
			Side:        side,
			EntryPrice:  entryPrice,
			ExitPrice:   trade.Price,
			Quantity:    trade.Quantity,
			RealizedPnL: trade.RealizedPnL,
			Fee:         trade.Fee,
			ExitTime:    trade.Time,
			EntryTime:   trade.Time,
			OrderID:     trade.TradeID,
			ExchangeID:  trade.TradeID,
			CloseType:   "unknown",
		})
	}

	return records, nil
}

// GetTrades retrieves trade history from Lighter
func (t *LighterTraderV2) GetTrades(startTime time.Time, limit int) ([]TradeRecord, error) {
	// Ensure we have account index
	if t.accountIndex == 0 {
		if err := t.initializeAccount(); err != nil {
			return nil, fmt.Errorf("failed to get account index: %w", err)
		}
	}

	// Build request URL
	startTimeMs := startTime.UnixMilli()
	endpoint := fmt.Sprintf("%s/api/v1/trades?account_index=%d&start_time=%d",
		t.baseURL, t.accountIndex, startTimeMs)
	if limit > 0 {
		endpoint = fmt.Sprintf("%s&limit=%d", endpoint, limit)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get trades: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Infof("⚠️  Lighter trades API returned %d: %s", resp.StatusCode, string(body))
		return []TradeRecord{}, nil
	}

	var response LighterTradeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		var trades []LighterTrade
		if err := json.Unmarshal(body, &trades); err != nil {
			logger.Infof("⚠️  Failed to parse Lighter trades response: %v", err)
			return []TradeRecord{}, nil
		}
		response.Trades = trades
	}

	// Convert to unified TradeRecord format
	var result []TradeRecord
	for _, lt := range response.Trades {
		price, _ := parseFloat(lt.Price)
		qty, _ := parseFloat(lt.Size)
		fee, _ := parseFloat(lt.Fee)
		pnl, _ := parseFloat(lt.RealizedPnl)

		var side string
		if strings.ToLower(lt.Side) == "buy" {
			side = "BUY"
		} else {
			side = "SELL"
		}

		trade := TradeRecord{
			TradeID:      lt.TradeID,
			Symbol:       lt.Symbol,
			Side:         side,
			PositionSide: "BOTH",
			Price:        price,
			Quantity:     qty,
			RealizedPnL:  pnl,
			Fee:          fee,
			Time:         time.UnixMilli(lt.Timestamp),
		}
		result = append(result, trade)
	}

	return result, nil
}
