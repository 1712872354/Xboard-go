package payment

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/xboard/xboard/internal/plugin"
)

// AlipayF2F implements Alipay Face-to-Face payment (扫码支付)
type AlipayF2F struct{}

func (p *AlipayF2F) Code() string        { return "alipay_f2f" }
func (p *AlipayF2F) Name() string        { return "支付宝当面付" }
func (p *AlipayF2F) Version() string     { return "2.0.0" }
func (p *AlipayF2F) Description() string { return "支付宝当面付 - 扫码支付" }
func (p *AlipayF2F) Author() string      { return "Xboard Team" }
func (p *AlipayF2F) Type() plugin.PluginType {
	return plugin.TypePayment
}
func (p *AlipayF2F) Boot(ctx *plugin.Context) error { return nil }
func (p *AlipayF2F) Install() error                 { return nil }
func (p *AlipayF2F) Uninstall() error               { return nil }
func (p *AlipayF2F) Update(old, new string) error   { return nil }

func (p *AlipayF2F) Form() []plugin.FormField {
	return []plugin.FormField{
		{Key: "app_id", Type: "text", Label: "支付宝APPID", Required: true, Description: "支付宝开放平台应用的APPID"},
		{Key: "private_key", Type: "text", Label: "应用私钥", Required: true, Description: "应用私钥，用于签名"},
		{Key: "public_key", Type: "text", Label: "支付宝公钥", Required: true, Description: "支付宝公钥，用于验签"},
		{Key: "product_name", Type: "text", Label: "自定义商品名称", Description: "将体现在支付宝账单中"},
	}
}

func (p *AlipayF2F) Pay(order *plugin.PaymentOrder) (*plugin.PaymentResult, error) {
	cfg := getPluginSettings()

	appID := getString(cfg, "app_id")
	privateKey := getString(cfg, "private_key")
	productName := getString(cfg, "product_name")

	if appID == "" || privateKey == "" {
		return nil, fmt.Errorf("missing app_id or private_key config")
	}

	subject := productName
	if subject == "" {
		subject = "XBoard - 订阅"
	}

	// Build biz_content
	bizContent := map[string]interface{}{
		"subject":      subject,
		"out_trade_no": order.TradeNo,
		"total_amount": fmt.Sprintf("%.2f", order.TotalAmount/100), // cents to yuan
	}
	bizJSON, _ := json.Marshal(bizContent)

	// Build request params
	params := map[string]string{
		"app_id":        appID,
		"method":        "alipay.trade.precreate",
		"charset":       "UTF-8",
		"sign_type":     "RSA2",
		"timestamp":     time.Now().Format("2006-01-02 15:04:05"),
		"version":       "1.0",
		"biz_content":   string(bizJSON),
		"_input_charset": "UTF-8",
	}
	if order.NotifyURL != "" {
		params["notify_url"] = order.NotifyURL
	}

	// Sign with RSA2
	signStr := alipayBuildSignStr(params)
	signature, err := rsaSignSHA256(signStr, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}
	params["sign"] = signature

	// Call Alipay gateway
	queryParams := make(url.Values)
	for k, v := range params {
		queryParams.Set(k, v)
	}
	gatewayURL := "https://openapi.alipay.com/gateway.do?" + queryParams.Encode()

	resp, err := http.Get(gatewayURL)
	if err != nil {
		return nil, fmt.Errorf("failed to call alipay gateway: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var alipayResp map[string]json.RawMessage
	if err := json.Unmarshal(body, &alipayResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	responseKey := "alipay_trade_precreate_response"
	responseData, ok := alipayResp[responseKey]
	if !ok {
		return nil, fmt.Errorf("invalid alipay response: %s", string(body))
	}

	var respData struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		SubCode string `json:"sub_code"`
		SubMsg  string `json:"sub_msg"`
		QRCode  string `json:"qr_code"`
	}
	if err := json.Unmarshal(responseData, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response data: %w", err)
	}

	if respData.Code != "10000" {
		return nil, fmt.Errorf("alipay error: %s %s", respData.SubCode, respData.SubMsg)
	}

	return &plugin.PaymentResult{
		Type:    "qrcode",
		QRCode:  respData.QRCode,
		TradeNo: order.TradeNo,
	}, nil
}

func (p *AlipayF2F) Notify(req interface{}) (*plugin.PaymentNotification, error) {
	cfg := getPluginSettings()
	alipayPublicKey := getString(cfg, "public_key")

	params, ok := req.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid request type")
	}

	// Check trade status
	tradeStatus := getStringMap(params, "trade_status")
	if tradeStatus != "TRADE_SUCCESS" {
		return &plugin.PaymentNotification{Status: "failed"}, nil
	}

	// Verify signature
	sign := getStringMap(params, "sign")
	if sign == "" {
		return nil, fmt.Errorf("missing signature")
	}

	// Build verification string (exclude sign and sign_type)
	signParams := make(map[string]string)
	for k, v := range params {
		if k != "sign" && k != "sign_type" {
			if sv, ok2 := v.(string); ok2 && sv != "" {
				signParams[k] = sv
			}
		}
	}

	signStr := alipayBuildSignStr(signParams)
	valid, err := rsaVerifySHA256(signStr, sign, alipayPublicKey)
	if err != nil || !valid {
		return nil, fmt.Errorf("signature verification failed")
	}

	return &plugin.PaymentNotification{
		TradeNo:        getStringMap(params, "out_trade_no"),
		GatewayTradeNo: getStringMap(params, "trade_no"),
		Amount:         getFloat64Map(params, "total_amount") * 100, // yuan to cents
		Status:         "success",
		RawData:        params,
	}, nil
}

// ---------------------------------------------------------------------------
// Alipay RSA2 helpers
// ---------------------------------------------------------------------------

// alipayBuildSignStr builds sorted key=value string for Alipay signature
func alipayBuildSignStr(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var pairs []string
	for _, k := range keys {
		v := params[k]
		if v != "" {
			pairs = append(pairs, k+"="+v)
		}
	}
	return strings.Join(pairs, "&")
}

// rsaSignSHA256 signs data with RSA-SHA256 (Alipay RSA2)
func rsaSignSHA256(data, privateKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		privateKeyPEM = formatPEMKey(privateKeyPEM, "RSA PRIVATE KEY")
		block, _ = pem.Decode([]byte(privateKeyPEM))
	}
	if block == nil {
		return "", fmt.Errorf("invalid private key format")
	}

	var priv *rsa.PrivateKey
	var err error

	priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		var key interface{}
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		priv, ok = key.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("not an RSA private key")
		}
	}

	h := sha256.New()
	h.Write([]byte(data))
	signature, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, h.Sum(nil))
	if err != nil {
		return "", fmt.Errorf("failed to sign: %w", err)
	}

	return base64.StdEncoding.EncodeToString(signature), nil
}

// rsaVerifySHA256 verifies RSA-SHA256 signature (Alipay RSA2)
func rsaVerifySHA256(data, signature, publicKeyPEM string) (bool, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		publicKeyPEM = formatPEMKey(publicKeyPEM, "PUBLIC KEY")
		block, _ = pem.Decode([]byte(publicKeyPEM))
	}
	if block == nil {
		return false, fmt.Errorf("invalid public key format")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return false, fmt.Errorf("not an RSA public key")
	}

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	h := sha256.New()
	h.Write([]byte(data))
	err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, h.Sum(nil), sigBytes)
	if err != nil {
		return false, nil // verification failed, not an error
	}
	return true, nil
}

// formatPEMKey wraps a raw base64 key string in PEM format
func formatPEMKey(key, keyType string) string {
	// Remove any existing PEM headers
	key = strings.ReplaceAll(key, "-----BEGIN "+keyType+"-----", "")
	key = strings.ReplaceAll(key, "-----END "+keyType+"-----", "")
	key = strings.ReplaceAll(key, "\n", "")
	key = strings.ReplaceAll(key, "\r", "")
	key = strings.TrimSpace(key)

	var lines []string
	lineLen := 64
	for i := 0; i < len(key); i += lineLen {
		end := i + lineLen
		if end > len(key) {
			end = len(key)
		}
		lines = append(lines, key[i:end])
	}
	return "-----BEGIN " + keyType + "-----\n" +
		strings.Join(lines, "\n") +
		"\n-----END " + keyType + "-----"
}
