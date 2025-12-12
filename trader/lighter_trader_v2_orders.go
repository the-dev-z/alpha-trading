package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/logger"
	"strconv"

	"github.com/elliottech/lighter-go/types"
	"github.com/elliottech/lighter-go/types/txtypes"
)

type LighterActiveOrder struct {
	OrderID      string
	OrderType    int
	TriggerPrice float64
	Raw          map[string]interface{}
}

func getFirstString(m map[string]interface{}, keys ...string) (string, bool) {
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			continue
		}
		v, err := SafeString(m, key)
		if err == nil && v != "" {
			return v, true
		}
	}
	return "", false
}

func getFirstFloat64(m map[string]interface{}, keys ...string) (float64, bool) {
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			continue
		}
		v, err := SafeFloat64(m, key)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

func getFirstInt(m map[string]interface{}, keys ...string) (int, bool) {
	for _, key := range keys {
		if _, ok := m[key]; !ok {
			continue
		}
		v, err := SafeInt(m, key)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

// SetStopLoss Set stop-loss order (implements Trader interface)
func (t *LighterTraderV2) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	logger.Infof("🛑 LIGHTER Setting stop-loss: %s %s qty=%.4f, stop=%.2f", symbol, positionSide, quantity, stopPrice)

	// Determine order direction (short position uses buy order, long position uses sell order)
	isAsk := (positionSide == "LONG" || positionSide == "long")

	_, err := t.createTriggerOrder(symbol, isAsk, quantity, stopPrice, txtypes.StopLossOrder)
	if err != nil {
		return fmt.Errorf("failed to set stop-loss: %w", err)
	}

	logger.Infof("✓ LIGHTER stop-loss set: %.2f", stopPrice)
	return nil
}

// SetTakeProfit Set take-profit order (implements Trader interface)
func (t *LighterTraderV2) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	logger.Infof("🎯 LIGHTER Setting take-profit: %s %s qty=%.4f, tp=%.2f", symbol, positionSide, quantity, takeProfitPrice)

	// Determine order direction (short position uses buy order, long position uses sell order)
	isAsk := (positionSide == "LONG" || positionSide == "long")

	_, err := t.createTriggerOrder(symbol, isAsk, quantity, takeProfitPrice, txtypes.TakeProfitOrder)
	if err != nil {
		return fmt.Errorf("failed to set take-profit: %w", err)
	}

	logger.Infof("✓ LIGHTER take-profit set: %.2f", takeProfitPrice)
	return nil
}

// CancelAllOrders Cancel all orders (implements Trader interface)
func (t *LighterTraderV2) CancelAllOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("invalid auth token: %w", err)
	}

	// Get all active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	if len(orders) == 0 {
		logger.Infof("✓ LIGHTER - No orders to cancel (no active orders)")
		return nil
	}

	// Batch cancel
	canceledCount := 0
	for _, order := range orders {
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️  Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	logger.Infof("✓ LIGHTER - Canceled %d orders", canceledCount)
	return nil
}

// GetOrderStatus Get order status (implements Trader interface)
func (t *LighterTraderV2) GetOrderStatus(symbol string, orderID string) (map[string]interface{}, error) {
	// LIGHTER market orders are usually filled immediately
	// Try to query order status
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	// Build request URL
	endpoint := fmt.Sprintf("%s/api/v1/order/%s", t.baseURL, orderID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", t.authToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// If query fails, assume order is filled
		return map[string]interface{}{
			"orderId":     orderID,
			"status":      "FILLED",
			"avgPrice":    0.0,
			"executedQty": 0.0,
			"commission":  0.0,
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"orderId":     orderID,
			"status":      "FILLED",
			"avgPrice":    0.0,
			"executedQty": 0.0,
			"commission":  0.0,
		}, nil
	}

	var order OrderResponse
	if err := json.Unmarshal(body, &order); err != nil {
		return map[string]interface{}{
			"orderId":     orderID,
			"status":      "FILLED",
			"avgPrice":    0.0,
			"executedQty": 0.0,
			"commission":  0.0,
		}, nil
	}

	// Convert status to unified format
	unifiedStatus := order.Status
	switch order.Status {
	case "filled":
		unifiedStatus = "FILLED"
	case "open":
		unifiedStatus = "NEW"
	case "cancelled":
		unifiedStatus = "CANCELED"
	}

	return map[string]interface{}{
		"orderId":     order.OrderID,
		"status":      unifiedStatus,
		"avgPrice":    order.Price,
		"executedQty": order.FilledQty,
		"commission":  0.0,
	}, nil
}

// CancelStopLossOrders Cancel only stop-loss orders (implements Trader interface)
func (t *LighterTraderV2) CancelStopLossOrders(symbol string) error {
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return err
	}

	hasType := false
	for _, order := range orders {
		if order.OrderType != -1 {
			hasType = true
			break
		}
	}
	if !hasType {
		logger.Infof("⚠️  LIGHTER cannot distinguish stop-loss/take-profit orders (missing order type), will cancel all stop orders")
		return t.CancelStopOrders(symbol)
	}

	canceledCount := 0
	for _, order := range orders {
		if order.OrderType != int(txtypes.StopLossOrder) && order.OrderType != int(txtypes.StopLossLimitOrder) {
			continue
		}
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️  Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	logger.Infof("✓ LIGHTER - Canceled %d stop-loss orders", canceledCount)
	return nil
}

// CancelTakeProfitOrders Cancel only take-profit orders (implements Trader interface)
func (t *LighterTraderV2) CancelTakeProfitOrders(symbol string) error {
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return err
	}

	hasType := false
	for _, order := range orders {
		if order.OrderType != -1 {
			hasType = true
			break
		}
	}
	if !hasType {
		logger.Infof("⚠️  LIGHTER cannot distinguish stop-loss/take-profit orders (missing order type), will cancel all stop orders")
		return t.CancelStopOrders(symbol)
	}

	canceledCount := 0
	for _, order := range orders {
		if order.OrderType != int(txtypes.TakeProfitOrder) && order.OrderType != int(txtypes.TakeProfitLimitOrder) {
			continue
		}
		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️  Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	logger.Infof("✓ LIGHTER - Canceled %d take-profit orders", canceledCount)
	return nil
}

// CancelStopOrders Cancel stop-loss/take-profit orders for this symbol (implements Trader interface)
func (t *LighterTraderV2) CancelStopOrders(symbol string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	if err := t.ensureAuthToken(); err != nil {
		return fmt.Errorf("invalid auth token: %w", err)
	}

	// Get active orders
	orders, err := t.GetActiveOrders(symbol)
	if err != nil {
		return fmt.Errorf("failed to get active orders: %w", err)
	}

	canceledCount := 0
	for _, order := range orders {
		isStop := order.TriggerPrice > 0
		if order.OrderType != -1 {
			switch order.OrderType {
			case int(txtypes.StopLossOrder), int(txtypes.StopLossLimitOrder), int(txtypes.TakeProfitOrder), int(txtypes.TakeProfitLimitOrder):
				isStop = true
			default:
				isStop = false
			}
		}
		if !isStop {
			continue
		}

		if err := t.CancelOrder(symbol, order.OrderID); err != nil {
			logger.Infof("⚠️  Failed to cancel order (ID: %s): %v", order.OrderID, err)
		} else {
			canceledCount++
		}
	}

	logger.Infof("✓ LIGHTER - Canceled %d stop orders", canceledCount)
	return nil
}

// GetActiveOrders Get active orders
func (t *LighterTraderV2) GetActiveOrders(symbol string) ([]LighterActiveOrder, error) {
	if err := t.ensureAuthToken(); err != nil {
		return nil, fmt.Errorf("invalid auth token: %w", err)
	}

	// Get market index
	marketIndex, err := t.getMarketIndex(symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to get market index: %w", err)
	}

	// Build request URL
	endpoint := fmt.Sprintf("%s/api/v1/accountActiveOrders?account_index=%d&market_id=%d",
		t.baseURL, t.accountIndex, marketIndex)

	// Send GET request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", t.authToken)
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

	// Parse response
	var apiResp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	if apiResp.Code != 200 {
		return nil, fmt.Errorf("failed to get active orders (code %d): %s", apiResp.Code, apiResp.Message)
	}

	var rawOrders []map[string]interface{}
	if err := json.Unmarshal(apiResp.Data, &rawOrders); err != nil {
		var dataObj map[string]json.RawMessage
		if err2 := json.Unmarshal(apiResp.Data, &dataObj); err2 != nil {
			return nil, fmt.Errorf("failed to parse active orders: %w", err)
		}
		for _, key := range []string{"orders", "data"} {
			raw, ok := dataObj[key]
			if !ok {
				continue
			}
			if err3 := json.Unmarshal(raw, &rawOrders); err3 != nil {
				return nil, fmt.Errorf("failed to parse active orders (%s): %w", key, err3)
			}
			break
		}
	}

	orders := make([]LighterActiveOrder, 0, len(rawOrders))
	for _, o := range rawOrders {
		orderID, ok := getFirstString(o, "order_id", "orderId", "id", "index")
		if !ok || orderID == "" {
			continue
		}

		orderType := -1
		if v, ok := getFirstInt(o, "type", "order_type", "orderType"); ok {
			orderType = v
		}

		triggerPrice := 0.0
		if v, ok := getFirstFloat64(o, "trigger_price", "triggerPrice"); ok {
			triggerPrice = v
		}

		orders = append(orders, LighterActiveOrder{
			OrderID:      orderID,
			OrderType:    orderType,
			TriggerPrice: triggerPrice,
			Raw:          o,
		})
	}

	logger.Infof("✓ LIGHTER - Retrieved %d active orders", len(orders))
	return orders, nil
}

// CancelOrder Cancel a single order
func (t *LighterTraderV2) CancelOrder(symbol, orderID string) error {
	if t.txClient == nil {
		return fmt.Errorf("TxClient not initialized")
	}

	// Get market index
	marketIndex, err := t.getSDKMarketIndex(symbol)
	if err != nil {
		return fmt.Errorf("failed to get market index: %w", err)
	}

	// Convert orderID to int64
	orderIndex, err := strconv.ParseInt(orderID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid order ID: %w", err)
	}

	// Build cancel order request
	txReq := &types.CancelOrderTxReq{
		MarketIndex: marketIndex,
		Index:       orderIndex,
	}

	// Sign transaction using SDK
	nonce := int64(-1) // -1 means auto-fetch
	tx, err := t.txClient.GetCancelOrderTransaction(txReq, &types.TransactOpts{
		Nonce: &nonce,
	})
	if err != nil {
		return fmt.Errorf("failed to sign cancel order: %w", err)
	}

	// Submit cancel order to LIGHTER API
	_, err = t.submitTx(tx, false)
	if err != nil {
		return fmt.Errorf("failed to submit cancel order: %w", err)
	}

	logger.Infof("✓ LIGHTER order canceled - ID: %s", orderID)
	return nil
}
