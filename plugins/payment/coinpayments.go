package payment

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/xboard/xboard/internal/plugin"
)

// CoinPayments implements CoinPayments.net payment gateway
type CoinPayments struct{}

func (p *CoinPayments) Code() string        { return "coinpayments" }
func (p *CoinPayments) Name() string        { return "CoinPayments" }
func (p *CoinPayments) Version() string     { return "2.0.0" }
func (p *CoinPayments) Description() string { return "CoinPayments.net - 加密货币支付" }
func (p *CoinPayments) Author() string      { return "Xboard Team" }
func (p *CoinPayments) Type() plugin.PluginType {
	return plugin.TypePayment
}
func (p *CoinPayments) Boot(ctx *plugin.Context) error { return nil }
func (p *CoinPayments) Install() error                 { return nil }
func (p *CoinPayments) Uninstall() error               { return nil }
func (p *CoinPayments) Update(old, new string) error   { return nil }

func (p *CoinPayments) Form() []plugin.FormField {
	return []plugin.FormField{
		{Key: "coinpayments_merchant_id", Type: "text", Label: "Merchant ID", Required: true, Description: "商户ID，Account Settings中获取"},
		{Key: "coinpayments_ipn_secret", Type: "password", Label: "IPN Secret", Required: true, Description: "通知密钥，Merchant Settings中设置的值"},
		{Key: "coinpayments_currency", Type: "text", Label: "货币代码", Required: true, Description: "货币代码（大写），建议与Merchant Settings中的值相同"},
	}
}

func (p *CoinPayments) Pay(order *plugin.PaymentOrder) (*plugin.PaymentResult, error) {
	cfg := getPluginSettings()

	merchantID := getString(cfg, "coinpayments_merchant_id")
	currency := getString(cfg, "coinpayments_currency")

	if merchantID == "" || currency == "" {
		return nil, fmt.Errorf("missing coinpayments config (merchant_id, currency)")
	}

	// Parse return_url to build success_url (scheme + host only, as PHP does)
	var successURL string
	if u, err := url.Parse(order.ReturnURL); err == nil {
		port := ""
		if u.Port() != "" && u.Port() != "80" && u.Port() != "443" {
			port = ":" + u.Port()
		}
		successURL = fmt.Sprintf("%s://%s%s", u.Scheme, u.Hostname(), port)
	}

	// Build params exactly as PHP
	params := url.Values{}
	params.Set("cmd", "_pay_simple")
	params.Set("reset", "1")
	params.Set("merchant", merchantID)
	params.Set("item_name", order.TradeNo)
	params.Set("item_number", order.TradeNo)
	params.Set("want_shipping", "0")
	params.Set("currency", currency)
	params.Set("amountf", fmt.Sprintf("%.2f", order.TotalAmount/100)) // cents to yuan
	params.Set("success_url", successURL)
	params.Set("cancel_url", order.ReturnURL)
	params.Set("ipn_url", order.NotifyURL)

	redirectURL := "https://www.coinpayments.net/index.php?" + params.Encode()
	return &plugin.PaymentResult{
		Type:        "redirect",
		RedirectURL: redirectURL,
		TradeNo:     order.TradeNo,
	}, nil
}

func (p *CoinPayments) Notify(req interface{}) (*plugin.PaymentNotification, error) {
	cfg := getPluginSettings()
	merchantID := getString(cfg, "coinpayments_merchant_id")
	ipnSecret := getString(cfg, "coinpayments_ipn_secret")

	params, ok := req.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid request type")
	}

	// Verify merchant ID
	if getStringMap(params, "merchant") != merchantID {
		return nil, fmt.Errorf("incorrect Merchant ID")
	}

	// Extract HMAC header
	hmacHeader, _ := params["_header_Hmac"].(string)
	if hmacHeader == "" {
		return nil, fmt.Errorf("missing HMAC header")
	}

	// Build request string: ksort params, http_build_query
	var keys []string
	signParams := make(map[string]string)
	for k, v := range params {
		if !startsWithUnderscore(k) {
			if sv, ok2 := v.(string); ok2 {
				signParams[k] = sv
				keys = append(keys, k)
			}
		}
	}

	// Sort keys and build query string
	sortStrings(keys)
	var pairs []string
	for _, k := range keys {
		v := signParams[k]
		pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	requestStr := joinPairs(pairs)

	// Compute HMAC-SHA512
	computedHMAC := hmacSHA512Hex(ipnSecret, requestStr)
	if !hmacEqual(hmacHeader, computedHMAC) {
		return nil, fmt.Errorf("HMAC signature does not match")
	}

	// Check payment status
	status := 0.0
	if s, ok := params["status"]; ok {
		if f, ok2 := s.(float64); ok2 {
			status = f
		}
	}

	// status >= 100 or status == 2 means confirmed
	if status < 0 {
		return nil, fmt.Errorf("payment timed out or error")
	}
	if status < 100 && status != 2 {
		// Pending, not yet confirmed
		return &plugin.PaymentNotification{
			TradeNo: getStringMap(params, "item_number"),
			Status:  "pending",
			RawData: params,
		}, nil
	}

	return &plugin.PaymentNotification{
		TradeNo:        getStringMap(params, "item_number"),
		GatewayTradeNo: getStringMap(params, "txn_id"),
		Status:         "success",
		RawData:        params,
	}, nil
}

// hmacSHA512Hex computes HMAC-SHA512 and returns hex-encoded string
func hmacSHA512Hex(key, data string) string {
	mac := hmac.New(sha512.New, []byte(key))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// startsWithUnderscore checks if a string starts with underscore
func startsWithUnderscore(s string) bool {
	return len(s) > 0 && s[0] == '_'
}

// sortStrings sorts a string slice in place
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// joinPairs joins URL-encoded pairs with &
func joinPairs(pairs []string) string {
	result := ""
	for i, p := range pairs {
		if i > 0 {
			result += "&"
		}
		result += p
	}
	return result
}
