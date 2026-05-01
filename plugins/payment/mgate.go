package payment

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/xboard/xboard/internal/plugin"
)

// MGate implements MGate payment gateway (USDT/TRC20)
type MGate struct{}

func (p *MGate) Code() string        { return "mgate" }
func (p *MGate) Name() string        { return "MGate支付" }
func (p *MGate) Version() string     { return "2.0.0" }
func (p *MGate) Description() string { return "MGate支付 - USDT-TRC20 加密货币支付" }
func (p *MGate) Author() string      { return "Xboard Team" }
func (p *MGate) Type() plugin.PluginType {
	return plugin.TypePayment
}
func (p *MGate) Boot(ctx *plugin.Context) error { return nil }
func (p *MGate) Install() error                 { return nil }
func (p *MGate) Uninstall() error               { return nil }
func (p *MGate) Update(old, new string) error   { return nil }

func (p *MGate) Form() []plugin.FormField {
	return []plugin.FormField{
		{Key: "mgate_url", Type: "text", Label: "API地址", Required: true, Description: "MGate支付网关API地址"},
		{Key: "mgate_app_id", Type: "text", Label: "APP ID", Required: true, Description: "MGate应用标识符"},
		{Key: "mgate_app_secret", Type: "password", Label: "App Secret", Required: true, Description: "MGate应用密钥"},
		{Key: "mgate_source_currency", Type: "text", Label: "源货币", Description: "默认CNY，源货币类型"},
	}
}

func (p *MGate) Pay(order *plugin.PaymentOrder) (*plugin.PaymentResult, error) {
	cfg := getPluginSettings()

	mgateURL := getString(cfg, "mgate_url")
	appID := getString(cfg, "mgate_app_id")
	appSecret := getString(cfg, "mgate_app_secret")
	sourceCurrency := getString(cfg, "mgate_source_currency")

	if mgateURL == "" || appID == "" || appSecret == "" {
		return nil, fmt.Errorf("missing mgate config (url, app_id, app_secret)")
	}

	// Build params exactly as PHP
	params := url.Values{}
	params.Set("out_trade_no", order.TradeNo)
	params.Set("total_amount", fmt.Sprintf("%.0f", order.TotalAmount)) // keep in cents
	params.Set("notify_url", order.NotifyURL)
	params.Set("return_url", order.ReturnURL)

	if sourceCurrency != "" {
		params.Set("source_currency", sourceCurrency)
	}

	params.Set("app_id", appID)

	// ksort + build query string + app_secret, then md5
	signStr := mgateBuildSignStr(params) + appSecret
	sign := md5HashStr(signStr)
	params.Set("sign", sign)

	// POST to MGate gateway
	resp, err := http.PostForm(strings.TrimRight(mgateURL, "/")+"/v1/gateway/fetch", params)
	if err != nil {
		return nil, fmt.Errorf("failed to call mgate gateway: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Errors  map[string][]string `json:"errors"`
		Data    struct {
			TradeNo string `json:"trade_no"`
			PayURL  string `json:"pay_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !result.Status {
		if len(result.Errors) > 0 {
			for _, errs := range result.Errors {
				if len(errs) > 0 {
					return nil, fmt.Errorf("mgate error: %s", errs[0])
				}
			}
		}
		if result.Message != "" {
			return nil, fmt.Errorf("mgate error: %s", result.Message)
		}
		return nil, fmt.Errorf("mgate unknown error")
	}

	if result.Data.PayURL == "" {
		return nil, fmt.Errorf("mgate: no pay_url in response")
	}

	return &plugin.PaymentResult{
		Type:        "redirect",
		RedirectURL: result.Data.PayURL,
		TradeNo:     order.TradeNo,
	}, nil
}

func (p *MGate) Notify(req interface{}) (*plugin.PaymentNotification, error) {
	cfg := getPluginSettings()
	appSecret := getString(cfg, "mgate_app_secret")

	params, ok := req.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid request type")
	}

	// Extract and remove sign
	sign := getStringMap(params, "sign")
	if sign == "" {
		return nil, fmt.Errorf("missing sign")
	}

	// Build verification params (exclude sign)
	signParams := make(map[string]string)
	var keys []string
	for k, v := range params {
		if k != "sign" {
			if sv, ok2 := v.(string); ok2 {
				signParams[k] = sv
				keys = append(keys, k)
			}
		}
	}

	// ksort + build query string + app_secret, then md5
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		v := signParams[k]
		if v != "" {
			pairs = append(pairs, k+"="+v)
		}
	}
	expectedSign := md5HashStr(strings.Join(pairs, "&") + appSecret)

	if sign != expectedSign {
		return nil, fmt.Errorf("signature verification failed")
	}

	return &plugin.PaymentNotification{
		TradeNo:        getStringMap(params, "out_trade_no"),
		GatewayTradeNo: getStringMap(params, "trade_no"),
		Amount:         getFloat64Map(params, "total_amount"),
		Status:         "success",
		RawData:        params,
	}, nil
}

// mgateBuildSignStr builds sorted query string from url.Values for MGate signature
func mgateBuildSignStr(params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		v := params.Get(k)
		if v != "" {
			pairs = append(pairs, k+"="+v)
		}
	}
	return strings.Join(pairs, "&")
}
