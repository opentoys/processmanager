package notifier

import (
	"crypto/tls"
	"fmt"
	"net/mail"
	"net/smtp"
	"strings"
)

// MailSender SMTP 邮件发送器
type MailSender struct {
	to      string
	from    string
	smtpHost string
	smtpPort string
	username string
	password string
	useTLS  bool
}

// NewMailSender 创建邮件发送器
// smtp 格式: user:passwd@host:port
func NewMailSender(to, from, smtpAddr string) (*MailSender, error) {
	if to == "" || from == "" || smtpAddr == "" {
		return nil, fmt.Errorf("mail config incomplete: to=%s, from=%s, smtp=%s", to, from, smtpAddr)
	}

	user, passwd, host, port, err := parseSMTPAddr(smtpAddr)
	if err != nil {
		return nil, err
	}

	useTLS := true
	if port == "25" {
		useTLS = false
	}

	return &MailSender{
		to:       to,
		from:     from,
		smtpHost: host,
		smtpPort: port,
		username: user,
		password: passwd,
		useTLS:   useTLS,
	}, nil
}

// Send 发送文本邮件
// processName: 进程名, content: 日志内容
func (m *MailSender) Send(processName string, content string) error {
	subject := fmt.Sprintf("[PM告警] 进程 %s 日志告警", processName)
	body := fmt.Sprintf("进程: %s\n\n日志内容:\n%s", processName, content)

	// 解析 from 地址
	fromAddr, err := mail.ParseAddress(m.from)
	if err != nil {
		return fmt.Errorf("parse from address: %w", err)
	}

	addr := fmt.Sprintf("%s:%s", m.smtpHost, m.smtpPort)
	auth := smtp.PlainAuth("", m.username, m.password, m.smtpHost)

	if m.useTLS {
		return m.sendTLS(addr, auth, fromAddr.Address, m.to, subject, body)
	}
	return m.sendPlain(addr, auth, fromAddr.Address, m.to, subject, body)
}

// sendTLS 通过 TLS 发送
func (m *MailSender) sendTLS(addr string, auth smtp.Auth, from, to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	tlsConfig := &tls.Config{ServerName: m.smtpHost}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, m.smtpHost)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err = w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}

	return client.Quit()
}

// sendPlain 通过普通 SMTP 发送
func (m *MailSender) sendPlain(addr string, auth smtp.Auth, from, to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}

// parseSMTPAddr 解析 smtp 地址格式: user:passwd@host:port
func parseSMTPAddr(addr string) (user, passwd, host, port string, err error) {
	// 分离 user:passwd 和 host:port
	atIdx := strings.LastIndex(addr, "@")
	if atIdx < 0 {
		return "", "", "", "", fmt.Errorf("invalid smtp addr: missing @, got %s", addr)
	}

	userPasswd := addr[:atIdx]
	hostPort := addr[atIdx+1:]

	// 解析 user:passwd
	colonIdx := strings.Index(userPasswd, ":")
	if colonIdx < 0 {
		user = userPasswd
	} else {
		user = userPasswd[:colonIdx]
		passwd = userPasswd[colonIdx+1:]
	}

	// 解析 host:port
	colonIdx = strings.LastIndex(hostPort, ":")
	if colonIdx < 0 {
		host = hostPort
		port = "465"
	} else {
		host = hostPort[:colonIdx]
		port = hostPort[colonIdx+1:]
	}

	if host == "" {
		return "", "", "", "", fmt.Errorf("invalid smtp addr: empty host")
	}

	return user, passwd, host, port, nil
}
