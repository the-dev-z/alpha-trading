package aster

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"nofx/logger"
	"nofx/store"

	"github.com/ethereum/go-ethereum/crypto"
)

// getAsterAgentCode 獲取 Aster Agent Code（三層優先級）
// 1. 數據庫配置（最高優先級，可動態修改）
// 2. 環境變數（部署時配置）
// 3. 空字符串（向後相容）
func getAsterAgentCode(database *store.Store) string {
	// 優先級 1: 從數據庫讀取
	if database != nil {
		if val, err := database.GetSystemConfig("aster_agent_code"); err == nil && val != "" {
			logger.Infof("✓ 使用數據庫配置的 Aster Agent Code: %s", val)
			return val
		}
	}

	// 優先級 2: 從環境變數讀取
	if envCode := os.Getenv("NOFX_ASTER_AGENT_CODE"); envCode != "" {
		logger.Infof("✓ 使用環境變數的 Aster Agent Code: %s", envCode)
		return envCode
	}

	// 優先級 3: 空字符串（不啟用 Referral）
	logger.Infof("⚠️ 未配置 Aster Agent Code，將不會獲得 Referral 佣金")
	logger.Infof("   提示：可通過以下方式配置：")
	logger.Infof("   1. 數據庫：INSERT INTO system_config (key, value) VALUES ('aster_agent_code', 'YOUR_CODE');")
	logger.Infof("   2. 環境變數：export NOFX_ASTER_AGENT_CODE=YOUR_CODE")
	return ""
}

// AsterLoginRequest 登錄請求
type AsterLoginRequest struct {
	Signature  string `json:"signature"`
	SourceAddr string `json:"sourceAddr"`
	ChainID    int    `json:"chainId"`
	AgentCode  string `json:"agentCode,omitempty"` // 可選，但推薦提供
}

// AsterLoginResponse 登錄響應
type AsterLoginResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
		UID   int64  `json:"uid"`
	} `json:"data"`
	Success bool `json:"success"`
}

// AsterGetNonceResponse 獲取 Nonce 響應
type AsterGetNonceResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Nonce string `json:"nonce"`
	} `json:"data"`
	Success bool `json:"success"`
}

// asterLoginWithAgentCode 使用 Agent Code 登錄 Aster（完整流程）
// 這是獲得 Referral 佣金的關鍵步驟！
func asterLoginWithAgentCode(user, privateKeyHex, agentCode string, client *http.Client) (string, error) {
	baseURL := "https://www.asterdex.com"

	// Step 1: 獲取 Nonce
	logger.Infof("📝 [Aster 登錄] Step 1/3: 獲取 Nonce...")
	nonce, err := AsterGetNonce(user, "WEB3_LOGIN", client)
	if err != nil {
		logger.Warnf("⚠️ WEB3_LOGIN nonce 失敗，嘗試 CREATE_API_KEY: %v", err)
		nonce, err = AsterGetNonce(user, "CREATE_API_KEY", client)
		if err != nil {
			return "", fmt.Errorf("獲取 nonce 失敗: %w", err)
		}
	}
	logger.Infof("✓ Nonce 獲取成功: %s", nonce)

	// Step 2: 簽名消息
	logger.Infof("📝 [Aster 登錄] Step 2/3: 簽名消息...")
	message := fmt.Sprintf("You are signing into Astherus %s", nonce)

	// 解析私鑰
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("解析私鑰失敗: %w", err)
	}

	// EIP-191 簽名
	prefixedMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	msgHash := crypto.Keccak256Hash([]byte(prefixedMsg))
	signature, err := crypto.Sign(msgHash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("簽名失敗: %w", err)
	}

	// 調整 V 值
	if len(signature) != 65 {
		return "", fmt.Errorf("簽名長度異常: %d", len(signature))
	}
	signature[64] += 27

	signatureHex := "0x" + fmt.Sprintf("%x", signature)
	logger.Infof("✓ 簽名成功")

	// Step 3: 登錄並綁定 Agent Code
	logger.Infof("📝 [Aster 登錄] Step 3/3: 登錄並綁定 Agent Code...")
	loginReq := AsterLoginRequest{
		Signature:  signatureHex,
		SourceAddr: user,
		ChainID:    56, // BSC
		AgentCode:  agentCode,
	}

	if agentCode != "" {
		logger.Infof("🎯 綁定 Agent Code: %s (將獲得 Referral 佣金)", agentCode)
	} else {
		logger.Infof("⚠️ 未提供 Agent Code (將不會獲得 Referral 佣金)")
	}

	loginBody, _ := json.Marshal(loginReq)
	loginResp, err := asterPostJSON(
		client,
		baseURL+"/bapi/futures/v1/public/future/web3/ae/login",
		loginBody,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("登錄失敗: %w", err)
	}
	defer loginResp.Body.Close()

	var loginData AsterLoginResponse
	loginBodyBytes, _ := io.ReadAll(loginResp.Body)
	if err := json.Unmarshal(loginBodyBytes, &loginData); err != nil {
		return "", fmt.Errorf("解析登錄響應失敗: %w, body: %s", err, string(loginBodyBytes))
	}

	if !loginData.Success {
		return "", fmt.Errorf("登錄失敗: code=%s, message=%s", loginData.Code, loginData.Message)
	}

	logger.Infof("✓ 登錄成功！")
	if agentCode != "" {
		logger.Infof("  └─ Agent Code 已綁定: %s", agentCode)
		logger.Infof("  └─ Referral 佣金已啟用: 10-20%%")
	}
	logger.Infof("  └─ Token: %s...", loginData.Data.Token[:20])
	logger.Infof("  └─ UID: %d", loginData.Data.UID)

	return loginData.Data.Token, nil
}

// AsterCreateApiKeyRequest 創建 API Key 請求
type AsterCreateApiKeyRequest struct {
	Desc       string `json:"desc"`
	IP         string `json:"ip"`
	Network    string `json:"network"`
	Signature  string `json:"signature"`
	SourceAddr string `json:"sourceAddr"`
	Type       string `json:"type"`
	SourceCode string `json:"sourceCode"` // "broker" 啟用 Broker 模式
}

// AsterCreateApiKeyResponse 創建 API Key 響應
type AsterCreateApiKeyResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    struct {
		APIKey    string `json:"apiKey"`
		APISecret string `json:"apiSecret"`
		SecretKey string `json:"secretKey"`
	} `json:"data"`
	Success bool `json:"success"`
}

const asterClientTypeHeader = "broker"

func asterPostJSON(client *http.Client, url string, body []byte, token string) (*http.Response, error) {
	maxRetries := getEnvInt("NOFX_ASTER_RETRY_MAX", 2)
	baseDelayMs := getEnvInt("NOFX_ASTER_RETRY_BASE_MS", 500)
	if maxRetries < 0 {
		maxRetries = 0
	}
	if baseDelayMs <= 0 {
		baseDelayMs = 500
	}
	baseDelay := time.Duration(baseDelayMs) * time.Millisecond

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("clientType", asterClientTypeHeader)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := client.Do(req)
		if err == nil {
			if shouldRetryAsterStatus(resp.StatusCode) && attempt < maxRetries {
				resp.Body.Close()
				time.Sleep(baseDelay * time.Duration(attempt+1))
				continue
			}
			return resp, nil
		}

		lastErr = err
		if attempt < maxRetries && isRetryableAsterError(err) {
			time.Sleep(baseDelay * time.Duration(attempt+1))
			continue
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("Aster request failed")
}

func shouldRetryAsterStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func isRetryableAsterError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return errors.Is(err, io.EOF) ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "EOF")
}

func pickAsterAPISecret(resp *AsterCreateApiKeyResponse) string {
	if resp == nil {
		return ""
	}
	if resp.Data.APISecret != "" {
		return resp.Data.APISecret
	}
	return resp.Data.SecretKey
}

// asterCreateBrokerApiKey 創建 Broker 模式的 API Key
// 注意：這個步驟不是必需的，因為 NoFx 使用現有的 signer/privateKey
// 但如果要自動化整個流程，可以使用這個函數
func asterCreateBrokerApiKey(token, user, privateKeyHex string, client *http.Client) (apiKey, secretKey string, err error) {
	baseURL := "https://www.asterdex.com"

	// 獲取新的 nonce
	nonce, err := AsterGetNonce(user, "CREATE_API_KEY", client)
	if err != nil {
		return "", "", fmt.Errorf("獲取 nonce 失敗: %w", err)
	}

	// 簽名
	message := fmt.Sprintf("You are signing into Astherus %s", nonce)
	signatureHex, err := SignMessageWithPrivateKey(privateKeyHex, message)
	if err != nil {
		return "", "", fmt.Errorf("簽名失敗: %w", err)
	}

	// 創建 API Key（Broker 模式）
	createReq := AsterCreateApiKeyRequest{
		Desc:       "NoFx Trading Bot",
		IP:         "", // 空字符串 = 不限制 IP
		Network:    "56",
		Signature:  signatureHex,
		SourceAddr: user,
		Type:       "CREATE_API_KEY",
		SourceCode: "broker", // ← 啟用 Broker 模式
	}

	createBody, _ := json.Marshal(createReq)
	createResp, err := asterPostJSON(
		client,
		baseURL+"/bapi/futures/v1/public/future/web3/broker-create-api-key",
		createBody,
		token,
	)
	if err != nil {
		return "", "", fmt.Errorf("創建 API Key 失敗: %w", err)
	}
	defer createResp.Body.Close()

	var createData AsterCreateApiKeyResponse
	if err := json.NewDecoder(createResp.Body).Decode(&createData); err != nil {
		return "", "", fmt.Errorf("解析創建響應失敗: %w", err)
	}

	if !createData.Success {
		return "", "", fmt.Errorf("創建 API Key 失敗: %s", createData.Message)
	}

	secretKey = pickAsterAPISecret(&createData)
	if secretKey == "" {
		return "", "", fmt.Errorf("創建 API Key 失敗: empty api secret")
	}

	logger.Infof("✓ API Key 創建成功（Broker 模式）")
	return createData.Data.APIKey, secretKey, nil
}

// setAsterAgentCodeInDB stores Agent Code to database
func setAsterAgentCodeInDB(database *store.Store, agentCode string) error {
	if database == nil {
		return fmt.Errorf("數據庫未初始化")
	}

	if err := database.SetSystemConfig("aster_agent_code", agentCode); err != nil {
		return fmt.Errorf("保存 Agent Code 到數據庫失敗: %w", err)
	}

	logger.Infof("✓ Agent Code 已保存到數據庫")
	return nil
}

// ========== 公開函數（供 API handler 調用） ==========

// GetAsterAgentCodePublic 公開版本的 getAsterAgentCode
func GetAsterAgentCodePublic(database *store.Store) string {
	return getAsterAgentCode(database)
}

// AsterLoginWithAgentCodePublic 公開版本的 asterLoginWithAgentCode
func AsterLoginWithAgentCodePublic(user, privateKeyHex, agentCode string, client *http.Client) (string, error) {
	return asterLoginWithAgentCode(user, privateKeyHex, agentCode, client)
}

// AsterCreateBrokerApiKeyPublic 公開版本的 asterCreateBrokerApiKey
func AsterCreateBrokerApiKeyPublic(token, user, privateKeyHex string, client *http.Client) (apiKey, secretKey string, err error) {
	return asterCreateBrokerApiKey(token, user, privateKeyHex, client)
}

// ========== 簽名模式函數（用於前端簽名方案） ==========

// AsterLoginWithSignature 使用前端簽名登錄 Aster（簽名模式）
// 與 asterLoginWithAgentCode 不同，這個函數接受已簽名的消息，而不是私鑰
//
// 參數：
// - user: 主錢包地址
// - signature: 前端使用 MetaMask 簽名的結果（0x...格式）
// - nonce: 從 Aster API 獲取的 nonce
// - agentCode: Agent Code（如 "3E58dc"）
// - client: HTTP 客戶端
//
// 返回：
// - token: 登錄 token
// - error: 錯誤信息
func AsterLoginWithSignature(user, signature, nonce, agentCode string, client *http.Client) (string, error) {
	baseURL := "https://www.asterdex.com"

	logger.Infof("📝 [Aster 簽名登錄] 使用前端簽名登錄")
	logger.Infof("  ├─ 地址: %s", user)
	logger.Infof("  ├─ Nonce: %s", nonce)
	if agentCode != "" {
		logger.Infof("  └─ Agent Code: %s", agentCode)
	}

	// 構造登錄請求
	loginReq := AsterLoginRequest{
		Signature:  signature,
		SourceAddr: user,
		ChainID:    56, // BSC
		AgentCode:  agentCode,
	}

	loginBody, _ := json.Marshal(loginReq)
	loginResp, err := asterPostJSON(
		client,
		baseURL+"/bapi/futures/v1/public/future/web3/ae/login",
		loginBody,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("登錄失敗: %w", err)
	}
	defer loginResp.Body.Close()

	var loginData AsterLoginResponse
	loginBodyBytes, _ := io.ReadAll(loginResp.Body)
	if err := json.Unmarshal(loginBodyBytes, &loginData); err != nil {
		return "", fmt.Errorf("解析登錄響應失敗: %w, body: %s", err, string(loginBodyBytes))
	}

	if !loginData.Success {
		return "", fmt.Errorf("登錄失敗: code=%s, message=%s", loginData.Code, loginData.Message)
	}

	logger.Infof("✓ 簽名登錄成功！")
	if agentCode != "" {
		logger.Infof("  └─ Agent Code 已綁定: %s", agentCode)
	}
	logger.Infof("  └─ Token: %s...", loginData.Data.Token[:20])

	return loginData.Data.Token, nil
}

// AsterCreateBrokerApiKeyWithSignature 使用前端簽名創建 API Key（簽名模式）
// 與 asterCreateBrokerApiKey 不同，這個函數接受已簽名的消息，而不是私鑰
//
// 參數：
// - token: 登錄後獲得的 token
// - user: 主錢包地址
// - signature: 前端使用 MetaMask 簽名的結果（0x...格式）
// - nonce: 從 Aster API 獲取的 nonce
// - client: HTTP 客戶端
//
// 返回：
// - apiKey: API Key
// - secretKey: Secret Key
// - error: 錯誤信息
func AsterCreateBrokerApiKeyWithSignature(token, user, signature, nonce string, client *http.Client) (apiKey, secretKey string, err error) {
	baseURL := "https://www.asterdex.com"

	logger.Infof("🔑 [Aster 簽名創建 API Key] 使用前端簽名")
	logger.Infof("  ├─ 地址: %s", user)
	logger.Infof("  └─ Nonce: %s", nonce)

	// 構造創建 API Key 請求（Broker 模式）
	createReq := AsterCreateApiKeyRequest{
		Desc:       "NoFx Trading Bot",
		IP:         "", // 空字符串 = 不限制 IP
		Network:    "56",
		Signature:  signature,
		SourceAddr: user,
		Type:       "CREATE_API_KEY",
		SourceCode: "broker", // ← 啟用 Broker 模式
	}

	createBody, _ := json.Marshal(createReq)
	createResp, err := asterPostJSON(
		client,
		baseURL+"/bapi/futures/v1/public/future/web3/broker-create-api-key",
		createBody,
		token,
	)
	if err != nil {
		return "", "", fmt.Errorf("創建 API Key 失敗: %w", err)
	}
	defer createResp.Body.Close()

	var createData AsterCreateApiKeyResponse
	createBodyBytes, _ := io.ReadAll(createResp.Body)
	if err := json.Unmarshal(createBodyBytes, &createData); err != nil {
		return "", "", fmt.Errorf("解析創建響應失敗: %w, body: %s", err, string(createBodyBytes))
	}

	if !createData.Success {
		return "", "", fmt.Errorf("創建 API Key 失敗: code=%s, message=%s", createData.Code, createData.Message)
	}

	secretKey = pickAsterAPISecret(&createData)
	if secretKey == "" {
		return "", "", fmt.Errorf("創建 API Key 失敗: empty api secret")
	}

	logger.Infof("✓ API Key 創建成功（Broker 模式，使用簽名）")
	logger.Infof("  └─ API Key: %s", createData.Data.APIKey)

	return createData.Data.APIKey, secretKey, nil
}

// ========== 自動生成 API Wallet 模式（類似 Hyperliquid Agent Wallet） ==========

// GenerateAsterAPIWallet 生成新的 API Wallet（類似 Hyperliquid Agent Wallet）
// 返回：API Wallet 地址、私鑰（hex 格式）
func GenerateAsterAPIWallet() (address, privateKeyHex string, err error) {
	// 生成隨機錢包
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return "", "", fmt.Errorf("生成私鑰失敗: %w", err)
	}

	// 獲取地址
	address = crypto.PubkeyToAddress(privateKey.PublicKey).Hex()

	// 轉換私鑰為 hex 格式（不帶 0x 前綴）
	privateKeyBytes := crypto.FromECDSA(privateKey)
	privateKeyHex = fmt.Sprintf("%x", privateKeyBytes)

	logger.Infof("✓ API Wallet 生成成功")
	logger.Infof("  ├─ 地址: %s", address)
	logger.Infof("  └─ 私鑰: %s...", privateKeyHex[:16])

	return address, privateKeyHex, nil
}

// SignMessageWithPrivateKey 使用私鑰簽名消息（EIP-191 Personal Sign）
// 參數：
// - privateKeyHex: 私鑰（hex 格式，可帶或不帶 0x 前綴）
// - message: 要簽名的消息
// 返回：
// - signature: 簽名結果（0x... 格式）
// - error: 錯誤信息
func SignMessageWithPrivateKey(privateKeyHex, message string) (signature string, err error) {
	// 解析私鑰
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return "", fmt.Errorf("解析私鑰失敗: %w", err)
	}

	// EIP-191 Personal Sign
	prefixedMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	msgHash := crypto.Keccak256Hash([]byte(prefixedMsg))

	// 簽名
	signatureBytes, err := crypto.Sign(msgHash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("簽名失敗: %w", err)
	}

	// 調整 V 值（EIP-191 要求）
	if len(signatureBytes) != 65 {
		return "", fmt.Errorf("簽名長度異常: %d", len(signatureBytes))
	}
	signatureBytes[64] += 27

	// 轉換為 hex 格式
	signature = "0x" + fmt.Sprintf("%x", signatureBytes)

	return signature, nil
}

// AsterGetNonce 獲取 Aster Nonce（統一封裝）
// 參數：
// - sourceAddr: 錢包地址
// - nonceType: "WEB3_LOGIN" 或 "CREATE_API_KEY"
// - client: HTTP 客戶端
// 返回：
// - nonce: nonce 字符串
// - error: 錯誤信息
func AsterGetNonce(sourceAddr, nonceType string, client *http.Client) (string, error) {
	baseURL := "https://www.asterdex.com"

	reqBody := []byte(fmt.Sprintf(`{"sourceAddr": "%s", "network": "56", "type": "%s"}`, sourceAddr, nonceType))
	resp, err := asterPostJSON(
		client,
		baseURL+"/bapi/futures/v1/public/future/web3/get-nonce",
		reqBody,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("獲取 nonce 失敗: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("獲取 nonce 失敗: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var nonceData AsterGetNonceResponse
	if err := json.NewDecoder(resp.Body).Decode(&nonceData); err != nil {
		return "", fmt.Errorf("解析 nonce 響應失敗: %w", err)
	}

	if !nonceData.Success {
		return "", fmt.Errorf("獲取 nonce 失敗: %s", nonceData.Message)
	}

	return nonceData.Data.Nonce, nil
}

// AsterAutoLogin 自動登錄（後端生成簽名）
// 使用 API Wallet 私鑰自動簽名並登錄
// 參數：
// - apiWalletAddr: API Wallet 地址
// - apiWalletPrivKey: API Wallet 私鑰（hex 格式）
// - agentCode: Agent Code（如 "3E58dc"）
// - client: HTTP 客戶端
// 返回：
// - token: 登錄 token
// - error: 錯誤信息
func AsterAutoLogin(apiWalletAddr, apiWalletPrivKey, agentCode string, client *http.Client) (string, error) {
	baseURL := "https://www.asterdex.com"

	logger.Infof("📝 [Aster 自動登錄] 使用 API Wallet")
	logger.Infof("  ├─ API Wallet: %s", apiWalletAddr)
	if agentCode != "" {
		logger.Infof("  └─ Agent Code: %s", agentCode)
	}

	// Step 1: 獲取 Nonce
	nonce, err := AsterGetNonce(apiWalletAddr, "WEB3_LOGIN", client)
	if err != nil {
		return "", err
	}
	logger.Infof("✓ Nonce: %s", nonce)

	// Step 2: 後端簽名
	message := fmt.Sprintf("You are signing into Astherus %s", nonce)
	signature, err := SignMessageWithPrivateKey(apiWalletPrivKey, message)
	if err != nil {
		return "", err
	}
	logger.Infof("✓ 簽名完成")

	// Step 3: 登錄
	loginReq := AsterLoginRequest{
		Signature:  signature,
		SourceAddr: apiWalletAddr,
		ChainID:    56,
		AgentCode:  agentCode,
	}

	loginBody, _ := json.Marshal(loginReq)
	loginResp, err := asterPostJSON(
		client,
		baseURL+"/bapi/futures/v1/public/future/web3/ae/login",
		loginBody,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("登錄失敗: %w", err)
	}
	defer loginResp.Body.Close()

	var loginData AsterLoginResponse
	loginBodyBytes, _ := io.ReadAll(loginResp.Body)
	if err := json.Unmarshal(loginBodyBytes, &loginData); err != nil {
		return "", fmt.Errorf("解析登錄響應失敗: %w", err)
	}

	if !loginData.Success {
		return "", fmt.Errorf("登錄失敗: code=%s, message=%s", loginData.Code, loginData.Message)
	}

	logger.Infof("✓ 自動登錄成功！")
	if agentCode != "" {
		logger.Infof("  └─ Agent Code 已綁定: %s", agentCode)
	}
	logger.Infof("  └─ Token: %s...", loginData.Data.Token[:20])

	return loginData.Data.Token, nil
}

// AsterAutoCreateBrokerApiKey 自動創建 Broker API Key（後端生成簽名）
// 使用 API Wallet 私鑰自動簽名並創建 API Key
// 參數：
// - token: 登錄後獲得的 token
// - apiWalletAddr: API Wallet 地址
// - apiWalletPrivKey: API Wallet 私鑰（hex 格式）
// - client: HTTP 客戶端
// 返回：
// - apiKey: API Key
// - secretKey: Secret Key
// - error: 錯誤信息
func AsterAutoCreateBrokerApiKey(token, apiWalletAddr, apiWalletPrivKey string, client *http.Client) (apiKey, secretKey string, err error) {
	baseURL := "https://www.asterdex.com"

	logger.Infof("🔑 [Aster 自動創建 API Key] 使用 API Wallet")
	logger.Infof("  └─ API Wallet: %s", apiWalletAddr)

	// Step 1: 獲取 Nonce
	nonce, err := AsterGetNonce(apiWalletAddr, "CREATE_API_KEY", client)
	if err != nil {
		return "", "", err
	}
	logger.Infof("✓ Nonce: %s", nonce)

	// Step 2: 後端簽名
	message := fmt.Sprintf("You are signing into Astherus %s", nonce)
	signature, err := SignMessageWithPrivateKey(apiWalletPrivKey, message)
	if err != nil {
		return "", "", err
	}
	logger.Infof("✓ 簽名完成")

	// Step 3: 創建 API Key
	createReq := AsterCreateApiKeyRequest{
		Desc:       "NoFx Trading Bot",
		IP:         "",
		Network:    "56",
		Signature:  signature,
		SourceAddr: apiWalletAddr,
		Type:       "CREATE_API_KEY",
		SourceCode: "broker",
	}

	createBody, _ := json.Marshal(createReq)
	createResp, err := asterPostJSON(
		client,
		baseURL+"/bapi/futures/v1/public/future/web3/broker-create-api-key",
		createBody,
		token,
	)
	if err != nil {
		return "", "", fmt.Errorf("創建 API Key 失敗: %w", err)
	}
	defer createResp.Body.Close()

	var createData AsterCreateApiKeyResponse
	createBodyBytes, _ := io.ReadAll(createResp.Body)
	if err := json.Unmarshal(createBodyBytes, &createData); err != nil {
		return "", "", fmt.Errorf("解析創建響應失敗: %w", err)
	}

	if !createData.Success {
		return "", "", fmt.Errorf("創建 API Key 失敗: code=%s, message=%s", createData.Code, createData.Message)
	}

	secretKey = pickAsterAPISecret(&createData)
	if secretKey == "" {
		return "", "", fmt.Errorf("創建 API Key 失敗: empty api secret")
	}

	logger.Infof("✓ API Key 自動創建成功（Broker 模式）")
	logger.Infof("  └─ API Key: %s", createData.Data.APIKey)

	return createData.Data.APIKey, secretKey, nil
}

func getEnvInt(key string, defaultVal int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultVal
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultVal
	}

	return parsed
}
