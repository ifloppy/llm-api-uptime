package probe

import (
	"bytes"
	"testing"

	"llm-api-uptime/internal/model"
)

func TestStripBOM(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "nil input returns nil",
			input: nil,
			want:  nil,
		},
		{
			name:  "empty input returns empty",
			input: []byte{},
			want:  []byte{},
		},
		{
			name:  "short input no BOM",
			input: []byte{0xEF, 0xBB},
			want:  []byte{0xEF, 0xBB},
		},
		{
			name:  "BOM only returns empty",
			input: []byte{0xEF, 0xBB, 0xBF},
			want:  []byte{},
		},
		{
			name:  "BOM with JSON content",
			input: append([]byte{0xEF, 0xBB, 0xBF}, []byte(`{"ok":true}`)...),
			want:  []byte(`{"ok":true}`),
		},
		{
			name:  "no BOM passes through unchanged",
			input: []byte(`{"ok":true}`),
			want:  []byte(`{"ok":true}`),
		},
		{
			name:  "3-byte input that looks like BOM but is not",
			input: []byte{0xEF, 0xBB, 0xC0},
			want:  []byte{0xEF, 0xBB, 0xC0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripBOM(tt.input)
			if !bytes.Equal(got, tt.want) {
				t.Errorf("stripBOM(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestClassifyContent_Positive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode string
	}{
		{"english quota exceeded", "Your monthly token quota has been exceeded.", "quota_exceeded"},
		{"english insufficient balance", "Insufficient balance. Please recharge.", "insufficient_balance"},
		{"english no more quota", "You have no more quota for this model.", "insufficient_balance"},
		{"english payment required", "Payment required. Please top up your account.", "payment_required"},
		{"english rate limit reached", "Rate limit reached. Try again later.", "rate_limited_content"},
		{"english too many requests", "Too many requests, slow down.", "rate_limited_content"},
		{"english api key invalid", "API key invalid or expired.", "auth_invalid_content"},
		{"english account disabled", "Account is suspended due to billing issues.", "billing_disabled"},
		{"english model unavailable", "The model is currently overloaded.", "model_unavailable"},
		{"english context too long", "Context length exceeded maximum limit.", "context_too_long"},

		{"chinese quota exhausted", "您的本月 token 额度已用尽，请充值后继续使用。", "quota_exceeded"},
		{"chinese balance insufficient", "余额不足,请充值。", "insufficient_balance"},
		{"chinese rate limit", "请求过于频繁，请稍后再试。", "rate_limited_content"},
		{"chinese auth invalid", "API key 无效,请检查。", "auth_invalid_content"},
		{"chinese account frozen", "账户已被冻结,请联系客服。", "billing_disabled"},
		{"chinese model unavailable", "该模型当前不可用,请稍后再试。", "model_unavailable"},
		{"chinese upstream error", "上游通道异常,已切换备用节点。", "service_unavailable"},
		{"chinese context too long", "上下文长度超过最大限制。", "context_too_long"},

		{"japanese quota", "クォータを超えました。", "quota_exceeded"},

		{"mixed casing", "QUOTA EXCEEDED for account.", "quota_exceeded"},
		{"embedded in longer reply", "Sure! However, your monthly quota exceeded the plan limit. Please top up.", "quota_exceeded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, matched := classifyContent(tt.input)
			if status != model.StatusSoftFail {
				t.Errorf("classifyContent(%q) status = %q, want %q", tt.input, status, model.StatusSoftFail)
			}
			if code != tt.wantCode {
				t.Errorf("classifyContent(%q) code = %q, want %q (matched=%q)", tt.input, code, tt.wantCode, matched)
			}
			if matched == "" {
				t.Errorf("classifyContent(%q) returned empty match", tt.input)
			}
		})
	}
}

func TestClassifyContent_Negative(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"normal greeting", "Hi! How can I help you today?"},
		{"normal reply about quota concept", "Your quota for the free tier is 100k tokens per month."},
		{"normal usage explanation", "If you run out of credits, you can purchase more in the billing portal."},
		{"empty string", ""},
		{"only whitespace", "   \n\t  "},
		{"short technical reply", "Hello"},
		{"normal reply mentioning rate but not limit", "The library applies backoff automatically when its rate estimate is high."},
		{"normal reply mentioning context", "Context is preserved across turns in this conversation."},
		{"normal chinese reply", "你好,有什么可以帮助你的?"},
		{"normal chinese reply mentioning balance", "我已经把信息写入数据库,请查看。"},
		{"normal chinese reply mentioning model", "这是一个基于 Transformer 的语言模型。"},
		{"normal japanese reply", "こんにちは、本日为您服务。"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, matched := classifyContent(tt.input)
			if status != "" {
				t.Errorf("classifyContent(%q) status = %q, want empty (false positive)", tt.input, status)
			}
			if code != "" || matched != "" {
				t.Errorf("classifyContent(%q) code=%q matched=%q, want both empty", tt.input, code, matched)
			}
		})
	}
}

func TestClassifyContent_Empty(t *testing.T) {
	status, code, matched := classifyContent("")
	if status != "" || code != "" || matched != "" {
		t.Errorf("classifyContent(\"\") = (%q,%q,%q), want all empty", status, code, matched)
	}
}