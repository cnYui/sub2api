package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestEasyPayCreateAPIPaymentReturnsQRCodeImageURL(t *testing.T) {
	t.Parallel()

	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/mapi.php" {
			t.Errorf("path = %q, want /mapi.php", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}
		gotForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":1,"msg":"ok","trade_no":"zpay-trade-1","img":"https://zpayz.cn/qrcode/123.jpg"}`))
	}))
	defer server.Close()

	provider := newTestEasyPay(t, server.URL)
	resp, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:     "sub2-order-1",
		Amount:      "29.00",
		PaymentType: payment.TypeAlipay,
		Subject:     "Sub2API subscription",
		ClientIP:    "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("CreatePayment returned error: %v", err)
	}
	if resp.TradeNo != "zpay-trade-1" {
		t.Fatalf("trade_no = %q, want zpay-trade-1", resp.TradeNo)
	}
	if resp.QRImageURL != "https://zpayz.cn/qrcode/123.jpg" {
		t.Fatalf("qr image url = %q, want ZPay img URL", resp.QRImageURL)
	}
	if resp.QRCode != "" {
		t.Fatalf("qr_code = %q, want empty when ZPay only returns img", resp.QRCode)
	}
	for key, want := range map[string]string{
		"pid":          "pid-1",
		"type":         payment.TypeAlipay,
		"out_trade_no": "sub2-order-1",
		"notify_url":   "https://example.com/notify",
		"return_url":   "https://example.com/return",
		"name":         "Sub2API subscription",
		"money":        "29.00",
		"clientip":     "127.0.0.1",
		"sign_type":    signTypeMD5,
	} {
		if got := gotForm.Get(key); got != want {
			t.Fatalf("form[%s] = %q, want %q (form=%v)", key, got, want, gotForm)
		}
	}
	if gotForm.Get("sign") == "" {
		t.Fatal("form[sign] is empty")
	}
}
