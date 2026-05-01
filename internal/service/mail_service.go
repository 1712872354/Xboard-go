package service

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/xboard/xboard/internal/model"
	"gorm.io/gorm"
)

// MailService handles email sending with SMTP
type MailService struct {
	host       string
	port       int
	username   string
	password   string
	encryption string
	fromAddr   string
	fromName   string
	db         *gorm.DB
}

func NewMailService(host string, port int, username, password, encryption, fromAddr, fromName string, db *gorm.DB) *MailService {
	return &MailService{
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		encryption: encryption,
		fromAddr:   fromAddr,
		fromName:   fromName,
		db:         db,
	}
}

// NewMailServiceFromDB creates a MailService from settings stored in database
func NewMailServiceFromDB(db *gorm.DB) *MailService {
	settings := NewSettingService(db)
	host, _ := settings.Get("mail_host")
	portStr, _ := settings.Get("mail_port")
	port := 587
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}
	username, _ := settings.Get("mail_username")
	password, _ := settings.Get("mail_password")
	encryption, _ := settings.Get("mail_encryption")
	fromAddr, _ := settings.Get("mail_from_address")
	fromName, _ := settings.Get("mail_from_name")
	return NewMailService(host, port, username, password, encryption, fromAddr, fromName, db)
}

// Send sends an email via SMTP with STARTTLS/SSL support
func (s *MailService) Send(to, subject, body string) error {
	fromName := s.fromName
	if fromName == "" {
		fromName = s.fromAddr
	}

	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		fromName, s.fromAddr, to, subject, body)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	var err error

	switch s.encryption {
	case "ssl":
		err = s.sendSSL(addr, auth, to, msg)
	default:
		err = s.sendSTARTTLS(addr, auth, to, msg)
	}

	// Log the result
	logEntry := &model.MailLog{
		Email:   to,
		Subject: subject,
		Status:  1,
	}
	if err != nil {
		logEntry.Status = 0
		logEntry.Error = err.Error()
	}
	s.db.Create(logEntry)

	return err
}

func (s *MailService) sendSSL(addr string, auth smtp.Auth, to, msg string) error {
	tlsConfig := &tls.Config{
		ServerName: s.host,
		InsecureSkipVerify: false,
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client failed: %w", err)
	}
	defer client.Quit()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}
	}

	if err := client.Mail(s.fromAddr); err != nil {
		return fmt.Errorf("mail from failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data failed: %w", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	return w.Close()
}

func (s *MailService) sendSTARTTLS(addr string, auth smtp.Auth, to, msg string) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer client.Quit()

	tlsConfig := &tls.Config{
		ServerName: s.host,
		InsecureSkipVerify: false,
	}

	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("starttls failed: %w", err)
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth failed: %w", err)
		}
	}

	if err := client.Mail(s.fromAddr); err != nil {
		return fmt.Errorf("mail from failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data failed: %w", err)
	}
	_, err = w.Write([]byte(msg))
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}
	return w.Close()
}

// SendWithTemplate loads a template, replaces params, and sends
func (s *MailService) SendWithTemplate(to, templateName string, params map[string]string) error {
	var tmpl model.MailTemplate
	if err := s.db.Where("name = ?", templateName).First(&tmpl).Error; err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	subject := tmpl.Subject
	body := tmpl.Template

	for k, v := range params {
		placeholder := "{{" + k + "}}"
		subject = strings.ReplaceAll(subject, placeholder, v)
		body = strings.ReplaceAll(body, placeholder, v)
	}

	return s.Send(to, subject, body)
}

// SendRemindMail sends expiry reminder emails to users whose plans expire soon
func (s *MailService) SendRemindMail() {
	var users []model.User
	now := time.Now()
	remindWindow := now.Add(72 * time.Hour)

	s.db.Where("expired_at BETWEEN ? AND ? AND plan_id > 0", now, remindWindow).Find(&users)

	for _, user := range users {
		daysLeft := int(user.ExpiredAt.Sub(now).Hours() / 24)
		if daysLeft < 0 {
			continue
		}

		params := map[string]string{
			"email":    user.Email,
			"days":     fmt.Sprintf("%d", daysLeft),
			"expired_at": user.ExpiredAt.Format("2006-01-02 15:04:05"),
		}

		if err := s.SendWithTemplate(user.Email, "expire_remind", params); err != nil {
			continue
		}
	}
}
