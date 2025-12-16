package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	baseURL = "https://api.zotero.org"
)

var (
	userID           string
	apiKey           string
	isGroup          bool
	limit            int    = 100
	outDir           string = "zotero_files"
	zoteroAPIVersion string = "3"
	collectionKey    string
	hasChildColls    bool
)

// ZoteroItem represents an attachment/item returned by Zotero
type ZoteroItem struct {
	Key  string `json:"key"`
	Data struct {
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
		Title       string `json:"title"`
	} `json:"data"`
}

// ZoteroCollection represents a collection entry
type ZoteroCollection struct {
	Key  string `json:"key"`
	Data struct {
		Name string `json:"name"`
	} `json:"data"`
}

// build library prefix: /users/ID or /groups/ID
func libraryPrefix() string {
	if isGroup {
		return "/groups/" + userID
	}
	return "/users/" + userID
}

func parseBoolEnv(name string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	return v == "1" || v == "true" || v == "yes" || v == "y"
}

func initConfig() {
	// Load .env file if present
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found, using environment variables only")
	}

	userID = os.Getenv("ZOTERO_USER_ID")
	if userID == "" {
		log.Fatal("ZOTERO_USER_ID is required")
	}

	apiKey = os.Getenv("ZOTERO_API_KEY")
	if apiKey == "" {
		log.Fatal("ZOTERO_API_KEY is required")
	}

	collectionKey = os.Getenv("ZOTERO_COLLECTION_KEY")
	if collectionKey == "" {
		log.Fatal("ZOTERO_COLLECTION_KEY is required (parent collection key)")
	}

	isGroup = parseBoolEnv("ZOTERO_IS_GROUP")
	hasChildColls = parseBoolEnv("HAS_CHILD_COLLECTIONS")

	if v := os.Getenv("OUT_DIR"); v != "" {
		outDir = v
	}

	if v := os.Getenv("LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		} else {
			log.Printf("Invalid LIMIT value %q, using default %d", v, limit)
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		log.Fatalf("Failed to create output directory %q: %v", outDir, err)
	}

	log.Printf("Config: userID=%s isGroup=%t collectionKey=%s hasChildCollections=%t outDir=%s limit=%d",
		userID, isGroup, collectionKey, hasChildColls, outDir, limit)
}

func main() {
	initConfig()

	if hasChildColls {
		log.Printf("Fetching child collections of %s …", collectionKey)
		children, err := fetchChildCollections(collectionKey)
		if err != nil {
			log.Fatalf("Error fetching child collections: %v", err)
		}
		if len(children) == 0 {
			log.Printf("No child collections found for %s, nothing to do.", collectionKey)
			return
		}

		for _, c := range children {
			log.Printf("Processing child collection %s (%s)", c.Data.Name, c.Key)
			if err := processCollection(c.Key, c.Data.Name); err != nil {
				log.Printf("Error processing collection %s: %v", c.Key, err)
			}
		}
	} else {
		log.Printf("Processing single collection %s …", collectionKey)
		if err := processCollection(collectionKey, "root"); err != nil {
			log.Fatalf("Error processing collection %s: %v", collectionKey, err)
		}
	}

	log.Println("Done.")
}

// fetchChildCollections gets immediate child collections of a parent collection
func fetchChildCollections(parentKey string) ([]ZoteroCollection, error) {
	url := fmt.Sprintf("%s%s/collections/%s/collections?limit=%d", baseURL, libraryPrefix(), parentKey, limit)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Zotero-API-Key", apiKey)
	req.Header.Set("Zotero-API-Version", zoteroAPIVersion)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var collections []ZoteroCollection
	if err := json.NewDecoder(resp.Body).Decode(&collections); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return collections, nil
}

// processCollection lists attachment items in the collection and downloads PDF files
func processCollection(collKey, collName string) error {
	start := 0
	totalDownloaded := 0

	for {
		items, err := fetchAttachmentItems(collKey, start, limit)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			break
		}

		for _, item := range items {
			ct := strings.ToLower(strings.TrimSpace(item.Data.ContentType))
			if ct != "application/pdf" {
				continue
			}

			filename := item.Data.Filename
			if filename == "" {
				// Fallback: use title
				filename = item.Data.Title
			}
			if filename == "" {
				filename = item.Key
			}

			filename = sanitizeFilename(filename)
			if !strings.HasSuffix(strings.ToLower(filename), ".pdf") {
				filename += ".pdf"
			}

			targetPath := filepath.Join(outDir, filename)
			if fileExists(targetPath) {
				log.Printf("[SKIP] %s (already exists)", targetPath)
				continue
			}

			log.Printf("[DL] Collection=%s File=%s (key=%s)", collName, filename, item.Key)
			if err := downloadAttachment(item.Key, targetPath); err != nil {
				log.Printf("Error downloading %s: %v", item.Key, err)
				continue
			}
			totalDownloaded++
		}

		if len(items) < limit {
			break
		}
		start += limit
	}

	log.Printf("Collection %s (%s): downloaded %d PDFs", collName, collKey, totalDownloaded)
	return nil
}

// fetchAttachmentItems fetches attachment-type items from a collection with pagination
func fetchAttachmentItems(collKey string, start, limit int) ([]ZoteroItem, error) {
	url := fmt.Sprintf(
		"%s%s/collections/%s/items?itemType=attachment&format=json&include=data&start=%d&limit=%d",
		baseURL,
		libraryPrefix(),
		collKey,
		start,
		limit,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating items request: %w", err)
	}

	req.Header.Set("Zotero-API-Key", apiKey)
	req.Header.Set("Zotero-API-Version", zoteroAPIVersion)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("performing items request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("items request unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var items []ZoteroItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decoding items response: %w", err)
	}

	return items, nil
}

// downloadAttachment downloads the file for a given item key
func downloadAttachment(itemKey, targetPath string) error {
	url := fmt.Sprintf("%s%s/items/%s/file", baseURL, libraryPrefix(), itemKey)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating file request: %w", err)
	}

	req.Header.Set("Zotero-API-Key", apiKey)
	req.Header.Set("Zotero-API-Version", zoteroAPIVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("performing file request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("file request unexpected status %d: %s", resp.StatusCode, string(body))
	}

	outFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", targetPath, err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("writing file %s: %w", targetPath, err)
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func sanitizeFilename(name string) string {
	// Simple sanitization: remove path separators and some problematic chars
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(strings.TrimSpace(name))
}
