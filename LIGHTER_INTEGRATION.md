# LIGHTER DEX 集成完成文檔

## ✅ 已完成功能

### 1. 核心架構
- ✅ 集成官方 `lighter-go` SDK (v1.0.1)
- ✅ 集成 Poseidon2 Goldilocks 簽名庫（純 Go，無需 CGO）
- ✅ 實現雙密鑰系統（L1錢包 + API Key）
- ✅ V2（SDK）支持完整交易流程（下單/撤單/槓桿/止盈止損）

### 2. 實現的 Trader 接口方法（17個）

#### 賬戶查詢
- ✅ `GetBalance()` - 獲取賬戶余額
- ✅ `GetPositions()` - 獲取所有持倉
- ✅ `GetMarketPrice(symbol)` - 獲取市場價格

#### 交易操作
- ✅ `OpenLong(symbol, quantity, leverage)` - 開多倉
- ✅ `OpenShort(symbol, quantity, leverage)` - 開空倉
- ✅ `CloseLong(symbol, quantity)` - 平多倉
- ✅ `CloseShort(symbol, quantity)` - 平空倉

#### 止盈止損
- ✅ `SetStopLoss(symbol, side, quantity, price)` - 設置止損
- ✅ `SetTakeProfit(symbol, side, quantity, price)` - 設置止盈
- ✅ `CancelStopLossOrders(symbol)` - 取消止損單
- ✅ `CancelTakeProfitOrders(symbol)` - 取消止盈單
- ✅ `CancelStopOrders(symbol)` - 取消止盈止損單

#### 訂單管理
- ✅ `CancelAllOrders(symbol)` - 取消所有訂單

#### 配置管理
- ✅ `SetLeverage(symbol, leverage)` - 設置杠杆
- ✅ `SetMarginMode(symbol, isCross)` - 設置倉位模式
- ✅ `FormatQuantity(symbol, quantity)` - 格式化數量

#### 系統方法
- ✅ `GetExchangeType()` - 返回 "lighter"
- ✅ `Cleanup()` - 清理資源

### 3. 核心功能

#### 認證與簽名
- ✅ 自動認證令牌管理（有效期約 7–8 小時，提前 30 分鐘刷新）
- ✅ 使用 SDK 簽名所有交易（Poseidon2 + Schnorr）
- ✅ API Key 驗證機制

#### 訂單處理
- ✅ 市價單支持
- ✅ 限價單支持
- ✅ 自動 nonce 管理
- ✅ 訂單狀態追蹤

---

## 🔑 雙密鑰系統說明

LIGHTER 使用雙密鑰架構：

### L1 私鑰（32字節，標準以太坊私鑰）
- **用途**：識別賬戶、註冊 API Key
- **格式**：標準 ECDSA 私鑰（0x...）
- **存儲**：`lighter_private_key` 數據庫字段

### API Key 私鑰（40字節）
- **用途**：簽名所有交易（使用 Poseidon2 + Schnorr）
- **格式**：40字節十六進制字符串
- **生成**：通過 LIGHTER 官網或 SDK
- **存儲**：`lighter_api_key_private_key` 數據庫字段（新增）

---

## 📋 使用步驟

### 步驟 1：獲取 L1 私鑰
這是你的標準以太坊錢包私鑰：
```
0x1234567890abcdef...（64字符）
```

### 步驟 2：獲取 API Key
有兩種方式：

#### 方式 A：通過 LIGHTER 官網
1. 訪問 https://mainnet.zklighter.elliot.ai (或 testnet)
2. 連接錢包
3. 生成 API Key
4. 保存 API Key 私鑰（40字節）

#### 方式 B：使用 SDK（已實現）
```go
// 生成新的 API Key
privateKey, publicKey, err := trader.GenerateAndRegisterAPIKey(seed)
```

### 步驟 3：配置到 NOFX
在交易所配置頁面添加：
- **Exchange**: LIGHTER
- **L1 Wallet Address**: 0x...
- **L1 Private Key**: 0x...（32字節）
- **API Key Private Key**: 0x...（40字節）⭐**新增**
- **Testnet**: true/false

### 步驟 4：啟動 Trader
交易所配置中必須提供 `lighter_api_key_private_key`（40 字節），否則 LIGHTER 將無法初始化交易能力（V1 已停用）。

---

## 🏗️ 架構設計

### 文件結構
```
trader/
├── lighter_trader.go              # V1 基本實現（舊版）
├── lighter_account.go             # V1 賬戶查詢
├── lighter_orders.go              # V1 訂單管理
├── lighter_trading.go             # V1 交易操作
│
├── lighter_trader_v2.go           # ⭐V2 核心（使用 SDK）
├── lighter_trader_v2_account.go   # ⭐V2 賬戶查詢
├── lighter_trader_v2_trading.go   # ⭐V2 交易操作
├── lighter_trader_v2_orders.go    # ⭐V2 訂單管理
└── interface.go                   # Trader 接口定義
```

### V1 vs V2 對比

| 功能 | V1 (基本實現) | V2 (SDK集成) |
|------|-------------|-------------|
| 認證令牌 | ❌ 不支持（已停用） | ✅ 支持 |
| 訂單簽名 | ❌ 不支持（已停用） | ✅ Poseidon2 + Schnorr |
| 開/平倉交易 | ❌ 不支持（已停用） | ✅ 支持 |
| 止盈止損 | ❌ 不支持（已停用） | ✅ 支持（Trigger Orders） |
| 槓桿/保證金 | ❌ 不支持（已停用） | ✅ 支持 |
| 依賴 | - | 純 Go（無需 CGO） |

---

## 🚀 當前狀態

### ✅ 已完成功能

#### 後端實現
1. ✅ **交易功能（V2）**
   - 下單/撤單（`sendTx`）
   - 止盈止損（Trigger Orders）
   - 槓桿與保證金模式（UpdateLeverage）

2. ✅ **認證**
   - 生成/刷新 Auth Token（用於查詢餘額、持倉、掛單等需要授權的 API）

3. ✅ **市場映射**
   - 動態 `orderBooks` 拉取 market_id / market_index（含欄位名稱兼容）

### ⏳ 待完成功能

#### 待確認（需要對照官方 API/實盤）
- `sendTx` 是否要求額外的 `account_index` / `api_key_index` 欄位（目前已有自動重試支援）
- 交易數量/價格的縮放常數是否需要按 market 做精度化（目前以固定 ticks 處理）
- Spot 市場（market_id >= 2048）目前未在 Trader 交易接口中支持（Perps-only）

### 測試
```bash
# 僅跑離線單元測試（不需要網路/不需要 httptest 綁定 port）
go test ./... -run TestLighterTraderV2_ -count=1
```

---

## 📝 配置示例

### 環境變量
```bash
# LIGHTER Mainnet
LIGHTER_L1_PRIVATE_KEY="0x..."
LIGHTER_API_KEY_PRIVATE_KEY="0x..."
LIGHTER_WALLET_ADDR="0x..."

# LIGHTER Testnet
LIGHTER_TESTNET=true
```

### 數據庫配置
```sql
-- 添加新列（遷移）
ALTER TABLE exchanges
ADD COLUMN lighter_api_key_private_key TEXT DEFAULT '';
```

---

## 🐛 已知問題與限制

1. **Spot 市場**
   - Spot market_id 從 2048 開始；目前交易流程以 Perps 為主，Spot 會被拒絕。

2. **離線環境測試限制**
   - 部分 trader 測試使用 `httptest.NewServer`，在 sandbox 環境可能因為無法綁定本地 port 而失敗；可用 `-run TestLighterTraderV2_` 跑 Lighter 的離線測試。

---

## 📚 參考資料

- [LIGHTER 官方文檔](https://apidocs.lighter.xyz/)
- [lighter-go SDK](https://github.com/elliottech/lighter-go)
- [lighter-python SDK](https://github.com/elliottech/lighter-python)
- [Poseidon2 論文](https://eprint.iacr.org/2023/323)

---

## 🎯 總結

✅ **完成度**: 後端可編譯 + 離線測試可跑（Lighter V2）

✅ **下一步**: 建議先在 Testnet 用小倉位做一次開倉/平倉/止損止盈/撤單的 smoke test（可參考 `scripts/test_lighter.sh`）。

---

**創建時間**: 2025-01-20
**最後更新**: 2025-01-20
**作者**: Claude (Anthropic)
**版本**: 1.0.0
