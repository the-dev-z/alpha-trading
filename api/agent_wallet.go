package api

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/vmihailenco/msgpack/v5"
	"gorm.io/gorm"

	"nofx/logger"
	"nofx/store"
)

const (
	hyperliquidSignatureChainID       = "0xa4b1"
	hyperliquidTypedDataChainID       = 1337
	hyperliquidTypedDataContract      = "0x0000000000000000000000000000000000000000"
	hyperliquidMainnetExchangeURLBase = "https://api.hyperliquid.xyz"
	hyperliquidTestnetExchangeURLBase = "https://api.hyperliquid-testnet.xyz"
)

type CreateAgentWalletRequest struct {
	MainWallet       string `json:"main_wallet" binding:"required"`
	HyperliquidChain string `json:"hyperliquid_chain"`
	Regenerate       bool   `json:"regenerate"`
}

type CreateAgentWalletResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	AgentAddress string `json:"agent_address,omitempty"`
	MainWallet   string `json:"main_wallet,omitempty"`
	Status       string `json:"status,omitempty"`
}

type AgentWalletStatus struct {
	AgentAddress         string     `json:"agent_address"`
	MainWallet           string     `json:"main_wallet"`
	Status               string     `json:"status"`
	HyperliquidChain     string     `json:"hyperliquid_chain"`
	BuilderFeeAuthorized bool       `json:"builder_fee_authorized"`
	BuilderFeeMaxRate    int        `json:"builder_fee_max_rate"`
	BuilderFeeAuthorizedAt *time.Time `json:"builder_fee_authorized_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type GetAgentWalletResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message,omitempty"`
	Data    *AgentWalletStatus `json:"data,omitempty"`
}

type AgentTypedDataRequest struct {
	MainWallet       string `json:"main_wallet" binding:"required"`
	HyperliquidChain string `json:"hyperliquid_chain"`
	ActionType       string `json:"action_type" binding:"required"`
	Nonce            int64  `json:"nonce"`
	AgentName        string `json:"agent_name"`
	BuilderAddress   string `json:"builder_address"`
	MaxFeeRate       int    `json:"max_fee_rate"`
}

type AgentTypedDataResponse struct {
	Success       bool                   `json:"success"`
	Message       string                 `json:"message,omitempty"`
	Nonce         int64                  `json:"nonce"`
	TypedData     map[string]interface{} `json:"typed_data"`
	BuilderAddress string                `json:"builder_address,omitempty"`
	MaxFeeRate    int                    `json:"max_fee_rate,omitempty"`
}

type SignatureRSV struct {
	R string `json:"r" binding:"required"`
	S string `json:"s" binding:"required"`
	V int    `json:"v" binding:"required"`
}

type AuthorizeAgentRequest struct {
	MainWallet    string       `json:"main_wallet" binding:"required"`
	Nonce         int64        `json:"nonce" binding:"required"`
	Signature     string       `json:"signature" binding:"required"`
	SignatureRSV  SignatureRSV `json:"signature_rsv" binding:"required"`
	AgentName     string       `json:"agent_name"`
}

type AuthorizeAgentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Status  string `json:"status,omitempty"`
}

type AuthorizeBuilderFeeRequest struct {
	MainWallet      string       `json:"main_wallet" binding:"required"`
	Nonce           int64        `json:"nonce" binding:"required"`
	Signature       string       `json:"signature" binding:"required"`
	SignatureRSV    SignatureRSV `json:"signature_rsv" binding:"required"`
	BuilderAddress  string       `json:"builder_address"`
	MaxFeeRate      int          `json:"max_fee_rate"`
}

type AuthorizeBuilderFeeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ConfirmBuilderFeeRequest struct {
	MainWallet string `json:"main_wallet" binding:"required"`
	MaxFeeRate int    `json:"max_fee_rate" binding:"required"`
}

type ConfirmBuilderFeeResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type VerifyAuthorizationResponse struct {
	Success           bool      `json:"success"`
	Message           string    `json:"message,omitempty"`
	Authorized        bool      `json:"authorized"`
	BuilderAuthorized bool      `json:"builder_authorized"`
	CheckedAt         time.Time `json:"checked_at"`
}

type approveAgentAction struct {
	Type             string `json:"type" msgpack:"type"`
	SignatureChainID string `json:"signatureChainId" msgpack:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain" msgpack:"hyperliquidChain"`
	AgentAddress     string `json:"agentAddress" msgpack:"agentAddress"`
	Nonce            int64  `json:"nonce" msgpack:"nonce"`
	AgentName        string `json:"agentName,omitempty" msgpack:"agentName,omitempty"`
}

type approveBuilderFeeAction struct {
	Type             string `json:"type" msgpack:"type"`
	SignatureChainID string `json:"signatureChainId" msgpack:"signatureChainId"`
	HyperliquidChain string `json:"hyperliquidChain" msgpack:"hyperliquidChain"`
	BuilderAddress   string `json:"builderAddress" msgpack:"builderAddress"`
	MaxFeeRate       int    `json:"maxFeeRate" msgpack:"maxFeeRate"`
	Nonce            int64  `json:"nonce" msgpack:"nonce"`
}

func (s *Server) handleCreateAgentWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, CreateAgentWalletResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req CreateAgentWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CreateAgentWalletResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	mainWallet := normalizeAddress(req.MainWallet)
	if !common.IsHexAddress(mainWallet) {
		c.JSON(http.StatusBadRequest, CreateAgentWalletResponse{
			Success: false,
			Message: "Invalid wallet address format",
		})
		return
	}

	chain, err := normalizeHyperliquidChain(req.HyperliquidChain)
	if err != nil {
		c.JSON(http.StatusBadRequest, CreateAgentWalletResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	if s.store == nil {
		c.JSON(http.StatusInternalServerError, CreateAgentWalletResponse{
			Success: false,
			Message: "Agent wallet store not available",
		})
		return
	}

	if existing, err := s.store.AgentWallet().GetByMainWallet(userID, mainWallet); err == nil && existing != nil {
		if req.Regenerate {
			if err := s.store.AgentWallet().DeleteByMainWallet(userID, mainWallet); err != nil {
				c.JSON(http.StatusInternalServerError, CreateAgentWalletResponse{
					Success: false,
					Message: "Failed to delete existing agent wallet: " + err.Error(),
				})
				return
			}
		} else {
			c.JSON(http.StatusOK, CreateAgentWalletResponse{
				Success:      true,
				Message:      "Agent wallet already exists",
				AgentAddress: existing.AgentAddress,
				MainWallet:   mainWallet,
				Status:       existing.Status,
			})
			return
		}
	}

	privateKey, err := ethcrypto.GenerateKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, CreateAgentWalletResponse{
			Success: false,
			Message: "Failed to generate agent wallet: " + err.Error(),
		})
		return
	}

	agentAddress := strings.ToLower(ethcrypto.PubkeyToAddress(privateKey.PublicKey).Hex())
	privateKeyHex := "0x" + hex.EncodeToString(ethcrypto.FromECDSA(privateKey))

	wallet, err := s.store.AgentWallet().Create(userID, mainWallet, agentAddress, privateKeyHex, chain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, CreateAgentWalletResponse{
			Success: false,
			Message: "Failed to save agent wallet: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, CreateAgentWalletResponse{
		Success:      true,
		Message:      "Agent wallet created successfully. Please authorize on Hyperliquid to activate.",
		AgentAddress: wallet.AgentAddress,
		MainWallet:   wallet.MainWallet,
		Status:       wallet.Status,
	})
}

func (s *Server) handleGetAgentWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, GetAgentWalletResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	mainWallet := normalizeAddress(c.Query("main_wallet"))
	if mainWallet == "" {
		c.JSON(http.StatusBadRequest, GetAgentWalletResponse{
			Success: false,
			Message: "Missing main_wallet parameter",
		})
		return
	}

	if s.store == nil {
		c.JSON(http.StatusInternalServerError, GetAgentWalletResponse{
			Success: false,
			Message: "Agent wallet store not available",
		})
		return
	}

	wallet, err := s.store.AgentWallet().GetByMainWallet(userID, mainWallet)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, GetAgentWalletResponse{
				Success: false,
				Message: "Agent wallet not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, GetAgentWalletResponse{
			Success: false,
			Message: "Database error: " + err.Error(),
		})
		return
	}

	status := agentWalletStatusFromModel(wallet)
	c.JSON(http.StatusOK, GetAgentWalletResponse{
		Success: true,
		Data:    status,
	})
}

func (s *Server) handleGetAgentTypedData(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, AgentTypedDataResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req AgentTypedDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AgentTypedDataResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	mainWallet := normalizeAddress(req.MainWallet)
	if !common.IsHexAddress(mainWallet) {
		c.JSON(http.StatusBadRequest, AgentTypedDataResponse{
			Success: false,
			Message: "Invalid wallet address format",
		})
		return
	}

	if s.store == nil {
		c.JSON(http.StatusInternalServerError, AgentTypedDataResponse{
			Success: false,
			Message: "Agent wallet store not available",
		})
		return
	}

	wallet, err := s.store.AgentWallet().GetByMainWallet(userID, mainWallet)
	if err != nil {
		c.JSON(http.StatusNotFound, AgentTypedDataResponse{
			Success: false,
			Message: "Agent wallet not found. Please create one first.",
		})
		return
	}

	chain := wallet.HyperliquidChain
	if req.HyperliquidChain != "" {
		normalizedChain, err := normalizeHyperliquidChain(req.HyperliquidChain)
		if err != nil {
			c.JSON(http.StatusBadRequest, AgentTypedDataResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}
		chain = normalizedChain
	}

	nonce := req.Nonce
	if nonce <= 0 {
		nonce = time.Now().UnixMilli()
	}

	actionType := strings.TrimSpace(req.ActionType)
	var action interface{}
	builderAddress := strings.TrimSpace(req.BuilderAddress)
	maxFeeRate := req.MaxFeeRate

	switch actionType {
	case "approveAgent":
		action = approveAgentAction{
			Type:             "approveAgent",
			SignatureChainID: hyperliquidSignatureChainID,
			HyperliquidChain: chain,
			AgentAddress:     wallet.AgentAddress,
			Nonce:            nonce,
			AgentName:        strings.TrimSpace(req.AgentName),
		}
	case "approveBuilderFee":
		if builderAddress == "" {
			builderAddress = strings.TrimSpace(os.Getenv("NOFX_HYPERLIQUID_BUILDER_ADDRESS"))
		}
		if maxFeeRate <= 0 {
			maxFeeRate = getEnvInt("NOFX_HYPERLIQUID_BUILDER_FEE_RATE", 0)
		}
		if builderAddress == "" || maxFeeRate <= 0 {
			c.JSON(http.StatusBadRequest, AgentTypedDataResponse{
				Success: false,
				Message: "Builder address and max fee rate are required",
			})
			return
		}
		if !common.IsHexAddress(builderAddress) {
			c.JSON(http.StatusBadRequest, AgentTypedDataResponse{
				Success: false,
				Message: "Invalid builder address format",
			})
			return
		}
		action = approveBuilderFeeAction{
			Type:             "approveBuilderFee",
			SignatureChainID: hyperliquidSignatureChainID,
			HyperliquidChain: chain,
			BuilderAddress:   strings.ToLower(builderAddress),
			MaxFeeRate:       maxFeeRate,
			Nonce:            nonce,
		}
	default:
		c.JSON(http.StatusBadRequest, AgentTypedDataResponse{
			Success: false,
			Message: "Invalid action type",
		})
		return
	}

	typedData, err := buildHyperliquidTypedData(action, nonce, strings.EqualFold(chain, "Mainnet"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, AgentTypedDataResponse{
			Success: false,
			Message: "Failed to build typed data: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AgentTypedDataResponse{
		Success:       true,
		Message:       "Typed data ready",
		Nonce:         nonce,
		TypedData:     typedData,
		BuilderAddress: builderAddress,
		MaxFeeRate:    maxFeeRate,
	})
}

func (s *Server) handleAuthorizeAgentWallet(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, AuthorizeAgentResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req AuthorizeAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AuthorizeAgentResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	if req.Nonce <= 0 {
		c.JSON(http.StatusBadRequest, AuthorizeAgentResponse{
			Success: false,
			Message: "Invalid nonce",
		})
		return
	}

	mainWallet := normalizeAddress(req.MainWallet)
	if !common.IsHexAddress(mainWallet) {
		c.JSON(http.StatusBadRequest, AuthorizeAgentResponse{
			Success: false,
			Message: "Invalid wallet address format",
		})
		return
	}

	if s.store == nil {
		c.JSON(http.StatusInternalServerError, AuthorizeAgentResponse{
			Success: false,
			Message: "Agent wallet store not available",
		})
		return
	}

	wallet, err := s.store.AgentWallet().GetByMainWallet(userID, mainWallet)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, AuthorizeAgentResponse{
				Success: false,
				Message: "Agent wallet not found. Please create one first.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, AuthorizeAgentResponse{
			Success: false,
			Message: "Database error: " + err.Error(),
		})
		return
	}

	action := approveAgentAction{
		Type:             "approveAgent",
		SignatureChainID: hyperliquidSignatureChainID,
		HyperliquidChain: wallet.HyperliquidChain,
		AgentAddress:     wallet.AgentAddress,
		Nonce:            req.Nonce,
		AgentName:        strings.TrimSpace(req.AgentName),
	}

	if err := submitHyperliquidAction(wallet.HyperliquidChain, action, req.Nonce, req.SignatureRSV); err != nil {
		c.JSON(http.StatusBadRequest, AuthorizeAgentResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	if err := s.store.AgentWallet().UpdateAuthorization(userID, mainWallet, normalizeSignature(req.Signature)); err != nil {
		c.JSON(http.StatusInternalServerError, AuthorizeAgentResponse{
			Success: false,
			Message: "Failed to update database: " + err.Error(),
		})
		return
	}

	if err := s.traderManager.LoadUserTradersFromStore(s.store, userID); err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	c.JSON(http.StatusOK, AuthorizeAgentResponse{
		Success: true,
		Message: "Agent wallet authorized successfully!",
		Status:  "ACTIVE",
	})
}

func (s *Server) handleAuthorizeBuilderFee(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req AuthorizeBuilderFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	if req.Nonce <= 0 {
		c.JSON(http.StatusBadRequest, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Invalid nonce",
		})
		return
	}

	mainWallet := normalizeAddress(req.MainWallet)
	if !common.IsHexAddress(mainWallet) {
		c.JSON(http.StatusBadRequest, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Invalid wallet address format",
		})
		return
	}

	if s.store == nil {
		c.JSON(http.StatusInternalServerError, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Agent wallet store not available",
		})
		return
	}

	wallet, err := s.store.AgentWallet().GetByMainWallet(userID, mainWallet)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, AuthorizeBuilderFeeResponse{
				Success: false,
				Message: "Agent wallet not found. Please create one first.",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Database error: " + err.Error(),
		})
		return
	}

	builderAddress := strings.TrimSpace(req.BuilderAddress)
	if builderAddress == "" {
		builderAddress = strings.TrimSpace(os.Getenv("NOFX_HYPERLIQUID_BUILDER_ADDRESS"))
	}
	maxFeeRate := req.MaxFeeRate
	if maxFeeRate <= 0 {
		maxFeeRate = getEnvInt("NOFX_HYPERLIQUID_BUILDER_FEE_RATE", 0)
	}
	if builderAddress == "" || maxFeeRate <= 0 {
		c.JSON(http.StatusBadRequest, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Builder address and max fee rate are required",
		})
		return
	}
	if !common.IsHexAddress(builderAddress) {
		c.JSON(http.StatusBadRequest, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Invalid builder address format",
		})
		return
	}

	action := approveBuilderFeeAction{
		Type:             "approveBuilderFee",
		SignatureChainID: hyperliquidSignatureChainID,
		HyperliquidChain: wallet.HyperliquidChain,
		BuilderAddress:   strings.ToLower(builderAddress),
		MaxFeeRate:       maxFeeRate,
		Nonce:            req.Nonce,
	}

	if err := submitHyperliquidAction(wallet.HyperliquidChain, action, req.Nonce, req.SignatureRSV); err != nil {
		c.JSON(http.StatusBadRequest, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	if err := s.store.AgentWallet().UpdateBuilderFee(userID, mainWallet, builderAddress, maxFeeRate, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, AuthorizeBuilderFeeResponse{
			Success: false,
			Message: "Failed to update database: " + err.Error(),
		})
		return
	}

	if err := s.traderManager.LoadUserTradersFromStore(s.store, userID); err != nil {
		logger.Infof("⚠️ Failed to reload user traders into memory: %v", err)
	}

	c.JSON(http.StatusOK, AuthorizeBuilderFeeResponse{
		Success: true,
		Message: "Builder fee authorized successfully!",
	})
}

func (s *Server) handleConfirmBuilderFee(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, ConfirmBuilderFeeResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	var req ConfirmBuilderFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ConfirmBuilderFeeResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	mainWallet := normalizeAddress(req.MainWallet)
	if !common.IsHexAddress(mainWallet) {
		c.JSON(http.StatusBadRequest, ConfirmBuilderFeeResponse{
			Success: false,
			Message: "Invalid wallet address format",
		})
		return
	}

	if s.store == nil {
		c.JSON(http.StatusInternalServerError, ConfirmBuilderFeeResponse{
			Success: false,
			Message: "Agent wallet store not available",
		})
		return
	}

	wallet, err := s.store.AgentWallet().GetByMainWallet(userID, mainWallet)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, ConfirmBuilderFeeResponse{
				Success: false,
				Message: "Agent wallet not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, ConfirmBuilderFeeResponse{
			Success: false,
			Message: "Database error: " + err.Error(),
		})
		return
	}

	if wallet.BuilderFeeAuthorized {
		c.JSON(http.StatusOK, ConfirmBuilderFeeResponse{
			Success: true,
			Message: "Builder fee already confirmed",
		})
		return
	}

	builderAddress := wallet.BuilderAddress
	if builderAddress == "" {
		builderAddress = strings.TrimSpace(os.Getenv("NOFX_HYPERLIQUID_BUILDER_ADDRESS"))
	}

	if err := s.store.AgentWallet().UpdateBuilderFee(userID, mainWallet, builderAddress, req.MaxFeeRate, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, ConfirmBuilderFeeResponse{
			Success: false,
			Message: "Failed to update database: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ConfirmBuilderFeeResponse{
		Success: true,
		Message: "Builder fee authorization confirmed successfully!",
	})
}

func (s *Server) handleVerifyAgentAuthorization(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, VerifyAuthorizationResponse{
			Success: false,
			Message: "Unauthorized",
		})
		return
	}

	mainWallet := normalizeAddress(c.Query("main_wallet"))
	if mainWallet == "" {
		c.JSON(http.StatusBadRequest, VerifyAuthorizationResponse{
			Success: false,
			Message: "main_wallet parameter is required",
		})
		return
	}

	if s.store == nil {
		c.JSON(http.StatusInternalServerError, VerifyAuthorizationResponse{
			Success: false,
			Message: "Agent wallet store not available",
		})
		return
	}

	wallet, err := s.store.AgentWallet().GetByMainWallet(userID, mainWallet)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, VerifyAuthorizationResponse{
				Success: false,
				Message: "Agent wallet not found in database",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, VerifyAuthorizationResponse{
			Success: false,
			Message: "Database error: " + err.Error(),
		})
		return
	}

	apiURL := hyperliquidInfoURL(wallet.HyperliquidChain)
	requestBody := map[string]interface{}{
		"type": "extraAgents",
		"user": mainWallet,
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifyAuthorizationResponse{
			Success: false,
			Message: "Failed to marshal request: " + err.Error(),
		})
		return
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, VerifyAuthorizationResponse{
			Success: false,
			Message: "Failed to query Hyperliquid: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var agents []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&agents); err != nil {
		c.JSON(http.StatusInternalServerError, VerifyAuthorizationResponse{
			Success: false,
			Message: "Failed to parse Hyperliquid response: " + err.Error(),
		})
		return
	}

	authorized := false
	for _, agent := range agents {
		if agentAddr, ok := agent["address"].(string); ok {
			if strings.EqualFold(agentAddr, wallet.AgentAddress) {
				authorized = true
				break
			}
		}
	}

	c.JSON(http.StatusOK, VerifyAuthorizationResponse{
		Success:           true,
		Authorized:        authorized,
		BuilderAuthorized: wallet.BuilderFeeAuthorized,
		CheckedAt:         time.Now(),
	})
}

func normalizeAddress(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

func normalizeHyperliquidChain(chain string) (string, error) {
	if chain == "" {
		return "Mainnet", nil
	}
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "mainnet":
		return "Mainnet", nil
	case "testnet":
		return "Testnet", nil
	default:
		return "", fmt.Errorf("Invalid hyperliquid_chain, must be 'Mainnet' or 'Testnet'")
	}
}

func hyperliquidExchangeURL(chain string) string {
	if strings.EqualFold(chain, "Testnet") {
		return hyperliquidTestnetExchangeURLBase + "/exchange"
	}
	return hyperliquidMainnetExchangeURLBase + "/exchange"
}

func hyperliquidInfoURL(chain string) string {
	if strings.EqualFold(chain, "Testnet") {
		return hyperliquidTestnetExchangeURLBase + "/info"
	}
	return hyperliquidMainnetExchangeURLBase + "/info"
}

func normalizeSignature(signature string) string {
	trimmed := strings.TrimSpace(signature)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "0x") {
		return trimmed
	}
	return "0x" + trimmed
}

func normalizeSignatureRSV(sig SignatureRSV) SignatureRSV {
	r := strings.TrimSpace(sig.R)
	s := strings.TrimSpace(sig.S)
	if !strings.HasPrefix(r, "0x") {
		r = "0x" + r
	}
	if !strings.HasPrefix(s, "0x") {
		s = "0x" + s
	}
	v := sig.V
	if v == 0 || v == 1 {
		v = v + 27
	}
	return SignatureRSV{R: r, S: s, V: v}
}

func submitHyperliquidAction(chain string, action interface{}, nonce int64, sig SignatureRSV) error {
	signature := normalizeSignatureRSV(sig)
	requestBody := map[string]interface{}{
		"action": action,
		"nonce":  nonce,
		"signature": map[string]interface{}{
			"r": signature.R,
			"s": signature.S,
			"v": signature.V,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("Failed to marshal request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(hyperliquidExchangeURL(chain), "application/json", bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("Failed to submit to Hyperliquid: %w", err)
	}
	defer resp.Body.Close()

	var hyperliquidResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&hyperliquidResp); err != nil {
		return fmt.Errorf("Failed to parse Hyperliquid response: %w", err)
	}

	if status, ok := hyperliquidResp["status"].(string); ok && status == "err" {
		errorMsg := "Unknown error"
		if response, ok := hyperliquidResp["response"].(string); ok {
			errorMsg = response
		}
		return fmt.Errorf("Hyperliquid rejected authorization: %s", errorMsg)
	}
	return nil
}

func buildHyperliquidTypedData(action interface{}, nonce int64, isMainnet bool) (map[string]interface{}, error) {
	if nonce <= 0 {
		return nil, fmt.Errorf("Invalid nonce")
	}

	var buf bytes.Buffer
	encoder := msgpack.NewEncoder(&buf)
	encoder.UseCompactInts(true)
	if err := encoder.Encode(action); err != nil {
		return nil, fmt.Errorf("msgpack encode action: %w", err)
	}
	msgpackBytes := convertStr16ToStr8(buf.Bytes())

	var nonceBytes [8]byte
	binary.BigEndian.PutUint64(nonceBytes[:], uint64(nonce))

	payload := make([]byte, 0, len(msgpackBytes)+len(nonceBytes)+1)
	payload = append(payload, msgpackBytes...)
	payload = append(payload, nonceBytes[:]...)
	payload = append(payload, 0x00)

	connectionID := ethcrypto.Keccak256(payload)
	connectionHex := "0x" + hex.EncodeToString(connectionID)

	source := "a"
	if !isMainnet {
		source = "b"
	}

	typedData := map[string]interface{}{
		"types": map[string]interface{}{
			"EIP712Domain": []map[string]string{
				{"name": "name", "type": "string"},
				{"name": "version", "type": "string"},
				{"name": "chainId", "type": "uint256"},
				{"name": "verifyingContract", "type": "address"},
			},
			"Agent": []map[string]string{
				{"name": "source", "type": "string"},
				{"name": "connectionId", "type": "bytes32"},
			},
		},
		"primaryType": "Agent",
		"domain": map[string]interface{}{
			"name":              "Exchange",
			"version":           "1",
			"chainId":           hyperliquidTypedDataChainID,
			"verifyingContract": hyperliquidTypedDataContract,
		},
		"message": map[string]interface{}{
			"source":       source,
			"connectionId": connectionHex,
		},
	}
	return typedData, nil
}

func convertStr16ToStr8(data []byte) []byte {
	result := make([]byte, 0, len(data))
	for i := 0; i < len(data); {
		if data[i] == 0xda && i+2 < len(data) {
			length := int(data[i+1])<<8 | int(data[i+2])
			if length < 256 && i+3+length <= len(data) {
				result = append(result, 0xd9, byte(length))
				result = append(result, data[i+3:i+3+length]...)
				i += 3 + length
				continue
			}
		}
		result = append(result, data[i])
		i++
	}
	return result
}

func getEnvInt(key string, defaultVal int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

func agentWalletStatusFromModel(wallet *store.AgentWallet) *AgentWalletStatus {
	if wallet == nil {
		return nil
	}
	return &AgentWalletStatus{
		AgentAddress:           wallet.AgentAddress,
		MainWallet:             wallet.MainWallet,
		Status:                 wallet.Status,
		HyperliquidChain:       wallet.HyperliquidChain,
		BuilderFeeAuthorized:   wallet.BuilderFeeAuthorized,
		BuilderFeeMaxRate:      wallet.BuilderFeeMaxRate,
		BuilderFeeAuthorizedAt: wallet.BuilderFeeAuthorizedAt,
		CreatedAt:              wallet.CreatedAt,
		UpdatedAt:              wallet.UpdatedAt,
	}
}

func (s *Server) resolveHyperliquidCredentials(userID string, exchangeCfg *store.Exchange) (string, string, int) {
	if exchangeCfg == nil {
		return "", "", 0
	}

	privateKey := strings.TrimSpace(string(exchangeCfg.APIKey))
	if privateKey != "" {
		return privateKey, "", 0
	}

	if s.store == nil {
		return "", "", 0
	}

	walletAddr := strings.TrimSpace(exchangeCfg.HyperliquidWalletAddr)
	if walletAddr == "" {
		return "", "", 0
	}

	agentWallet, err := s.store.AgentWallet().GetActiveByMainWallet(userID, walletAddr)
	if err != nil {
		return "", "", 0
	}

	builderAddress := ""
	builderFeeRate := 0
	if agentWallet.BuilderFeeAuthorized && agentWallet.BuilderFeeMaxRate > 0 {
		builderAddress = strings.TrimSpace(agentWallet.BuilderAddress)
		builderFeeRate = agentWallet.BuilderFeeMaxRate
	}

	return string(agentWallet.EncryptedPrivateKey), builderAddress, builderFeeRate
}
