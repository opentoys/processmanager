package notifier

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"processmanager/internal/utils"
)

// Sender 消息发送接口
type Sender interface {
	Send(processName string, content string) error
}

// Notifier 通知调度器，集成到 LogWriter 中
type Notifier struct {
	mu      sync.RWMutex
	senders map[string]Sender     // channel name -> sender
	rules   map[string]*ruleEntry // compiled rules
}

// NewNotifier 创建通知调度器
func NewNotifier(cfg *utils.Config) *Notifier {
	n := &Notifier{}
	if cfg != nil {
		n.Reload(cfg)
	}
	return n
}

// Reload 重新加载配置
func (n *Notifier) Reload(cfg *utils.Config) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 解析并构建 senders
	senders := make(map[string]Sender, len(cfg.Channels))
	for name, ch := range cfg.Channels {
		s, err := buildSender(ch)
		if err != nil {
			slog.Error("Failed to build sender", "channel", name, "error", err)
			continue
		}
		senders[name] = s
	}

	// 解析并编译规则
	rawRules := make(map[string]utils.NoticeRule, len(cfg.Notice))
	for k, v := range cfg.Notice {
		rawRules[k] = utils.NoticeRule{
			Expr:    v.Expr,
			Channel: v.Channel,
		}
	}
	parsed := parseRules(rawRules)
	rules := compileRules(parsed)

	n.senders = senders
	n.rules = rules

	slog.Info("Notifier reloaded", "channels", len(senders), "rules", len(rules))
}

// Notify 处理日志行，匹配规则并发送通知
// processName: 进程名, line: 日志行文本
func (n *Notifier) Notify(processName string, line string) {
	if n == nil {
		return
	}

	n.mu.RLock()
	rules := n.rules
	senders := n.senders
	n.mu.RUnlock()

	if len(rules) == 0 || len(senders) == 0 {
		return
	}

	// 去除行尾空白
	text := strings.TrimRight(line, "\r\n")
	if text == "" {
		return
	}

	matched := matchLine(text, processName, rules)
	if len(matched) == 0 {
		return
	}

	// 收集需要发送的通道（去重）
	seen := make(map[string]struct{})
	for _, rule := range matched {
		for _, ch := range rule.channels {
			if _, ok := seen[ch]; ok {
				continue
			}
			seen[ch] = struct{}{}
			sender, exists := senders[ch]
			if !exists {
				slog.Warn("Notice channel not found", "channel", ch)
				continue
			}
			// 异步发送，避免阻塞日志写入
			go func(s Sender, proc, content string) {
				if err := s.Send(proc, content); err != nil {
					slog.Error("Failed to send notification", "process", proc, "channel", ch, "error", err)
				}
			}(sender, processName, text)
		}
	}
}

// buildSender 根据通道配置创建发送器
func buildSender(ch utils.ChanConfig) (Sender, error) {
	switch ch.Type {
	case "wecombot":
		if ch.Key == "" {
			return nil, fmt.Errorf("wecombot key is required")
		}
		return NewWecombotSender(ch.Key), nil
	case "mail":
		return NewMailSender(ch.To, ch.From, ch.SMTP)
	default:
		return nil, fmt.Errorf("unknown channel type: %s", ch.Type)
	}
}
