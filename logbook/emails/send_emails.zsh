#!/usr/bin/env bash
set -euo pipefail

# Optional: echo what we're doing
echo "==> Running Go email sender..."
echo "==> Make sure .env, contacts.csv and email_template.md are present."

# Initialize module if not already done
if [ ! -f "go.mod" ]; then
  echo "==> Initializing Go module..."
  go mod init email-script
fi

# Ensure dependency is present
go get github.com/joho/godotenv

# Run the program
go run email-script.go
