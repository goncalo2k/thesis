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
	"time"

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
	ignoreTag        string
	downloadedFiles  map[string]bool
	processedEntries map[string]bool
)

// ZoteroItem represents an attachment/item returned by Zotero
type ZoteroItem struct {
	Key  string `json:"key"`
	Data struct {
		Filename    string      `json:"filename"`
		ContentType string      `json:"contentType"`
		Title       string      `json:"title"`
		ParentItem  string      `json:"parentItem"`
		Tags        []ZoteroTag `json:"tags"`
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

	ignoreTag = os.Getenv("IGNORE_TAG")

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

	log.Printf("Config: userID=%s isGroup=%t collectionKey=%s hasChildCollections=%t outDir=%s limit=%d ignoreTag=%s",
		userID, isGroup, collectionKey, hasChildColls, outDir, limit, ignoreTag)
}

func main() {
	initConfig()

	if hasChildColls {
		log.Printf("Fetching all nested collections of %s …", collectionKey)
		allCollections, err := fetchAllNestedCollections(collectionKey)
		if err != nil {
			log.Fatalf("Error fetching nested collections: %v", err)
		}
		if len(allCollections) == 0 {
			log.Printf("No collections found for %s, nothing to do.", collectionKey)
			return
		}

		for _, c := range allCollections {
			log.Printf("Processing collection %s (%s) -> %s", c.Data.Name, c.Key, c.FullPath)
			if err := processCollection(c.Key, c.Data.Name, c.FullPath); err != nil {
				log.Printf("Error processing collection %s: %v", c.Key, err)
			}
		}
	} else {
		log.Printf("Processing single collection %s …", collectionKey)
		if err := processCollection(collectionKey, "root", "root"); err != nil {
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

// fetchAllNestedCollections recursively gets all nested collections with hierarchy info
type CollectionWithPath struct {
	ZoteroCollection
	ParentName string
	FullPath   string
}

func fetchAllNestedCollections(rootKey string) ([]CollectionWithPath, error) {
	// Get parent collection first
	parentColl, err := fetchCollectionInfo(rootKey)
	if err != nil {
		return nil, err
	}

	var allCollections []CollectionWithPath
	if parentColl != nil {
		allCollections = append(allCollections, CollectionWithPath{
			ZoteroCollection: *parentColl,
			ParentName:       "",
			FullPath:         sanitizeFolderName(parentColl.Data.Name),
		})
	}

	// Get all nested collections with their paths
	nested, err := fetchNestedCollectionsWithPath(rootKey, "")
	if err != nil {
		return nil, err
	}

	allCollections = append(allCollections, nested...)
	return allCollections, nil
}

// fetchNestedCollectionsWithPath gets all nested collections with their full paths
func fetchNestedCollectionsWithPath(parentKey, parentPath string) ([]CollectionWithPath, error) {
	var collections []CollectionWithPath

	children, err := fetchChildCollections(parentKey)
	if err != nil {
		return nil, err
	}

	for _, child := range children {
		// Build full path
		var fullPath string
		if parentPath == "" {
			fullPath = sanitizeFolderName(child.Data.Name)
		} else {
			fullPath = filepath.Join(parentPath, sanitizeFolderName(child.Data.Name))
		}

		// Add this collection
		collections = append(collections, CollectionWithPath{
			ZoteroCollection: child,
			ParentName:       parentPath,
			FullPath:         fullPath,
		})

		// Recursively get nested children
		nested, err := fetchNestedCollectionsWithPath(child.Key, fullPath)
		if err != nil {
			log.Printf("Warning: could not fetch nested collections for %s: %v", child.Key, err)
			continue
		}

		collections = append(collections, nested...)
	}

	return collections, nil
}

// fetchCollectionInfo gets info for a single collection
func fetchCollectionInfo(collKey string) (*ZoteroCollection, error) {
	url := fmt.Sprintf("%s%s/collections/%s", baseURL, libraryPrefix(), collKey)
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

	var collection ZoteroCollection
	if err := json.NewDecoder(resp.Body).Decode(&collection); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &collection, nil
}

// processCollection lists attachment items in the collection and downloads PDF files
func processCollection(collKey, collName, fullPath string) error {
	// Create collection-specific directory using full path
	collectionDir := filepath.Join(outDir, fullPath)
	if err := os.MkdirAll(collectionDir, 0o755); err != nil {
		log.Printf("Failed to create collection directory %q: %v", collectionDir, err)
		return err
	}

	start := 0
	totalDownloaded := 0
	foundPDFs := false

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

			// Debug: log tags for first few items
			if len(item.Data.Tags) > 0 && totalDownloaded < 3 {
				log.Printf("[DEBUG] Item %s tags: %v", item.Key, item.Data.Tags)
			}

			// Skip item if it has the ignore tag
			ignored := shouldIgnoreItem(item)
			if ignored {
				log.Printf("[SKIP] Item %s has ignore tag '%s'", item.Key, ignoreTag)
				continue
			}

			// Log first few items to see what's happening
			if totalDownloaded < 3 {
				log.Printf("[DEBUG] Item %s: ignored=%v, parent=%s", item.Key, ignored, item.Data.ParentItem)
			}

			// Get parent entry key (use attachment key if no parent)
			parentKey := item.Data.ParentItem
			if parentKey == "" {
				parentKey = item.Key
			}

			// Skip if we already processed a PDF for this entry
			if isEntryProcessed(parentKey) {
				log.Printf("[SKIP] Entry %s already has PDF downloaded (attachment: %s)", parentKey, item.Key)
				continue
			}

			foundPDFs = true

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

			// Check if file already downloaded globally
			if isAlreadyDownloaded(filename) {
				log.Printf("[SKIP] %s already downloaded to another collection", filename)
				markEntryProcessed(parentKey) // Mark as processed to avoid checking this entry again
				continue
			}

			targetPath := filepath.Join(collectionDir, filename)
			if fileExists(targetPath) {
				log.Printf("[SKIP] %s (already exists)", targetPath)
				markAsDownloaded(filename)
				markEntryProcessed(parentKey)
				continue
			}

			log.Printf("[DL] Collection=%s File=%s (entry=%s, attachment=%s)", collName, filename, parentKey, item.Key)
			if err := downloadAttachment(item.Key, targetPath); err != nil {
				log.Printf("Error downloading %s: %v", item.Key, err)
				continue
			}
			markAsDownloaded(filename)
			markEntryProcessed(parentKey)
			totalDownloaded++
		}

		if len(items) < limit {
			break
		}
		start += limit
	}

	// Create README for empty collections
	if !foundPDFs {
		if err := createReadmeForEmptyCollection(collectionDir, collName); err != nil {
			log.Printf("Failed to create README for empty collection %s: %v", collName, err)
		} else {
			log.Printf("Created README for empty collection %s", collName)
		}
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

func sanitizeFolderName(name string) string {
	// Sanitize and truncate folder names for safe filesystem use
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

	sanitized := replacer.Replace(strings.TrimSpace(name))

	// Remove leading/trailing dots and spaces
	sanitized = strings.Trim(sanitized, " .")

	// Truncate to reasonable length (50 chars)
	if len(sanitized) > 50 {
		sanitized = sanitized[:50]
	}

	// Ensure name isn't empty after sanitization
	if sanitized == "" {
		sanitized = "unnamed_collection"
	}

	return sanitized
}

func isAlreadyDownloaded(filename string) bool {
	if downloadedFiles == nil {
		downloadedFiles = make(map[string]bool)
	}
	return downloadedFiles[filename]
}

func markAsDownloaded(filename string) {
	if downloadedFiles == nil {
		downloadedFiles = make(map[string]bool)
	}
	downloadedFiles[filename] = true
}

func isEntryProcessed(entryKey string) bool {
	if processedEntries == nil {
		processedEntries = make(map[string]bool)
	}
	return processedEntries[entryKey]
}

func markEntryProcessed(entryKey string) {
	if processedEntries == nil {
		processedEntries = make(map[string]bool)
	}
	processedEntries[entryKey] = true
}

func createReadmeForEmptyCollection(collectionDir, collName string) error {
	readmePath := filepath.Join(collectionDir, "README_NO_PDFS_FOUND.txt")
	content := fmt.Sprintf("Collection: %s\nCreated: %s\n\nNo PDF attachments found in this collection.\n", collName, time.Now().Format("2006-01-02 15:04:05"))

	return os.WriteFile(readmePath, []byte(content), 0644)
}

// ZoteroTag represents a tag structure in Zotero API
type ZoteroTag struct {
	Tag  string `json:"tag"`
	Type int    `json:"type"`
}

func shouldIgnoreItem(item ZoteroItem) bool {
	if ignoreTag == "" {
		return false
	}

	// First check attachment's own tags
	for _, tag := range item.Data.Tags {
		tagName := strings.TrimSpace(tag.Tag)
		if strings.EqualFold(tagName, strings.TrimSpace(ignoreTag)) {
			return true
		}
	}

	// Then check parent item tags if parent exists
	if item.Data.ParentItem != "" {
		parentTags, err := fetchItemTags(item.Data.ParentItem)
		if err != nil {
			log.Printf("Warning: could not fetch tags for parent item %s: %v", item.Data.ParentItem, err)
			return false
		}

		for _, tag := range parentTags {
			tagName := strings.TrimSpace(tag.Tag)
			if strings.EqualFold(tagName, strings.TrimSpace(ignoreTag)) {
				return true
			}
		}
	}

	return false
}

// fetchItemTags gets tags for a specific item
func fetchItemTags(itemKey string) ([]ZoteroTag, error) {
	url := fmt.Sprintf("%s%s/items/%s", baseURL, libraryPrefix(), itemKey)
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

	var item struct {
		Data struct {
			Tags []ZoteroTag `json:"tags"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return item.Data.Tags, nil
}
