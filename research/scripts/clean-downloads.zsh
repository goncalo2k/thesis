#!/bin/zsh
# Clear downloaded Zotero files

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Change to script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Load .env.example to get default output directory
OUTPUT_DIR="zotero_files"
if [[ -f ".env.example" ]]; then
  OUTPUT_DIR=$(grep "ZOTERO_OUT_DIR=" .env.example | cut -d'=' -f2 | tr -d ' ')
fi

if [[ ! -d "$OUTPUT_DIR" ]]; then
  echo "${YELLOW}⚠️  Directory $OUTPUT_DIR does not exist${NC}"
  exit 0
fi

# Count all files and folders (including subdirectories)
FILE_COUNT=$(find "$OUTPUT_DIR" -type f | wc -l)
DIR_COUNT=$(find "$OUTPUT_DIR" -mindepth 1 -type d | wc -l)
TOTAL_COUNT=$((FILE_COUNT + DIR_COUNT))

if [[ $TOTAL_COUNT -eq 0 ]]; then
  echo "${GREEN}✅ $OUTPUT_DIR is already empty${NC}"
  exit 0
fi

echo "${BLUE}Directory: $OUTPUT_DIR${NC}"
echo "${BLUE}Files to delete: $FILE_COUNT${NC}"
echo "${BLUE}Folders to delete: $DIR_COUNT${NC}"
echo "${BLUE}Total items: $TOTAL_COUNT${NC}"
echo ""
echo "${YELLOW}Are you sure? (y/N)${NC}"

read -q REPLY

if [[ $REPLY == "y" ]]; then
  # Remove all contents (files and folders) but keep the main directory
  rm -rf "$OUTPUT_DIR"/*
  echo ""
  echo "${GREEN}✅ Cleaned up $TOTAL_COUNT item(s) from $OUTPUT_DIR${NC}"
else
  echo ""
  echo "${YELLOW}Cancelled.${NC}"
fi
