package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type docsLocaleManifest struct {
	SchemaVersion     string   `json:"schema_version"`
	CanonicalLocale   string   `json:"canonical_locale"`
	Locales           []string `json:"locales"`
	TopLevelDocs      []string `json:"top_level_documents"`
	DocumentSets      []string `json:"document_sets"`
	CanonicalPolicy   string   `json:"canonical_policy"`
	TranslationPolicy string   `json:"translation_policy"`
}

func TestDocsLocalesMirrorCanonicalTree(t *testing.T) {
	docsDir := filepath.Join("..", "..", "docs")
	manifestData, err := os.ReadFile(filepath.Join(docsDir, "locales", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest docsLocaleManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SchemaVersion != "v1" || manifest.CanonicalLocale != "en" || len(manifest.Locales) == 0 {
		t.Fatalf("invalid locale manifest: %+v", manifest)
	}
	if manifest.CanonicalPolicy == "" || manifest.TranslationPolicy == "" {
		t.Fatalf("manifest must state canonical and translation policy: %+v", manifest)
	}
	canonical := markdownTree(t, docsDir, func(path string) bool {
		return !strings.HasPrefix(filepath.ToSlash(path), "locales/")
	})
	for _, locale := range manifest.Locales {
		localeDir := filepath.Join(docsDir, "locales", locale)
		files := markdownTree(t, localeDir, func(string) bool { return true })
		if diff := stringSetDiff(canonical, files); len(diff) > 0 {
			t.Fatalf("locale %s missing canonical docs: %v", locale, diff)
		}
		if diff := stringSetDiff(files, canonical); len(diff) > 0 {
			t.Fatalf("locale %s has non-canonical extra docs: %v", locale, diff)
		}
	}
}

func markdownTree(t *testing.T, root string, include func(string) bool) []string {
	t.Helper()
	files := make([]string, 0)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if filepath.Base(path) == "locales" && root == filepath.Join("..", "..", "docs") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if include(relative) {
			files = append(files, relative)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func stringSetDiff(left []string, right []string) []string {
	seen := make(map[string]struct{}, len(right))
	for _, value := range right {
		seen[value] = struct{}{}
	}
	diff := make([]string, 0)
	for _, value := range left {
		if _, ok := seen[value]; !ok {
			diff = append(diff, value)
		}
	}
	return diff
}
