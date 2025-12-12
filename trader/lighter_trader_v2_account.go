package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
)

// GetBalance Get account balance (implements Trader interface)
func (t *LighterTraderV2) GetBalance() (map[string]interface{}, error) {
	balance, err := t.GetAccountBalance()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total_equity":       balance.TotalEquity,
		"available_balance":  balance.AvailableBalance,
		"margin_used":        balance.MarginUsed,
		"unrealized_pnl":     balance.UnrealizedPnL,
		"maintenance_margin": balance.MaintenanceMargin,
	}, nil
}

// GetAccountBalance Get detailed account balance information
func (t *LighterTraderV2) GetAccountBalance() (*AccountBalance, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	authToken := t.authToken
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/balance", t.baseURL, accountIndex)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication header
	req.Header.Set("Authorization", authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get balance (status %d): %s", resp.StatusCode, string(body))
	}

	var balance AccountBalance
	if err := json.Unmarshal(body, &balance); err != nil {
		return nil, fmt.Errorf("failed to parse balance response: %w", err)
	}

	return &balance, nil
}

// GetPositions Get all positions (implements Trader interface)
func (t *LighterTraderV2) GetPositions() ([]map[string]interface{}, error) {
	positions, err := t.GetPositionsRaw("")
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(positions))
	for _, pos := range positions {
		result = append(result, map[string]interface{}{
			"symbol":            pos.Symbol,
			"side":              pos.Side,
			"size":              pos.Size,
			"entry_price":       pos.EntryPrice,
			"mark_price":        pos.MarkPrice,
			"liquidation_price": pos.LiquidationPrice,
			"unrealized_pnl":    pos.UnrealizedPnL,
			"leverage":          pos.Leverage,
			"margin_used":       pos.MarginUsed,
		})
	}

	return result, nil
}

// GetPositionsRaw Get all positions (returns raw type)
func (t *LighterTraderV2) GetPositionsRaw(symbol string) ([]Position, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	t.accountMutex.RLock()
	accountIndex := t.accountIndex
	authToken := t.authToken
	t.accountMutex.RUnlock()

	endpoint := fmt.Sprintf("%s/api/v1/account/%d/positions", t.baseURL, accountIndex)
	if symbol != "" {
		endpoint += fmt.Sprintf("?symbol=%s", symbol)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", authToken)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get positions (status %d): %s", resp.StatusCode, string(body))
	}

	var positions []Position
	if err := json.Unmarshal(body, &positions); err != nil {
		return nil, fmt.Errorf("failed to parse positions response: %w", err)
	}

	return positions, nil
}

// GetPosition Get position for specified symbol
func (t *LighterTraderV2) GetPosition(symbol string) (*Position, error) {
	positions, err := t.GetPositionsRaw(symbol)
	if err != nil {
		return nil, err
	}

	for _, pos := range positions {
		if pos.Symbol == symbol && pos.Size > 0 {
			return &pos, nil
		}
	}

	return nil, nil // No position
}

// GetMarketPrice Get market price (implements Trader interface)
func (t *LighterTraderV2) GetMarketPrice(symbol string) (float64, error) {
	endpoint := fmt.Sprintf("%s/api/v1/market/ticker?symbol=%s", t.baseURL, symbol)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return 0, err
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to get market price (status %d): %s", resp.StatusCode, string(body))
	}

	var ticker map[string]interface{}
	if err := json.Unmarshal(body, &ticker); err != nil {
		return 0, fmt.Errorf("failed to parse price response: %w", err)
	}

	price, err := SafeFloat64(ticker, "last_price")
	if err != nil {
		return 0, fmt.Errorf("failed to get price: %w", err)
	}

	return price, nil
}

func (t *LighterTraderV2) getPrecision(symbol string) (SymbolPrecision, error) {
	t.precisionMutex.RLock()
	if prec, ok := t.symbolPrecision[symbol]; ok {
		t.precisionMutex.RUnlock()
		return prec, nil
	}
	t.precisionMutex.RUnlock()

	// Best-effort: reuse the market list endpoint (also fills precision cache when available).
	if t.client != nil && t.baseURL != "" {
		_, _ = t.fetchMarketList()
	}

	t.precisionMutex.RLock()
	if prec, ok := t.symbolPrecision[symbol]; ok {
		t.precisionMutex.RUnlock()
		return prec, nil
	}
	t.precisionMutex.RUnlock()

	// Fallback: Lighter base amount ticks are treated as 1e-8.
	return SymbolPrecision{
		QuantityPrecision: 8,
		StepSize:          1.0 / lighterBaseAmountScale,
	}, nil
}

// FormatQuantity Format quantity to correct precision (implements Trader interface)
func (t *LighterTraderV2) FormatQuantity(symbol string, quantity float64) (string, error) {
	if quantity == 0 {
		return "0", nil
	}
	if quantity < 0 {
		return "", fmt.Errorf("invalid quantity: %.8f", quantity)
	}

	prec, err := t.getPrecision(symbol)
	if err != nil {
		return "", err
	}

	// Prefer step size when available; otherwise fall back to precision.
	if prec.StepSize > 0 {
		quantity = roundToTickSize(quantity, prec.StepSize)
	} else if prec.QuantityPrecision > 0 {
		multiplier := math.Pow10(prec.QuantityPrecision)
		quantity = math.Round(quantity*multiplier) / multiplier
	}

	baseAmount, err := toLighterBaseAmount(quantity)
	if err != nil {
		return "", err
	}

	formatted := fmt.Sprintf("%.8f", float64(baseAmount)/lighterBaseAmountScale)
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")
	if formatted == "" {
		formatted = "0"
	}
	return formatted, nil
}
