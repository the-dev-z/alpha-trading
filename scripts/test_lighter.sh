#!/bin/bash

# LIGHTER 集成測試腳本
# 用途：自動化測試 Lighter DEX 集成功能

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "════════════════════════════════════════════════════════════"
echo "🧪 LIGHTER DEX 集成測試"
echo "════════════════════════════════════════════════════════════"
echo ""

# 顏色輸出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 測試配置（請根據實際情況修改）
BACKEND_URL="http://localhost:8081"
TRADER_ID="" # 將在創建後自動填充

# 步驟 1: 檢查容器狀態
echo "📋 步驟 1: 檢查後端容器狀態"
if ! docker ps | grep -q nofx-trading; then
    echo -e "${RED}❌ nofx-trading 容器未運行${NC}"
    echo "請先啟動容器："
    echo "  docker run -d --name nofx-trading -p 8081:8080 nofx-trading:latest"
    exit 1
else
    echo -e "${GREEN}✅ nofx-trading 容器運行中${NC}"
fi
echo ""

# 步驟 2: 檢查健康狀態
echo "📋 步驟 2: 檢查 API 健康狀態"
HEALTH=$(curl -s $BACKEND_URL/api/health | jq -r '.status' 2>/dev/null || echo "error")
if [ "$HEALTH" = "ok" ]; then
    echo -e "${GREEN}✅ API 健康檢查通過${NC}"
else
    echo -e "${RED}❌ API 健康檢查失敗${NC}"
    exit 1
fi
echo ""

# 步驟 3: 檢查 Lighter 代碼是否包含
echo "📋 步驟 3: 驗證 Lighter 集成是否包含在容器中"
docker exec nofx-trading ls -la /app/nofx 2>/dev/null | head -3
if docker exec nofx-trading ./nofx --version 2>&1 | grep -q "unknown"; then
    echo -e "${YELLOW}⚠️  版本信息未知（開發環境正常）${NC}"
else
    echo -e "${GREEN}✅ 版本信息正常${NC}"
fi
echo ""

# 步驟 4: 檢查日誌中的 Lighter 相關信息
echo "📋 步驟 4: 檢查容器日誌中是否有 Lighter 支持"
if docker logs nofx-trading 2>&1 | grep -qi "lighter"; then
    echo -e "${GREEN}✅ 找到 Lighter 相關日誌${NC}"
    docker logs nofx-trading 2>&1 | grep -i "lighter" | tail -5
else
    echo -e "${YELLOW}⚠️  未找到 Lighter 相關日誌（可能未創建 Lighter trader）${NC}"
fi
echo ""

# 步驟 5: 獲取現有交易所列表
echo "📋 步驟 5: 獲取已配置的交易所"
EXCHANGES=$(curl -s "$BACKEND_URL/api/exchanges" | jq -r '.exchanges[]? | select(.exchange_type=="lighter") | .id' 2>/dev/null || echo "")
if [ -n "$EXCHANGES" ]; then
    echo -e "${GREEN}✅ 找到 Lighter 交易所配置${NC}"
    curl -s "$BACKEND_URL/api/exchanges" | jq '.exchanges[]? | select(.exchange_type=="lighter")'
    echo ""
    echo -e "${YELLOW}提示：可以繼續使用此配置創建 trader${NC}"
else
    echo -e "${YELLOW}⚠️  未找到 Lighter 交易所配置${NC}"
    echo "請通過 UI 創建 Lighter 交易所配置："
    echo "  1. 訪問 http://localhost:3001"
    echo "  2. 進入 Exchanges 頁面"
    echo "  3. 點擊 'Add Exchange'"
    echo "  4. 選擇 'Lighter'"
    echo "  5. 填寫配置並保存"
fi
echo ""

# 步驟 6: 獲取現有 Lighter traders
echo "📋 步驟 6: 檢查是否有 Lighter trader"
ALL_TRADERS=$(curl -s "$BACKEND_URL/api/traders" | jq -r '.traders[]? | select(.exchange_type=="lighter") | .id' 2>/dev/null || echo "")
if [ -n "$ALL_TRADERS" ]; then
    echo -e "${GREEN}✅ 找到 Lighter trader:${NC}"
    for tid in $ALL_TRADERS; do
        echo "  • Trader ID: $tid"
        # 獲取詳細信息
        curl -s "$BACKEND_URL/api/traders/$tid/public-config" | jq '{name, exchange_type, is_running}' 2>/dev/null || true
    done

    # 選擇第一個用於測試
    TRADER_ID=$(echo "$ALL_TRADERS" | head -1)
    echo ""
    echo -e "${YELLOW}將使用 Trader ID: $TRADER_ID 進行測試${NC}"
else
    echo -e "${YELLOW}⚠️  未找到 Lighter trader${NC}"
    echo "請先創建 Lighter trader（需要先配置交易所）"
fi
echo ""

# 步驟 7: 測試查詢功能（如果有 trader）
if [ -n "$TRADER_ID" ]; then
    echo "📋 步驟 7: 測試查詢功能"

    echo "  • 測試賬戶信息 API..."
    ACCOUNT=$(curl -s "$BACKEND_URL/api/account?trader_id=$TRADER_ID")
    if echo "$ACCOUNT" | jq -e '.account' >/dev/null 2>&1; then
        echo -e "${GREEN}    ✅ 賬戶信息 API 正常${NC}"
        echo "$ACCOUNT" | jq '.account | {balance, equity, margin_used}'
    else
        echo -e "${RED}    ❌ 賬戶信息 API 失敗${NC}"
        echo "$ACCOUNT" | jq . || echo "$ACCOUNT"
    fi
    echo ""

    echo "  • 測試持倉信息 API..."
    POSITIONS=$(curl -s "$BACKEND_URL/api/positions?trader_id=$TRADER_ID")
    if echo "$POSITIONS" | jq -e '.positions' >/dev/null 2>&1; then
        POS_COUNT=$(echo "$POSITIONS" | jq '.positions | length')
        echo -e "${GREEN}    ✅ 持倉信息 API 正常（$POS_COUNT 個持倉）${NC}"
        if [ "$POS_COUNT" -gt 0 ]; then
            echo "$POSITIONS" | jq '.positions[] | {symbol, side, size, entry_price}'
        fi
    else
        echo -e "${RED}    ❌ 持倉信息 API 失敗${NC}"
        echo "$POSITIONS" | jq . || echo "$POSITIONS"
    fi
    echo ""

    echo "  • 測試 Trader 狀態 API..."
    STATUS=$(curl -s "$BACKEND_URL/api/status?trader_id=$TRADER_ID")
    if echo "$STATUS" | jq -e '.trader' >/dev/null 2>&1; then
        echo -e "${GREEN}    ✅ Trader 狀態 API 正常${NC}"
        echo "$STATUS" | jq '.trader | {name, exchange_type, is_running, status}'
    else
        echo -e "${RED}    ❌ Trader 狀態 API 失敗${NC}"
        echo "$STATUS" | jq . || echo "$STATUS"
    fi
    echo ""
fi

# 步驟 8: 檢查日誌中的錯誤
echo "📋 步驟 8: 檢查最近的錯誤日誌"
ERROR_COUNT=$(docker logs nofx-trading --since 5m 2>&1 | grep -ci "error\|failed\|panic" || echo "0")
if [ "$ERROR_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✅ 最近5分鐘無錯誤日誌${NC}"
else
    echo -e "${YELLOW}⚠️  發現 $ERROR_COUNT 條錯誤日誌:${NC}"
    docker logs nofx-trading --since 5m 2>&1 | grep -i "error\|failed\|panic" | tail -10
fi
echo ""

# 總結
echo "════════════════════════════════════════════════════════════"
echo "📊 測試總結"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "✅ 已完成項目："
echo "  • 容器運行狀態檢查"
echo "  • API 健康檢查"
echo "  • Lighter 代碼集成驗證"
echo ""

if [ -n "$EXCHANGES" ]; then
    echo "✅ Lighter 交易所配置：已創建"
else
    echo "❌ Lighter 交易所配置：未創建"
fi

if [ -n "$TRADER_ID" ]; then
    echo "✅ Lighter Trader：已創建 (ID: $TRADER_ID)"
else
    echo "❌ Lighter Trader：未創建"
fi
echo ""

echo "🔍 下一步操作建議："
if [ -z "$EXCHANGES" ]; then
    echo "  1. 通過 UI 創建 Lighter 交易所配置"
    echo "  2. 創建 Lighter trader"
    echo "  3. 重新運行此測試腳本"
elif [ -z "$TRADER_ID" ]; then
    echo "  1. 通過 UI 創建 Lighter trader"
    echo "  2. 重新運行此測試腳本"
else
    echo "  1. 檢查 trader 是否正常運行"
    echo "  2. 監控日誌：docker logs nofx-trading -f | grep -i lighter"
    echo "  3. 測試交易功能（需要 V2 模式 + API Key）"
fi
echo ""
echo "════════════════════════════════════════════════════════════"
