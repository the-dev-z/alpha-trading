package trader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"nofx/logger"
	"strings"
	"time"

	"github.com/elliottech/lighter-go/types"
	"github.com/elliottech/lighter-go/types/txtypes"
)

const (
	lighterBaseAmountScale    = 1e8
	lighterPriceScale         = 1e2
	lighterDefaultOrderExpiry = 28 * 24 * time.Hour
	lighterDefaultSlippageBps = 500 // 5%
)

// OpenLong Open long position (implements Trader interface)
func (t *LighterTraderV2) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized, please set API Key first")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("invalid quantity: %.8f", quantity)
	}
	if leverage <= 0 {
		return nil, fmt.Errorf("invalid leverage: %d", leverage)
	}

	logger.Infof("📈 LIGHTER opening long: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// 1. Set leverage (if needed)
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️  Failed to set leverage: %v", err)
	}

	// 2. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 3. Create market buy order (open long)
	orderResult, err := t.CreateOrder(symbol, false, quantity, marketPrice, "market", false)
	if err != nil {
		return nil, fmt.Errorf("failed to open long: %w", err)
	}

	logger.Infof("✓ LIGHTER opened long successfully: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "long",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// OpenShort Open short position (implements Trader interface)
func (t *LighterTraderV2) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized, please set API Key first")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("invalid quantity: %.8f", quantity)
	}
	if leverage <= 0 {
		return nil, fmt.Errorf("invalid leverage: %d", leverage)
	}

	logger.Infof("📉 LIGHTER opening short: %s, qty=%.4f, leverage=%dx", symbol, quantity, leverage)

	// 1. Set leverage
	if err := t.SetLeverage(symbol, leverage); err != nil {
		logger.Infof("⚠️  Failed to set leverage: %v", err)
	}

	// 2. Get market price
	marketPrice, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market price: %w", err)
	}

	// 3. Create market sell order (open short)
	orderResult, err := t.CreateOrder(symbol, true, quantity, marketPrice, "market", false)
	if err != nil {
		return nil, fmt.Errorf("failed to open short: %w", err)
	}

	logger.Infof("✓ LIGHTER opened short successfully: %s @ %.2f", symbol, marketPrice)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"side":    "short",
		"status":  "FILLED",
		"price":   marketPrice,
	}, nil
}

// CloseLong Close long position (implements Trader interface)
func (t *LighterTraderV2) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// If quantity=0, get current position quantity
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	logger.Infof("🔻 LIGHTER closing long: %s, qty=%.4f", symbol, quantity)

	// Create market sell order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, true, quantity, 0, "market", true)
	if err != nil {
		return nil, fmt.Errorf("failed to close long: %w", err)
	}

	// Cancel all open orders after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel orders: %v", err)
	}

	logger.Infof("✓ LIGHTER closed long successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CloseShort Close short position (implements Trader interface)
func (t *LighterTraderV2) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}

	// If quantity=0, get current position quantity
	if quantity == 0 {
		pos, err := t.GetPosition(symbol)
		if err != nil {
			return nil, fmt.Errorf("failed to get position: %w", err)
		}
		if pos == nil || pos.Size == 0 {
			return map[string]interface{}{
				"symbol": symbol,
				"status": "NO_POSITION",
			}, nil
		}
		quantity = pos.Size
	}

	logger.Infof("🔺 LIGHTER closing short: %s, qty=%.4f", symbol, quantity)

	// Create market buy order to close (reduceOnly=true)
	orderResult, err := t.CreateOrder(symbol, false, quantity, 0, "market", true)
	if err != nil {
		return nil, fmt.Errorf("failed to close short: %w", err)
	}

	// Cancel all open orders after closing position
	if err := t.CancelAllOrders(symbol); err != nil {
		logger.Infof("⚠️  Failed to cancel orders: %v", err)
	}

	logger.Infof("✓ LIGHTER closed short successfully: %s", symbol)

	return map[string]interface{}{
		"orderId": orderResult["orderId"],
		"symbol":  symbol,
		"status":  "FILLED",
	}, nil
}

// CreateOrder Create order (market or limit) - uses official SDK for signing
func (t *LighterTraderV2) CreateOrder(symbol string, isAsk bool, quantity float64, price float64, orderType string, reduceOnly bool) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("invalid quantity: %.8f", quantity)
	}

	marketIndex, err := t.getSDKMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market index: %w", err)
	}

	clientOrderIndex := t.nextClientOrderIndex()

	baseAmount, err := toLighterBaseAmount(quantity)
	if err != nil {
		return nil, err
	}

	var orderTypeValue uint8
	var timeInForce uint8
	var triggerPrice uint32 = txtypes.NilOrderTriggerPrice
	var orderExpiry int64
	var priceValue uint32

	switch strings.ToLower(orderType) {
	case "market":
		orderTypeValue = txtypes.MarketOrder
		timeInForce = txtypes.ImmediateOrCancel
		orderExpiry = txtypes.NilOrderExpiry
		triggerPrice = txtypes.NilOrderTriggerPrice

		refPrice := price
		if refPrice <= 0 {
			refPrice, err = t.GetMarketPrice(symbol)
			if err != nil {
				return nil, fmt.Errorf("failed to get market price: %w", err)
			}
		}

		slippageFactor := float64(lighterDefaultSlippageBps) / 10_000.0
		if isAsk {
			priceValue, err = toLighterPriceTicksFloor(refPrice * (1 - slippageFactor))
		} else {
			priceValue, err = toLighterPriceTicksCeil(refPrice * (1 + slippageFactor))
		}
		if err != nil {
			return nil, err
		}

	case "limit":
		if price <= 0 {
			return nil, fmt.Errorf("invalid limit price: %.8f", price)
		}

		orderTypeValue = txtypes.LimitOrder
		timeInForce = txtypes.GoodTillTime
		triggerPrice = txtypes.NilOrderTriggerPrice
		orderExpiry = time.Now().Add(lighterDefaultOrderExpiry).UnixMilli()

		// For buys, round down to avoid paying more; for sells, round up.
		if isAsk {
			priceValue, err = toLighterPriceTicksCeil(price)
		} else {
			priceValue, err = toLighterPriceTicksFloor(price)
		}
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported order type: %s", orderType)
	}

	txReq := &types.CreateOrderTxReq{
		MarketIndex:      marketIndex,
		ClientOrderIndex: clientOrderIndex,
		BaseAmount:       baseAmount,
		Price:            priceValue,
		IsAsk:            boolToUint8(isAsk),
		Type:             orderTypeValue,
		TimeInForce:      timeInForce,
		ReduceOnly:       boolToUint8(reduceOnly),
		TriggerPrice:     triggerPrice,
		OrderExpiry:      orderExpiry,
	}

	nonce := int64(-1)
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign order: %w", err)
	}

	orderResp, err := t.submitTx(tx, true)
	if err != nil {
		return nil, fmt.Errorf("failed to submit order: %w", err)
	}

	side := "buy"
	if isAsk {
		side = "sell"
	}
	logger.Infof("✓ LIGHTER order created: %s %s qty=%.4f", symbol, side, quantity)

	return orderResp, nil
}

// createTriggerOrder creates a reduce-only stop-loss / take-profit order (IOC when triggered).
func (t *LighterTraderV2) createTriggerOrder(symbol string, isAsk bool, quantity float64, triggerPrice float64, orderType uint8) (map[string]interface{}, error) {
	if t.txClient == nil {
		return nil, fmt.Errorf("TxClient not initialized")
	}
	if quantity < 0 {
		return nil, fmt.Errorf("invalid quantity: %.8f", quantity)
	}
	if triggerPrice <= 0 {
		return nil, fmt.Errorf("invalid trigger price: %.8f", triggerPrice)
	}

	marketIndex, err := t.getSDKMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market index: %w", err)
	}

	clientOrderIndex := t.nextClientOrderIndex()

	var baseAmount int64
	if quantity == 0 {
		baseAmount = txtypes.NilOrderBaseAmount
	} else {
		baseAmount, err = toLighterBaseAmount(quantity)
		if err != nil {
			return nil, err
		}
	}

	triggerTicks, err := toLighterPriceTicks(triggerPrice)
	if err != nil {
		return nil, err
	}

	// Use widest possible bounds; price_protection should guard against extreme fills.
	priceValue := txtypes.MinOrderPrice
	if !isAsk {
		priceValue = txtypes.MaxOrderPrice
	}

	txReq := &types.CreateOrderTxReq{
		MarketIndex:      marketIndex,
		ClientOrderIndex: clientOrderIndex,
		BaseAmount:       baseAmount,
		Price:            priceValue,
		IsAsk:            boolToUint8(isAsk),
		Type:             orderType,
		TimeInForce:      txtypes.ImmediateOrCancel,
		ReduceOnly:       1,
		TriggerPrice:     triggerTicks,
		OrderExpiry:      time.Now().Add(lighterDefaultOrderExpiry).UnixMilli(),
	}

	nonce := int64(-1)
	tx, err := t.txClient.GetCreateOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign trigger order: %w", err)
	}

	return t.submitTx(tx, true)
}

// SendTxRequest Send transaction request
type SendTxRequest struct {
	TxType          int    `json:"tx_type"`
	TxInfo          string `json:"tx_info"`
	PriceProtection bool   `json:"price_protection,omitempty"`
}

// SendTxResponse Send transaction response
type SendTxResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
}

func (t *LighterTraderV2) submitTx(tx txtypes.TxInfo, priceProtection bool) (map[string]interface{}, error) {
	txInfo, err := tx.GetTxInfo()
	if err != nil {
		return nil, fmt.Errorf("failed to get tx_info: %w", err)
	}

	// Build request
	req := SendTxRequest{
		TxType:          int(tx.GetTxType()),
		TxInfo:          txInfo,
		PriceProtection: priceProtection,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/sendTx", t.baseURL)

	send := func(body []byte) ([]byte, int, error) {
		httpReq, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
		if err != nil {
			return nil, 0, err
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if err := t.ensureAuthToken(); err == nil {
			t.accountMutex.RLock()
			if t.authToken != "" {
				httpReq.Header.Set("Authorization", t.authToken)
			}
			t.accountMutex.RUnlock()
		} else {
			logger.Infof("⚠️  Failed to ensure auth token for sendTx: %v", err)
		}

		resp, err := t.client.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, resp.StatusCode, err
		}
		if resp.StatusCode != http.StatusOK {
			return respBody, resp.StatusCode, fmt.Errorf("sendTx http %d: %s", resp.StatusCode, string(respBody))
		}
		return respBody, resp.StatusCode, nil
	}

	parse := func(body []byte) (map[string]interface{}, int, string, error) {
		// New format: { code, message, data }
		var wrapped SendTxResponse
		if err := json.Unmarshal(body, &wrapped); err == nil && (wrapped.Code != 0 || wrapped.Message != "" || wrapped.Data != nil) {
			if wrapped.Code != 200 {
				return nil, wrapped.Code, wrapped.Message, fmt.Errorf("sendTx failed (code %d): %s", wrapped.Code, wrapped.Message)
			}
			return wrapped.Data, wrapped.Code, wrapped.Message, nil
		}

		// Legacy format: { success, message, data }
		var legacy struct {
			Success bool                   `json:"success"`
			Message string                 `json:"message"`
			Data    map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(body, &legacy); err == nil && (legacy.Success || legacy.Data != nil || legacy.Message != "") {
			if !legacy.Success {
				return nil, 0, legacy.Message, fmt.Errorf("sendTx failed: %s", legacy.Message)
			}
			return legacy.Data, 200, legacy.Message, nil
		}

		// Fallback: raw object
		var raw map[string]interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, 0, "", fmt.Errorf("failed to parse sendTx response: %w, body: %s", err, string(body))
		}
		if data, ok := raw["data"].(map[string]interface{}); ok {
			return data, 200, "", nil
		}
		return raw, 200, "", nil
	}

	respBody, _, sendErr := send(reqBody)
	if sendErr != nil {
		return nil, sendErr
	}

	data, _, msg, parseErr := parse(respBody)
	if parseErr != nil {
		// Retry with explicit account/api key indices if the API requires them.
		bodyStr := string(respBody)
		if strings.Contains(msg, "account_index") || strings.Contains(msg, "api_key_index") || strings.Contains(bodyStr, "account_index") || strings.Contains(bodyStr, "api_key_index") {
			t.accountMutex.RLock()
			accountIndex := t.accountIndex
			apiKeyIndex := t.apiKeyIndex
			t.accountMutex.RUnlock()

			type SendTxRequestWithAccount struct {
				TxType          int    `json:"tx_type"`
				TxInfo          string `json:"tx_info"`
				AccountIndex    int64  `json:"account_index"`
				APIKeyIndex     uint8  `json:"api_key_index"`
				PriceProtection bool   `json:"price_protection,omitempty"`
			}

			req2 := SendTxRequestWithAccount{
				TxType:          req.TxType,
				TxInfo:          req.TxInfo,
				AccountIndex:    accountIndex,
				APIKeyIndex:     apiKeyIndex,
				PriceProtection: req.PriceProtection,
			}
			req2Body, err := json.Marshal(req2)
			if err == nil {
				if retryBody, _, retryErr := send(req2Body); retryErr == nil {
					if retryData, _, _, retryParseErr := parse(retryBody); retryParseErr == nil {
						data = retryData
						parseErr = nil
					} else {
						parseErr = retryParseErr
					}
				} else {
					parseErr = retryErr
				}
			}
		}
	}
	if parseErr != nil {
		return nil, parseErr
	}

	// Extract transaction hash and order ID
	result := map[string]interface{}{
		"tx_hash": data["tx_hash"],
		"status":  "submitted",
	}

	// Add order ID to result if available
	if orderID, ok := data["order_id"]; ok {
		result["orderId"] = orderID
	} else if orderID, ok := data["orderId"]; ok {
		result["orderId"] = orderID
	}

	// Fallback to locally signed hash if server doesn't return one.
	if result["tx_hash"] == nil && tx.GetTxHash() != "" {
		result["tx_hash"] = tx.GetTxHash()
	}

	logger.Infof("✓ Tx submitted to LIGHTER - tx_hash: %v", result["tx_hash"])

	return result, nil
}

// getMarketIndex Get market index (convert from symbol) - dynamically fetch from API
func (t *LighterTraderV2) getMarketIndex(symbol string) (int16, error) {
	// 1. Check cache
	t.marketMutex.RLock()
	if index, ok := t.marketIndexMap[symbol]; ok {
		t.marketMutex.RUnlock()
		return index, nil
	}
	t.marketMutex.RUnlock()

	// 2. Fetch market list from API
	markets, err := t.fetchMarketList()
	if err != nil {
		// If API fails, fallback to hardcoded mapping
		logger.Infof("⚠️  Failed to fetch market list from API, using hardcoded mapping: %v", err)
		return t.getFallbackMarketIndex(symbol)
	}

	// 3. Update cache
	t.marketMutex.Lock()
	for _, market := range markets {
		t.marketIndexMap[market.Symbol] = market.MarketID
	}
	t.marketMutex.Unlock()

	// 4. Get from cache
	t.marketMutex.RLock()
	index, ok := t.marketIndexMap[symbol]
	t.marketMutex.RUnlock()

	if !ok {
		return 0, fmt.Errorf("unknown market symbol: %s", symbol)
	}

	return index, nil
}

// MarketInfo Market information
type MarketInfo struct {
	Symbol   string `json:"symbol"`
	MarketID int16  `json:"market_id"`
}

// fetchMarketList Fetch market list from API
func (t *LighterTraderV2) fetchMarketList() ([]MarketInfo, error) {
	endpoint := fmt.Sprintf("%s/api/v1/orderBooks", t.baseURL)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get market list (status %d): %s", resp.StatusCode, string(body))
	}

	type orderBookMarket struct {
		Symbol           string `json:"symbol"`
		MarketIndex      *int16 `json:"market_index"`
		MarketID         *int16 `json:"market_id"`
		MarketIndexCamel *int16 `json:"marketIndex"`
		MarketIDCamel    *int16 `json:"marketId"`
	}

	var rawMarketList json.RawMessage
	var rawMarkets []orderBookMarket
	var apiResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err == nil && apiResp.Data != nil {
		if apiResp.Code != 0 && apiResp.Code != 200 {
			return nil, fmt.Errorf("failed to get market list (code %d): %s", apiResp.Code, apiResp.Message)
		}
		if err := json.Unmarshal(apiResp.Data, &rawMarkets); err != nil {
			// Some deployments wrap again: { data: { data: [...] } }
			var dataObj map[string]json.RawMessage
			if err2 := json.Unmarshal(apiResp.Data, &dataObj); err2 != nil {
				return nil, fmt.Errorf("failed to parse market list: %w", err)
			}
			for _, key := range []string{"data", "order_books", "orderBooks"} {
				raw, ok := dataObj[key]
				if !ok {
					continue
				}
				if err3 := json.Unmarshal(raw, &rawMarkets); err3 != nil {
					return nil, fmt.Errorf("failed to parse market list (%s): %w", key, err3)
				}
				rawMarketList = raw
				break
			}
			if rawMarketList == nil {
				return nil, fmt.Errorf("missing market list in wrapped response: %s", string(body))
			}
		} else {
			rawMarketList = apiResp.Data
		}
	} else {
		// Old format: { data: [...] } or raw array
		if err := json.Unmarshal(body, &rawMarkets); err != nil {
			var dataObj map[string]json.RawMessage
			if err2 := json.Unmarshal(body, &dataObj); err2 != nil {
				return nil, fmt.Errorf("failed to parse market list: %w", err)
			}
			raw, ok := dataObj["data"]
			if !ok {
				return nil, fmt.Errorf("missing data in market list response: %s", string(body))
			}
			if err3 := json.Unmarshal(raw, &rawMarkets); err3 != nil {
				return nil, fmt.Errorf("failed to parse market list data: %w", err3)
			}
			rawMarketList = raw
		} else {
			rawMarketList = body
		}
	}

	// Convert to MarketInfo list
	markets := make([]MarketInfo, len(rawMarkets))
	for i, market := range rawMarkets {
		var marketID *int16
		switch {
		case market.MarketID != nil:
			marketID = market.MarketID
		case market.MarketIndex != nil:
			marketID = market.MarketIndex
		case market.MarketIDCamel != nil:
			marketID = market.MarketIDCamel
		case market.MarketIndexCamel != nil:
			marketID = market.MarketIndexCamel
		default:
			return nil, fmt.Errorf("missing market_id for symbol: %s", market.Symbol)
		}

		markets[i] = MarketInfo{
			Symbol:   market.Symbol,
			MarketID: *marketID,
		}
	}

	t.cacheMarketPrecision(rawMarketList)

	logger.Infof("✓ Retrieved %d markets", len(markets))
	return markets, nil
}

func (t *LighterTraderV2) cacheMarketPrecision(rawMarketList json.RawMessage) {
	if len(rawMarketList) == 0 {
		return
	}

	var marketObjs []map[string]interface{}
	if err := json.Unmarshal(rawMarketList, &marketObjs); err != nil {
		return
	}

	t.precisionMutex.Lock()
	defer t.precisionMutex.Unlock()

	if t.symbolPrecision == nil {
		t.symbolPrecision = make(map[string]SymbolPrecision)
	}

	for _, obj := range marketObjs {
		symbol, ok := obj["symbol"].(string)
		if !ok || strings.TrimSpace(symbol) == "" {
			continue
		}

		pricePrecision, _ := optionalIntFromMap(obj, "price_precision", "pricePrecision")
		quantityPrecision, _ := optionalIntFromMap(obj, "quantity_precision", "quantityPrecision")
		tickSize, _ := optionalFloat64FromMap(obj, "tick_size", "tickSize", "price_tick_size", "priceTickSize")
		stepSize, _ := optionalFloat64FromMap(obj, "step_size", "stepSize", "quantity_step_size", "quantityStepSize")

		prec := SymbolPrecision{
			PricePrecision:    pricePrecision,
			QuantityPrecision: quantityPrecision,
			TickSize:          tickSize,
			StepSize:          stepSize,
		}

		// Only cache when we have at least one usable hint.
		if prec.PricePrecision == 0 && prec.QuantityPrecision == 0 && prec.TickSize <= 0 && prec.StepSize <= 0 {
			continue
		}
		t.symbolPrecision[symbol] = prec
	}
}

func optionalIntFromMap(data map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		if _, ok := data[key]; !ok {
			continue
		}
		v, err := SafeInt(data, key)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

func optionalFloat64FromMap(data map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		if _, ok := data[key]; !ok {
			continue
		}
		v, err := SafeFloat64(data, key)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

// getFallbackMarketIndex Hardcoded fallback mapping
func (t *LighterTraderV2) getFallbackMarketIndex(symbol string) (int16, error) {
	fallbackMap := map[string]int16{
		"BTC-PERP":  0,
		"ETH-PERP":  1,
		"SOL-PERP":  2,
		"DOGE-PERP": 3,
		"AVAX-PERP": 4,
		"XRP-PERP":  5,
	}

	if index, ok := fallbackMap[symbol]; ok {
		logger.Infof("✓ Using hardcoded market index: %s -> %d", symbol, index)
		return index, nil
	}

	return 0, fmt.Errorf("unknown market symbol: %s", symbol)
}

func (t *LighterTraderV2) getSDKMarketIndex(symbol string) (int16, error) {
	marketID, err := t.getMarketIndex(symbol)
	if err != nil {
		return 0, err
	}
	if marketID < txtypes.MinPerpsMarketIndex || marketID > txtypes.MaxPerpsMarketIndex {
		return 0, fmt.Errorf("market_id %d for %s is not a perps market (spot markets start at %d)", marketID, symbol, txtypes.MinSpotMarketIndex)
	}
	return marketID, nil
}

func (t *LighterTraderV2) setCachedLeverage(symbol string, leverage int) {
	t.settingsMutex.Lock()
	t.leverageCache[symbol] = leverage
	t.settingsMutex.Unlock()
}

func (t *LighterTraderV2) getCachedLeverage(symbol string) (int, bool) {
	t.settingsMutex.RLock()
	leverage, ok := t.leverageCache[symbol]
	t.settingsMutex.RUnlock()
	return leverage, ok
}

func (t *LighterTraderV2) setCachedMarginMode(symbol string, marginMode uint8) {
	t.settingsMutex.Lock()
	t.marginModeCache[symbol] = marginMode
	t.settingsMutex.Unlock()
}

func (t *LighterTraderV2) getCachedMarginMode(symbol string) uint8 {
	t.settingsMutex.RLock()
	marginMode, ok := t.marginModeCache[symbol]
	t.settingsMutex.RUnlock()
	if ok {
		return marginMode
	}
	return txtypes.CrossMargin
}

// SetLeverage Set leverage (implements Trader interface)
func (t *LighterTraderV2) SetLeverage(symbol string, leverage int) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}
	if leverage <= 0 {
		return fmt.Errorf("invalid leverage: %d", leverage)
	}

	logger.Infof("⚙️  Setting leverage: %s = %dx", symbol, leverage)

	marketIndex, err := t.getSDKMarketIndex(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market index: %w", err)
	}

	initialMarginFraction, err := leverageToInitialMarginFraction(leverage)
	if err != nil {
		return err
	}

	txReq := &types.UpdateLeverageTxReq{
		MarketIndex:           marketIndex,
		InitialMarginFraction: initialMarginFraction,
		MarginMode:            t.getCachedMarginMode(symbol),
	}

	nonce := int64(-1)
	tx, err := t.txClient.GetUpdateLeverageTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return fmt.Errorf("failed to sign update leverage tx: %w", err)
	}

	if _, err := t.submitTx(tx, false); err != nil {
		return fmt.Errorf("failed to submit update leverage tx: %w", err)
	}

	t.setCachedLeverage(symbol, leverage)
	return nil
}

// SetMarginMode Set margin mode (implements Trader interface)
func (t *LighterTraderV2) SetMarginMode(symbol string, isCrossMargin bool) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	modeStr := "isolated"
	if isCrossMargin {
		modeStr = "cross"
	}

	logger.Infof("⚙️  Setting margin mode: %s = %s", symbol, modeStr)

	var marginMode uint8 = txtypes.IsolatedMargin
	if isCrossMargin {
		marginMode = txtypes.CrossMargin
	}
	t.setCachedMarginMode(symbol, marginMode)

	// If we know the leverage for this symbol, apply immediately by submitting an UpdateLeverage tx.
	if lev, ok := t.getCachedLeverage(symbol); ok && lev > 0 {
		return t.SetLeverage(symbol, lev)
	}

	// Try to infer leverage from current position to apply immediately.
	pos, err := t.GetPosition(symbol)
	if err != nil {
		logger.Infof("⚠️  Failed to fetch position for margin mode update: %v", err)
		return nil
	}
	if pos != nil && pos.Leverage >= 1 {
		return t.SetLeverage(symbol, int(math.Round(pos.Leverage)))
	}

	// Otherwise, keep the desired margin mode cached and apply on the next SetLeverage().
	logger.Infof("⚠️  Margin mode cached; will apply on next SetLeverage()")
	return nil
}

// boolToUint8 Convert boolean to uint8
func boolToUint8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func (t *LighterTraderV2) nextClientOrderIndex() int64 {
	now := time.Now().UnixMilli()

	t.clientOrderMutex.Lock()
	if now <= t.lastClientOrderIndex {
		now = t.lastClientOrderIndex + 1
	}
	t.lastClientOrderIndex = now
	t.clientOrderMutex.Unlock()

	return now
}

func toLighterBaseAmount(quantity float64) (int64, error) {
	if quantity <= 0 {
		return 0, fmt.Errorf("invalid quantity: %.8f", quantity)
	}
	baseAmount := int64(math.Round(quantity * lighterBaseAmountScale))
	if baseAmount < txtypes.MinOrderBaseAmount {
		return 0, fmt.Errorf("quantity too small after rounding: %.8f", quantity)
	}
	if baseAmount > txtypes.MaxOrderBaseAmount {
		return 0, fmt.Errorf("quantity too large: %.8f", quantity)
	}
	return baseAmount, nil
}

func toLighterPriceTicksCeil(price float64) (uint32, error) {
	return toLighterPriceTicksWithRounding(price, math.Ceil)
}

func toLighterPriceTicks(price float64) (uint32, error) {
	return toLighterPriceTicksWithRounding(price, math.Round)
}

func toLighterPriceTicksFloor(price float64) (uint32, error) {
	return toLighterPriceTicksWithRounding(price, math.Floor)
}

func toLighterPriceTicksWithRounding(price float64, round func(float64) float64) (uint32, error) {
	if price <= 0 {
		return 0, fmt.Errorf("invalid price: %.8f", price)
	}

	ticks := int64(round(price * lighterPriceScale))
	if ticks < int64(txtypes.MinOrderPrice) {
		ticks = int64(txtypes.MinOrderPrice)
	}
	if ticks > int64(txtypes.MaxOrderPrice) {
		return 0, fmt.Errorf("price out of range: %.8f", price)
	}
	return uint32(ticks), nil
}

func leverageToInitialMarginFraction(leverage int) (uint16, error) {
	if leverage <= 0 {
		return 0, fmt.Errorf("invalid leverage: %d", leverage)
	}
	fraction := float64(txtypes.MarginFractionTick) / float64(leverage)
	if fraction < 1 {
		fraction = 1
	}
	if fraction > float64(txtypes.MarginFractionTick) {
		fraction = float64(txtypes.MarginFractionTick)
	}
	return uint16(math.Round(fraction)), nil
}
