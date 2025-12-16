#!/bin/zsh
# Zotero File Downloader - Install dependencies, build, and run

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Change to script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "${BLUE}========================================${NC}"
echo "${BLUE}Zotero File Downloader${NC}"
echo "${BLUE}========================================${NC}\n"

# ============================================
# 1. CHECK PREREQUISITES
# ============================================
echo "${BLUE}[1/5]${NC} Checking prerequisites..."

if ! command -v go &> /dev/null; then
  echo "${RED}❌ Go is not installed${NC}"
  exit 1
fi
GO_VERSION=$(go version | awk '{print $3}')
echo "${GREEN}✅ Go $GO_VERSION found${NC}"

# ============================================
# 2. LOAD/CREATE ENV FILE
# ============================================
echo "\n${BLUE}[2/5]${NC} Setting up environment..."

if [[ -f ".env" ]]; then
  set -a
  source .env
  set +a
  echo "${GREEN}✅ Loaded .env file${NC}"
else
  echo "${YELLOW}⚠️  No .env file found${NC}"
  echo "Creating template .env file..."
  cat > .env << 'EOF'
# Zotero API Configuration
ZOTERO_USER_ID=your_user_id_here
ZOTERO_API_KEY=your_api_key_here
ZOTERO_IS_GROUP=false
ZOTERO_LIMIT=100
ZOTERO_OUT_DIR=zotero_files
EOF
  echo "${RED}❌ Please edit .env with your credentials and run again${NC}"
  exit 1
fi

# Reload to get variables
set -a
source .env
set +a

# Validate required environment variables
REQUIRED_VARS=("ZOTERO_USER_ID" "ZOTERO_API_KEY")
for var in "${REQUIRED_VARS[@]}"; do
  if [[ -z "${(P)var}" ]]; then
    echo "${RED}❌ Missing required environment variable: $var${NC}"
    exit 1
  fi
done
echo "${GREEN}✅ Environment variables validated${NC}"

# ============================================
# 3. INSTALL DEPENDENCIES
# ============================================
echo "\n${BLUE}[3/5]${NC} Installing Go dependencies..."

if [[ -f "go.mod" ]]; then
  go mod tidy
  echo "${GREEN}✅ Dependencies installed${NC}"
else
  echo "${RED}❌ go.mod not found${NC}"
  exit 1
fi

# ============================================
# 4. BUILD
# ============================================
echo "\n${BLUE}[4/5]${NC} Building binary..."

BINARY_NAME="zotero-downloader"
if go build -o "$BINARY_NAME" zotero-file-downloader.go; then
  echo "${GREEN}✅ Build successful${NC}"
else
  echo "${RED}❌ Build failed${NC}"
  exit 1
fi

# ============================================
# 5. RUN
# ============================================
echo "\n${BLUE}[5/5]${NC} Running application..."
echo "${YELLOW}Downloading attachments to: ${ZOTERO_OUT_DIR:-zotero_files}${NC}\n"

# Export environment variables for the binary
export ZOTERO_USER_ID ZOTERO_API_KEY ZOTERO_IS_GROUP ZOTERO_LIMIT ZOTERO_OUT_DIR

if ./"$BINARY_NAME"; then
  echo "\n${GREEN}========================================${NC}"
  echo "${GREEN}✅ All done!${NC}"
  echo "${GREEN}========================================${NC}"
else
  echo "\n${RED}========================================${NC}"
  echo "${RED}❌ Download failed${NC}"
  echo "${RED}========================================${NC}"
  exit 1
fi
