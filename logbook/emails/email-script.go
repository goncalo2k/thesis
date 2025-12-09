package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
)

type Contact struct {
	Name    string
	Email   string
	Company string
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromName := os.Getenv("FROM_NAME")
	fromEmail := os.Getenv("FROM_EMAIL")
	subject := os.Getenv("EMAIL_SUBJECT")

	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" || fromEmail == "" {
		log.Fatal("Missing SMTP config in .env")
	}

	// Load email template (Markdown)
	templateBytes, err := os.ReadFile("./templates/email_template.md")
	if err != nil {
		log.Fatalf("Error reading email_template.md: %v", err)
	}
	template := string(templateBytes)

	// Load contacts
	contacts, err := loadContacts("contacts.csv")
	if err != nil {
		log.Fatalf("Error loading contacts: %v", err)
	}

	for _, c := range contacts {
		body := personalizeTemplate(template, c)

		if err := sendEmail(
			smtpHost,
			smtpPort,
			smtpUser,
			smtpPass,
			fromName,
			fromEmail,
			c.Email,
			subject,
			body,
		); err != nil {
			log.Printf("❌ Failed to send to %s (%s): %v", c.Name, c.Email, err)
		} else {
			log.Printf("✅ Sent to %s (%s)", c.Name, c.Email)
		}
	}
}

func loadContacts(path string) ([]Contact, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(bufio.NewReader(f))
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) <= 1 {
		return nil, fmt.Errorf("no contacts found in %s", path)
	}

	var contacts []Contact
	for i, r := range records {
		if i == 0 {
			// skip header
			continue
		}
		if len(r) < 3 {
			log.Printf("Skipping invalid row %d: %#v", i, r)
			continue
		}
		contacts = append(contacts, Contact{
			Name:    strings.TrimSpace(r[0]),
			Email:   strings.TrimSpace(r[1]),
			Company: strings.TrimSpace(r[2]),
		})
	}
	return contacts, nil
}

func personalizeTemplate(template string, c Contact) string {
	result := strings.ReplaceAll(template, "[Name]", c.Name)
	result = strings.ReplaceAll(result, "[Company]", c.Company)
	return result
}

func sendEmail(host, port, user, pass, fromName, fromEmail, toEmail, subject, body string) error {
	addr := host + ":" + port

	auth := smtp.PlainAuth("", user, pass, host)

	msg := buildRawMessage(fromName, fromEmail, toEmail, subject, body)

	return smtp.SendMail(addr, auth, fromEmail, []string{toEmail}, []byte(msg))
}

func markdownToHTML(markdown string) string {
	// Convert [text](url) to <a href="url">text</a>
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	return re.ReplaceAllString(markdown, `<a href="$2">$1</a>`)
}

func buildRawMessage(fromName, fromEmail, toEmail, subject, body string) string {
	htmlBody := markdownToHTML(body)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s <%s>\r\n", fromName, fromEmail))
	sb.WriteString(fmt.Sprintf("To: %s\r\n", toEmail))
	sb.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	sb.WriteString("\r\n")
	sb.WriteString("<html><body><pre style=\"font-family: sans-serif; white-space: pre-wrap;\">\r\n")
	sb.WriteString(htmlBody)
	sb.WriteString("\r\n</pre></body></html>")
	return sb.String()
}
