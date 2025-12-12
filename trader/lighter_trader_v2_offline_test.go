package trader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	lighterClient "github.com/elliottech/lighter-go/client"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type mockMinimalHTTPClient struct {
	nextNonce int64
	apiKey    string
}

func (m mockMinimalHTTPClient) GetNextNonce(accountIndex int64, apiKeyIndex uint8) (int64, error) {
	_ = accountIndex
	_ = apiKeyIndex
	return m.nextNonce, nil
}

func (m mockMinimalHTTPClient) GetApiKey(accountIndex int64, apiKeyIndex uint8) (string, error) {
	_ = accountIndex
	_ = apiKeyIndex
	if m.apiKey == "" {
		return "", fmt.Errorf("no api key configured")
	}
	return m.apiKey, nil
}

func TestLighterTraderV2_fetchMarketList_ParsesUnwrappedResponse(t *testing.T) {
	tr := &LighterTraderV2{
		baseURL: "https://example.test",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v1/orderBooks" {
				return jsonResponse(http.StatusOK, `{"data":[{"symbol":"BTC-PERP","market_index":0}]}`), nil
			}
			return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
		})},
	}

	markets, err := tr.fetchMarketList()
	assert.NoError(t, err)
	if assert.Len(t, markets, 1) {
		assert.Equal(t, "BTC-PERP", markets[0].Symbol)
		assert.Equal(t, int16(0), markets[0].MarketID)
	}
}

func TestLighterTraderV2_getAccountByL1Address_ParsesWrappedResponse(t *testing.T) {
	tr := &LighterTraderV2{
		baseURL:    "https://example.test",
		walletAddr: "0xabc",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v1/account" {
				return jsonResponse(http.StatusOK, `{"code":200,"data":{"account_index":123,"l1_address":"0xabc"}}`), nil
			}
			return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
		})},
	}

	info, err := tr.getAccountByL1Address()
	assert.NoError(t, err)
	assert.Equal(t, int64(123), info.AccountIndex)
}

func TestLighterTraderV2_getAccountByL1Address_FallsBackToLegacyEndpoint(t *testing.T) {
	tr := &LighterTraderV2{
		baseURL:    "https://example.test",
		walletAddr: "0xabc",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method == http.MethodGet && r.URL.Path == "/api/v1/account" {
				return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
			}
			if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/account/by/l1/") {
				return jsonResponse(http.StatusOK, `{"index":456}`), nil
			}
			return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
		})},
	}

	info, err := tr.getAccountByL1Address()
	assert.NoError(t, err)
	assert.Equal(t, int64(456), info.AccountIndex)
}

func TestLighterTraderV2_CreateOrder_RetriesWithAccountFields(t *testing.T) {
	sendCalls := 0
	var sawRetryWithAccountFields bool

	apiKeyHex := "0x" + strings.Repeat("01", 40)
	httpClient := mockMinimalHTTPClient{nextNonce: 7}
	txClient, err := lighterClient.NewTxClient(httpClient, apiKeyHex, 123, 0, 42766)
	assert.NoError(t, err)

	tr := &LighterTraderV2{
		baseURL: "https://example.test",
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sendTx" {
				return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
			}

			reqBody, _ := io.ReadAll(r.Body)
			sendCalls++

			var raw map[string]interface{}
			_ = json.Unmarshal(reqBody, &raw)

			switch sendCalls {
			case 1:
				// First attempt should NOT include these fields (exercise retry).
				_, hasAccountIndex := raw["account_index"]
				assert.False(t, hasAccountIndex)
				return jsonResponse(http.StatusOK, `{"code":400,"message":"missing account_index","data":{}}`), nil

			case 2:
				// Second attempt should include account_index/api_key_index.
				_, hasAccountIndex := raw["account_index"]
				_, hasAPIKeyIndex := raw["api_key_index"]
				assert.True(t, hasAccountIndex)
				assert.True(t, hasAPIKeyIndex)
				sawRetryWithAccountFields = hasAccountIndex && hasAPIKeyIndex

				// Validate basic tx envelope.
				assert.Equal(t, float64(14), raw["tx_type"]) // json decodes numbers as float64
				assert.Equal(t, true, raw["price_protection"])

				txInfoStr, _ := raw["tx_info"].(string)
				var txInfo map[string]interface{}
				_ = json.Unmarshal([]byte(txInfoStr), &txInfo)

				assert.Equal(t, float64(123), txInfo["AccountIndex"])
				assert.Equal(t, float64(0), txInfo["ApiKeyIndex"])
				assert.Equal(t, float64(0), txInfo["MarketIndex"])
				assert.Equal(t, float64(1000000), txInfo["BaseAmount"]) // 0.01 * 1e8
				assert.Equal(t, float64(10500), txInfo["Price"])        // 100*(1+5%) -> 105.00 -> 10500 ticks
				assert.Equal(t, float64(0), txInfo["IsAsk"])
				assert.Equal(t, float64(1), txInfo["Type"])        // MarketOrder
				assert.Equal(t, float64(0), txInfo["TimeInForce"]) // IOC
				assert.Equal(t, float64(0), txInfo["ReduceOnly"])
				assert.Equal(t, float64(0), txInfo["TriggerPrice"])
				assert.Equal(t, float64(0), txInfo["OrderExpiry"])
				assert.Equal(t, float64(7), txInfo["Nonce"])

				return jsonResponse(http.StatusOK, `{"code":200,"message":"","data":{"tx_hash":"0xabc","order_id":"123"}}`), nil
			default:
				return jsonResponse(http.StatusInternalServerError, `{"error":"unexpected call"}`), nil
			}
		})},
		txClient:       txClient,
		accountIndex:   123,
		apiKeyIndex:    0,
		chainID:        42766,
		marketIndexMap: map[string]int16{"BTC-PERP": 0},
		leverageCache:  make(map[string]int),
		marginModeCache: map[string]uint8{
			"BTC-PERP": 0,
		},
	}

	res, err := tr.CreateOrder("BTC-PERP", false, 0.01, 100, "market", false)
	assert.NoError(t, err)
	assert.Equal(t, "123", res["orderId"])
	assert.Equal(t, "0xabc", res["tx_hash"])
	assert.True(t, sawRetryWithAccountFields)
}

func TestLighterTraderV2_GenerateAndRegisterAPIKey_SubmitsChangePubKeyTx(t *testing.T) {
	sendCalls := 0

	l1Key, err := crypto.HexToECDSA(strings.Repeat("11", 32))
	assert.NoError(t, err)

	tr := &LighterTraderV2{
		baseURL:      "https://example.test",
		privateKey:   l1Key,
		walletAddr:   "0xabc",
		accountIndex: 123,
		apiKeyIndex:  0,
		chainID:      42766,
		httpClient:   mockMinimalHTTPClient{nextNonce: 5},
		client: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodPost || r.URL.Path != "/api/v1/sendTx" {
				return jsonResponse(http.StatusNotFound, `{"error":"not found"}`), nil
			}
			sendCalls++

			reqBody, _ := io.ReadAll(r.Body)
			var raw map[string]interface{}
			_ = json.Unmarshal(reqBody, &raw)

			assert.Equal(t, float64(8), raw["tx_type"]) // ChangePubKey

			txInfoStr, _ := raw["tx_info"].(string)
			var txInfo map[string]interface{}
			_ = json.Unmarshal([]byte(txInfoStr), &txInfo)

			l1Sig, _ := txInfo["L1Sig"].(string)
			assert.True(t, strings.HasPrefix(l1Sig, "0x"))
			assert.NotEmpty(t, l1Sig)

			return jsonResponse(http.StatusOK, `{"code":200,"message":"","data":{"tx_hash":"0xdef"}}`), nil
		})},
	}

	priv, pub, err := tr.GenerateAndRegisterAPIKey("seed")
	assert.NoError(t, err)
	assert.True(t, strings.HasPrefix(priv, "0x"))
	assert.True(t, strings.HasPrefix(pub, "0x"))
	assert.NotNil(t, tr.txClient)
	assert.Equal(t, priv, tr.apiKeyPrivateKey)
	assert.Equal(t, 1, sendCalls)

	pubKeyBytes := tr.txClient.GetKeyManager().PubKeyBytes()
	assert.Equal(t, pub, hexutil.Encode(pubKeyBytes[:]))
}
