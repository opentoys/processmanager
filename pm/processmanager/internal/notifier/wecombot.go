package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// WecombotSender 企微机器人发送器
type WecombotSender struct {
	key    string
	client *http.Client
}

// NewWecombotSender 创建企微机器人发送器
func NewWecombotSender(key string) *WecombotSender {
	return &WecombotSender{
		key: key,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send 发送 markdown 消息
// processName: 进程名, content: 日志内容
func (w *WecombotSender) Send(processName string, content string) error {
	if w.key == "" {
		return fmt.Errorf("wecombot key is empty")
	}

	// 截断过长内容，企微机器人消息体限制 2048 字节
	content = truncateText(content, 1500)

	// 构造 markdown_v2 消息
	md := fmt.Sprintf("## <font color=\"warning\">进程告警</font>\n"+
		"> 进程: <font color=\"info\">%s</font>\n\n"+
		"> %s\n",
		processName,
		content,
	)

	payload := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": md,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal wecombot payload: %w", err)
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=%s", w.key)
	resp, err := w.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wecombot post: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read wecombot response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wecombot response %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("unmarshal wecombot response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wecombot error %d: %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// truncateText 截断文本到指定长度，保持行完整性
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	// 按行截断，保持行完整
	lines := strings.Split(text, "\n")
	var buf strings.Builder
	for _, line := range lines {
		if buf.Len()+len(line)+1 > maxLen {
			buf.WriteString("\n...")
			break
		}
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(line)
	}
	return buf.String()
}
