package payment

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/xboard/xboard/internal/plugin"
)

// Epay implements 易支付 (EPay) third-party aggregated payment
type Epay struct{}

func (p *Epay) Code() string        { return "epay" }
func (p *Epay) Name() string        { return "易支付" }
func (p *Epay) Version() string     { return "2.0.0" }
func (p *Epay) Description() string { return "易支付 - 第三方聚合支付" }
func (p *Epay) Author() string      { return "Xboard Team" }
func (p *Epay) Type() plugin.PluginType {
	return plugin.TypePayment
}
func (p *Epay) Boot(ctx *plugin.Context) error { return nil }
func (p *Epay) Install() error                 { return nil }
func (p *Epay) Uninstall() error               { return nil }
func (p *Epay) Update(old, new string) error   { return nil }

func (p *Epay) Form() []plugin.FormField {
	return []plugin.FormField{
		{Key: "url", Type: "text", Label: "支付网关地址", Required: true, Description: "完整的支付网关地址，包括协议(http或https)"},
		{Key: "pid", Type: "text", Label: "商户ID", Required: true, Description: "商户ID"},
		{Key: "key", Type: "password", Label: "通信密钥", Required: true, Description: "通信密钥"},
		{Key: "type", Type: "text", Label: "支付类型", Description: "支付类型如: alipay, wxpay, qqpay 等"},
	}
}

func (p *Epay) Pay(order *plugin.PaymentOrder) (*plugin.PaymentResult, error) {
	cfg := getPluginSettings()

	epayURL := getString(cfg, "url")
	pid := getString(cfg, "pid")
	key := getString(cfg, "key")
	payType := getString(cfg, "type")

	if epayURL == "" || pid == "" || key == "" {
		return nil, fmt.Errorf("missing epay config (url, pid, key)")
	}

	// Build params exactly as PHP: ksort before signing
	params := url.Values{}
	params.Set("pid", pid)
	params.Set("money", fmt.Sprintf("%.2f", order.TotalAmount/100)) // cents to yuan
	params.Set("name", order.TradeNo)
	params.Set("notify_url", order.NotifyURL)
	params.Set("return_url", order.ReturnURL)
	params.Set("out_trade_no", order.TradeNo)

	if payType != "" {
		params.Set("type", payType)
	}

	// MD5 sign: ksort params, build query string, append key, md5
	signStr := epayBuildSignStrFromValues(params) + key
	sign := md5HashStr(signStr)
	params.Set("sign", sign)
	params.Set("sign_type", "MD5")

	// Build redirect URL
	redirectURL := strings.TrimRight(epayURL, "/") + "/submit.php?" + params.Encode()
	return &plugin.PaymentResult{
		Type:        "redirect",
		RedirectURL: redirectURL,
		TradeNo:     order.TradeNo,
	}, nil
}

func (p *Epay) Notify(req interface{}) (*plugin.PaymentNotification, error) {
	cfg := getPluginSettings()
	key := getString(cfg, "key")

	params, ok := req.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid request type")
	}

	// Extract and remove sign fields
	sign := getStringMap(params, "sign")
	signType := getStringMap(params, "sign_type")
	if signType != "MD5" {
		return nil, fmt.Errorf("invalid sign_type: %s", signType)
	}

	// Build verification params (exclude sign and sign_type)
	signParams := make(map[string]string)
	var keys []string
	for k, v := range params {
		if k != "sign" && k != "sign_type" {
			if sv, ok2 := v.(string); ok2 {
				signParams[k] = sv
				keys = append(keys, k)
			}
		}
	}

	// ksort + build query string + key, then md5
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		v := signParams[k]
		if v != "" {
			pairs = append(pairs, k+"="+v)
		}
	}
	expectedSign := md5HashStr(strings.Join(pairs, "&") + key)

	if sign != expectedSign {
		return nil, fmt.Errorf("signature verification failed")
	}

	return &plugin.PaymentNotification{
		TradeNo:        getStringMap(params, "out_trade_no"),
		GatewayTradeNo: getStringMap(params, "trade_no"),
		Amount:         getFloat64Map(params, "money") * 100, // yuan to cents
		Status:         "success",
		RawData:        params,
	}, nil
}

// epayBuildSignStrFromValues builds sorted query string from url.Values (for Pay)
func epayBuildSignStrFromValues(params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		v := params.Get(k)
		if v != "" && k != "sign" && k != "sign_type" {
			pairs = append(pairs, k+"="+v)
		}
	}
	return strings.Join(pairs, "&")
}

// md5HashStr computes MD5 and returns hex string
func md5HashStr(data string) string {
	h := md5.Sum([]byte(data))
	return hex.EncodeToString(h[:])
}


