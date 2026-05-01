package payment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/xboard/xboard/internal/plugin"
)

// BTCPay implements BTCPay Server payment gateway
type BTCPay struct{}

func (p *BTCPay) Code() string        { return "btcpay" }
func (p *BTCPay) Name() string        { return "BTCPay" }
func (p *BTCPay) Version() string     { return "2.0.0" }
func (p *BTCPay) Description() string { return "BTCPay Server - 自托管比特币支付" }
func (p *BTCPay) Author() string      { return "Xboard Team" }
func (p *BTCPay) Type() plugin.PluginType {
	return plugin.TypePayment
}
func (p *BTCPay) Boot(ctx *plugin.Context) error { return nil }
func (p *BTCPay) Install() error                 { return nil }
func (p *BTCPay) Uninstall() error               { return nil }
func (p *BTCPay) Update(old, new string) error   { return nil }

func (p *BTCPay) Form() []plugin.FormField {
	return []plugin.FormField{
		{Key: "btcpay_url", Type: "text", Label: "API接口所在网址", Required: true, Description: "包含最后的斜杠，例如：https://your-btcpay.com/"},
		{Key: "btcpay_storeId", Type: "text", Label: "Store ID", Required: true, Description: "BTCPay商店标识符"},
		{Key: "btcpay_api_key", Type: "password", Label: "API KEY", Required: true, Description: "个人设置中的API KEY(非商店设置中的)"},
		{Key: "btcpay_webhook_key", Type: "password", Label: "WEBHOOK KEY", Required: true, Description: "Webhook通知密钥"},
	}
}

func (p *BTCPay) Pay(order *plugin.PaymentOrder) (*plugin.PaymentResult, error) {
	cfg := getPluginSettings()

	btcpayURL := getString(cfg, "btcpay_url")
	storeID := getString(cfg, "btcpay_storeId")
	apiKey := getString(cfg, "btcpay_api_key")

	if btcpayURL == "" || storeID == "" || apiKey == "" {
		return nil, fmt.Errorf("missing btcpay config (url, storeId, api_key)")
	}

	// Build invoice request body
	payload := map[string]interface{}{
		"jsonResponse": true,
		"amount":       fmt.Sprintf("%.2f", order.TotalAmount/100), // cents to yuan
		"currency":     "CNY",
		"metadata": map[string]string{
			"orderId": order.TradeNo,
		},
	}
	payloadJSON, _ := json.Marshal(payload)

	// POST to BTCPay Server API
	apiEndpoint := fmt.Sprintf("%sapi/v1/stores/%s/invoices",
		ensureTrailingSlash(btcpayURL), storeID)

	req, err := http.NewRequest("POST", apiEndpoint, bytes.NewReader(payloadJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call btcpay api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		CheckoutLink string `json:"checkoutLink"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if result.CheckoutLink == "" {
		return nil, fmt.Errorf("btcpay: no checkoutLink in response: %s", string(body))
	}

	return &plugin.PaymentResult{
		Type:        "redirect",
		RedirectURL: result.CheckoutLink,
		TradeNo:     order.TradeNo,
	}, nil
}

func (p *BTCPay) Notify(req interface{}) (*plugin.PaymentNotification, error) {
	cfg := getPluginSettings()
	btcpayURL := getString(cfg, "btcpay_url")
	storeID := getString(cfg, "btcpay_storeId")
	apiKey := getString(cfg, "btcpay_api_key")
	webhookKey := getString(cfg, "btcpay_webhook_key")

	notifyData, ok := req.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid request type")
	}

	// Extract raw payload and signature header
	rawPayload, _ := notifyData["_raw_payload"].(string)
	sigHeader, _ := notifyData["_header_Btcpay_Sig"].(string)

	if rawPayload == "" || sigHeader == "" {
		return nil, fmt.Errorf("missing payload or signature header")
	}

	// Verify HMAC-SHA256 signature (format: sha256=<hex>)
	computedSig := "sha256=" + hmacSHA256Hex(webhookKey, rawPayload)
	if !hmacEqual(sigHeader, computedSig) {
		return nil, fmt.Errorf("HMAC signature does not match")
	}

	// Parse webhook payload
	var payload struct {
		InvoiceID string `json:"invoiceId"`
	}
	if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	// Fetch invoice details from BTCPay API to get orderId from metadata
	invoiceDetail, err := p.fetchInvoiceDetail(btcpayURL, storeID, apiKey, payload.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch invoice detail: %w", err)
	}

	var invoice struct {
		Metadata struct {
			OrderID string `json:"orderId"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(invoiceDetail, &invoice); err != nil {
		return nil, fmt.Errorf("failed to parse invoice detail: %w", err)
	}

	return &plugin.PaymentNotification{
		TradeNo:        invoice.Metadata.OrderID,
		GatewayTradeNo: payload.InvoiceID,
		Status:         "success",
		RawData:        notifyData,
	}, nil
}

// fetchInvoiceDetail retrieves invoice details from BTCPay Server API
func (p *BTCPay) fetchInvoiceDetail(btcpayURL, storeID, apiKey, invoiceID string) ([]byte, error) {
	apiEndpoint := fmt.Sprintf("%sapi/v1/stores/%s/invoices/%s",
		ensureTrailingSlash(btcpayURL), storeID, invoiceID)

	req, err := http.NewRequest("GET", apiEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "token "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ensureTrailingSlash ensures URL ends with /
func ensureTrailingSlash(u string) string {
	if len(u) > 0 && u[len(u)-1] != '/' {
		return u + "/"
	}
	return u
}
