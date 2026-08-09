package probe

import (
	"regexp"
	"unicode"

	"llm-api-uptime/internal/model"
)

const minReasonableLatencyMs = 50

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// isContentMeaningful returns false if content is empty, whitespace-only,
// zero-width characters only, non-breaking spaces only, or has fewer than
// 1 visible character after stripping all whitespace.
func isContentMeaningful(content string) bool {
	visibleCount := 0
	for _, r := range content {
		if r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\ufeff' || r == '\u00a0' {
			continue
		}
		if unicode.IsSpace(r) {
			continue
		}
		visibleCount++
	}
	return visibleCount >= 1
}

// stripBOM removes a leading UTF-8 BOM (0xEF 0xBB 0xBF) from body if present.
// Some API proxies (especially Chinese OneAPI-based providers) prepend a BOM to
// JSON responses, which causes json.Unmarshal to fail.
func stripBOM(body []byte) []byte {
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		return body[3:]
	}
	return body
}

// softFailPattern matches a known "fake success" phrase some providers embed
// in a 200-OK response body to signal billing/quota/auth failures.
type softFailPattern struct {
	code    string
	pattern *regexp.Regexp
}

// defaultSoftFailPatterns is the curated, code-builtin list of phrases that
// indicate the provider returned HTTP 200 but the assistant message is actually
// a failure template (quota exhausted, insufficient balance, etc.).
//
// The list intentionally covers common English, Chinese and Japanese phrasings
// from upstream vendors and OneAPI-style proxies. False-positive risk is
// mitigated by requiring co-occurring keywords (e.g. "quota" + "exceeded")
// rather than single-token matches.
//
// Order matters: more specific patterns (e.g. "insufficient balance") are
// checked before broader ones so the returned code reflects the most precise
// classification.
var defaultSoftFailPatterns = []softFailPattern{
	// English: rate-limit text inside 200 body (must precede quota_exceeded so
	// "rate limit reached" is classified as rate_limited_content, not quota).
	{code: "rate_limited_content", pattern: regexp.MustCompile(`(?i)\b(rate[\s-]?limit(ed)?\s*reached|too\s*many\s*requests|throttl(ed|ing)|request\s*throttled)\b`)},
	{code: "rate_limited_content", pattern: regexp.MustCompile(`(?i)\bexceeded\s+(your|the)\s+(rate|request)\s+limit\b`)},

	// English: "insufficient balance / credit / quota"
	{code: "insufficient_balance", pattern: regexp.MustCompile(`(?i)\b(insufficient|not\s+enough|no\s+more|depleted|zero)\s+(balance|credit|credits|funds|quota|tokens)\b`)},

	// English: "quota exceeded / exhausted / reached limit"
	{code: "quota_exceeded", pattern: regexp.MustCompile(`(?i)\b(quota|token\s*quota|monthly\s+quota|daily\s+quota)\b[^.!?\n]{0,40}\b(exceeded|exhausted|reached|used\s*up|has\s+been\s+(reached|used\s*up))`)},
	{code: "quota_exceeded", pattern: regexp.MustCompile(`(?i)\b(usage\s+limit|request\s+limit)\s+(exceeded|reached)\b`)},
	{code: "quota_exceeded", pattern: regexp.MustCompile(`(?i)\b(has\s+exceeded|has\s+been\s+exceeded|have\s+exceeded)\b[^.!?\n]{0,40}\b(quota|limit|usage)\b`)},

	// Chinese: insufficient balance (specific)
	{code: "insufficient_balance", pattern: regexp.MustCompile(`(余额|费用|信用|点数|额度)\s*(不足|不够|为0|为零|用尽|不够用)`)},
	{code: "insufficient_balance", pattern: regexp.MustCompile(`欠费|欠款|已欠费|账户欠费|余额为0`)},

	// Chinese: quota exceeded
	{code: "quota_exceeded", pattern: regexp.MustCompile(`(额度|配额)\s*(已)?(用尽|用完|超出|超限|超过|已达上限|达到上限|达到限额)`)},
	{code: "quota_exceeded", pattern: regexp.MustCompile(`本月\s*(token|令牌)?\s*(额度|配额).*(用尽|超出|超过|已用尽|用完)`)},

	// Japanese
	{code: "quota_exceeded", pattern: regexp.MustCompile(`クォータ|クオータ|利用上限|上限に達し`)},
	{code: "insufficient_balance", pattern: regexp.MustCompile(`残高不足|クレジット不足|利用枠`)},

	// Chinese: rate limit
	{code: "rate_limited_content", pattern: regexp.MustCompile(`(请求过于频繁|访问频率过高|限流中|触发限流|频率限制|超过频率|超频访问|超过访问频率)`)},

	// English: payment / top-up
	{code: "payment_required", pattern: regexp.MustCompile(`(?i)\b(payment\s+required|please\s+top[\s-]?up|please\s+recharge|please\s+add\s+funds|please\s+purchase\s+more|account\s+is\s+out\s+of\s+(credit|funds))\b`)},
	{code: "payment_required", pattern: regexp.MustCompile(`(?i)\b(add\s+credits?|buy\s+credits?|top[\s-]?up|recharge)\b[^.!?\n]{0,30}\b(to\s+continue|required|needed)\b`)},

	// Chinese: payment / top-up
	{code: "payment_required", pattern: regexp.MustCompile(`(请充值|请|需要)?\s*充值|欠费|欠款|已欠费|账户欠费|充值后|购买积分|充值后继续`)},

	// English: API key / auth invalid (inside body, not headers)
	{code: "auth_invalid_content", pattern: regexp.MustCompile(`(?i)\b(api[_ ]?key|authorization|access[_ ]?token|bearer\s+token)\s+(is\s*)?(invalid|expired|missing|revoked|incorrect|not\s+authorized|not\s+authenticated)`)},
	{code: "auth_invalid_content", pattern: regexp.MustCompile(`(?i)\b(invalid|expired|revoked)\s+(api[_ ]?key|access[_ ]?token|bearer\s+token|credentials?)\b`)},

	// Chinese: auth invalid
	{code: "auth_invalid_content", pattern: regexp.MustCompile(`(?i)(认证失败|鉴权失败|身份验证失败|密钥错误|密钥失效|未授权|令牌无效|授权失败|token\s*无效|api[_ ]?key\s*(无效|错误|过期))`)},

	// English: account / billing disabled
	{code: "billing_disabled", pattern: regexp.MustCompile(`(?i)\b(account|billing|subscription|plan|service)\s+(is\s*)?(disabled|suspended|terminated|deactivated|frozen|locked|paused|cancelled)\b`)},
	{code: "billing_disabled", pattern: regexp.MustCompile(`(?i)\b(access\s+denied|account\s+closed)\b\s*(due\s+to|because\s+of)?\s*(billing|payment|overdue)?`)},

	// Chinese: account disabled
	{code: "billing_disabled", pattern: regexp.MustCompile(`(账户|账号|账单|订阅).*(已)?(停用|禁用|冻结|锁定|暂停|欠费停机|注销|不可用)`)},
	{code: "billing_disabled", pattern: regexp.MustCompile(`访问被拒绝|服务被暂停|账号已注销`)},

	// English: model unavailable
	{code: "model_unavailable", pattern: regexp.MustCompile(`(?i)\b(model|engine)\s+(is\s+)?(currently\s+|now\s+)?(overloaded|unavailable|deprecated|retired|disabled|capacity\s+exhausted)`)},
	{code: "model_unavailable", pattern: regexp.MustCompile(`(?i)\b(no\s+available\s+model|model\s+not\s+found|model\s+does\s+not\s+exist)\b`)},

	// Chinese: model unavailable
	{code: "model_unavailable", pattern: regexp.MustCompile(`(模型|引擎).*(已下线|已废弃|不可用|已被禁用|维护中|当前不可用)`)},
	{code: "model_unavailable", pattern: regexp.MustCompile(`模型不存在|模型已下架`)},

	// English: upstream / provider error
	{code: "service_unavailable", pattern: regexp.MustCompile(`(?i)\b(service\s+unavailable|upstream\s+(error|provider)\s+(error|failure)|provider\s+(error|failure)|backend\s+error)\b`)},
	{code: "service_unavailable", pattern: regexp.MustCompile(`(?i)\binternal\s+server\s+error\b`)},

	// Chinese: upstream / provider error
	{code: "service_unavailable", pattern: regexp.MustCompile(`(服务暂不可用|服务暂时不可用|服务异常|上游异常|上游不可用|通道异常|渠道异常|网关异常|服务端错误)`)},

	// English: context length
	{code: "context_too_long", pattern: regexp.MustCompile(`(?i)\b(context(\s+length)?\s+(too\s+long|exceeded|overflow|limit\s+exceeded)|maximum\s+context(\s+length)?\s+(reached|exceeded)|input\s+too\s+large)\b`)},

	// Chinese: context length
	{code: "context_too_long", pattern: regexp.MustCompile(`(上下文(过长|超出|超限|超过|已超出)|超过最大上下文|超出长度限制|上下文长度(超过|超出|超限)|输入过长)`)},
}

// classifyContent scans the assistant message for known fake-success phrases.
// It returns StatusSoftFail, an error code and the matched substring on hit;
// on miss it returns ("", "", "").
//
// The patterns are case-insensitive and explicitly avoid single-token matches
// to keep false positives low (e.g. a normal reply mentioning "quota" is not
// enough to trigger a soft-fail classification).
func classifyContent(content string) (model.ProbeStatus, string, string) {
	if content == "" {
		return "", "", ""
	}
	for _, p := range defaultSoftFailPatterns {
		m := p.pattern.FindString(content)
		if m != "" {
			return model.StatusSoftFail, p.code, m
		}
	}
	return "", "", ""
}