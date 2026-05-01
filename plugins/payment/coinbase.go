package payment

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/xboard/xboard/internal/plugin"
)

// Coinbase implements Coinbase Commerce payment gateway
type Coinbase struct{}

func (p *Coinbase) Code() string        { return "coinbase" }
func (p *Coinbase) Name() string        { return "Coinbase Commerce" }
func (p *Coinbase) Version() string     { return "2.0.0" }
func (p *Coinbase) Description() string { return "Coinbase Commerce - 加密货币支付" }
func (p *Coinbase) Author() string      { return "Xboard Team" }
func (p *Coinbase) Type() plugin.PluginType {
	return plugin.TypePayment
}
func (p *Coinbase) Boot(ctx *plugin.Context) error { return nil }
func (p *Coinbase) Install() error                 { return nil }
func (p *Coinbase) Uninstall() error               { return nil }
func (p *Coinbase) Update(old, new string) error   { return nil }

func (p *Coinbase) Form() []plugin.FormField {
	return []plugin.FormField{
		{Key: "coinbase_url", Type: "text", Label: "接口地址", Required: true, Description: "Coinbase Commerce API地址"},
		{Key: "coinbase_api_key", Type: "password", Label: "API KEY", Required: true, Description: "Coinbase Commerce API密钥"},
		{Key: "coinbase_webhook_key", Type: "password", Label: "WEBHOOK KEY", Required: true, Description: "Webhook签名验证密钥"},
	}
}

func (p *Coinbase) Pay(order *plugin.PaymentOrder) (*plugin.PaymentResult, error) {
	cfg := getPluginSettings()

	apiURL := getString(cfg, "coinbase_url")
	apiKey := getString(cfg, "coinbase_api_key")

	if apiURL == "" || apiKey == "" {
		return nil, fmt.Errorf("missing coinbase config (url, api_key)")
	}

	// Build charge request body
	payload := map[string]interface{}{
		"name":         "订阅套餐",
		"description":  "订单号 " + order.TradeNo,
		"pricing_type": "fixed_price",
		"local_price": map[string]string{
			"amount":   fmt.Sprintf("%.2f", order.TotalAmount/100), // cents to yuan
			"currency": "CNY",
		},
		"metadata": map[string]string{
			"outTradeNo": order.TradeNo,
		},
	}
	payloadJSON, _ := json.Marshal(payload)

	// POST to Coinbase Commerce API
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CC-Api-Key", apiKey)
	req.Header.Set("X-CC-Version", "2018-03-22")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call coinbase api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Data struct {
			HostedURL string `json:"hosted_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Data.HostedURL == "" {
		return nil, fmt.Errorf("coinbase: no hosted_url in response: %s", string(body))
	}

	return &plugin.PaymentResult{
		Type:        "redirect",
		RedirectURL: result.Data.HostedURL,
		TradeNo:     order.TradeNo,
	}, nil
}

func (p *Coinbase) Notify(req interface{}) (*plugin.PaymentNotification, error) {
	cfg := getPluginSettings()
	webhookKey := getString(cfg, "coinbase_webhook_key")

	// req should contain raw payload and headers
	notifyData, ok := req.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid request type")
	}

	// Extract raw payload and signature
	rawPayload, _ := notifyData["_raw_payload"].(string)
	signatureHeader, _ := notifyData["_header_X_Cc_Webhook_Signature"].(string)

	if rawPayload == "" || signatureHeader == "" {
		return nil, fmt.Errorf("missing payload or signature header")
	}

	// Verify HMAC-SHA256 signature
	computedSig := hmacSHA256Hex(webhookKey, rawPayload)
	if !hmacEqual(signatureHeader, computedSig) {
		return nil, fmt.Errorf("HMAC signature does not match")
	}

	// Parse JSON payload
	var payload struct {
		Event struct {
			ID   string `json:"id"`
			Data struct {
				Metadata struct {
					OutTradeNo string `json:"outTradeNo"`
				} `json:"metadata"`
			} `json:"data"`
		} `json:"event"`
	}
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	return &plugin.PaymentNotification{
		TradeNo:        payload.Event.Data.Metadata.OutTradeNo,
		GatewayTradeNo: payload.Event.ID,
		Status:         "success",
		RawData:        notifyData,
	}, nil
}

// hmacSHA256Hex computes HMAC-SHA256 and returns hex-encoded string
func hmacSHA256Hex(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// hmacEqual does constant-time comparison of two HMAC strings
func hmacEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}
