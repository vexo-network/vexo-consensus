package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type docsLocaleManifest struct {
	SchemaVersion     string            `json:"schema_version"`
	CanonicalLocale   string            `json:"canonical_locale"`
	Locales           []string          `json:"locales"`
	TopLevelDocs      []string          `json:"top_level_documents"`
	DocumentSets      []string          `json:"document_sets"`
	CanonicalPolicy   string            `json:"canonical_policy"`
	TranslationPolicy string            `json:"translation_policy"`
	CanonicalHashes   map[string]string `json:"canonical_hashes"`
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
	if len(manifest.CanonicalHashes) == 0 {
		t.Fatalf("manifest must bind canonical English docs by SHA-256: %+v", manifest)
	}
	canonical := markdownTree(t, docsDir, func(path string) bool {
		return !strings.HasPrefix(filepath.ToSlash(path), "locales/")
	})
	for _, relative := range canonical {
		data, err := os.ReadFile(filepath.Join(docsDir, relative))
		if err != nil {
			t.Fatal(err)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		if manifest.CanonicalHashes[relative] != hash {
			t.Fatalf("canonical document %s hash mismatch: manifest=%q actual=%q", relative, manifest.CanonicalHashes[relative], hash)
		}
	}
	if diff := stringSetDiff(canonical, sortedMapKeys(manifest.CanonicalHashes)); len(diff) > 0 {
		t.Fatalf("manifest missing canonical document hashes: %v", diff)
	}
	if diff := stringSetDiff(sortedMapKeys(manifest.CanonicalHashes), canonical); len(diff) > 0 {
		t.Fatalf("manifest has hashes for non-canonical documents: %v", diff)
	}
	canonicalLocaleFiles := readMarkdownFiles(t, filepath.Join(docsDir, "locales", manifest.CanonicalLocale))
	for _, locale := range manifest.Locales {
		localeDir := filepath.Join(docsDir, "locales", locale)
		files := markdownTree(t, localeDir, func(string) bool { return true })
		if diff := stringSetDiff(canonical, files); len(diff) > 0 {
			t.Fatalf("locale %s missing canonical docs: %v", locale, diff)
		}
		if diff := stringSetDiff(files, canonical); len(diff) > 0 {
			t.Fatalf("locale %s has non-canonical extra docs: %v", locale, diff)
		}
		localeFiles := readMarkdownFiles(t, localeDir)
		for relative, body := range localeFiles {
			if locale == manifest.CanonicalLocale {
				continue
			}
			if !strings.Contains(body, "Locale: "+locale) {
				t.Fatalf("locale %s document %s is missing locale marker", locale, relative)
			}
			if body == canonicalLocaleFiles[relative] {
				t.Fatalf("locale %s document %s is identical to canonical English", locale, relative)
			}
			if err := validateLocalizedMarkdownBody(locale, relative, body); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func validateLocalizedMarkdownBody(locale string, relative string, body string) error {
	if len(strings.TrimSpace(body)) < 1500 {
		return &docsLocaleQualityError{locale: locale, relative: relative, reason: "localized document is too short to be useful"}
	}
	for _, forbidden := range []string{"todo", "tbd", "placeholder", "coming soon", "translation pending", "machine translation pending"} {
		if placeholderPattern(forbidden).MatchString(body) {
			return &docsLocaleQualityError{locale: locale, relative: relative, reason: "localized document contains placeholder text: " + forbidden}
		}
	}
	if strings.Count(body, "\n## ") < 2 {
		return &docsLocaleQualityError{locale: locale, relative: relative, reason: "localized document must keep multiple explanatory sections"}
	}
	return nil
}

func placeholderPattern(value string) *regexp.Regexp {
	quoted := regexp.QuoteMeta(value)
	if strings.Contains(value, " ") {
		quoted = strings.ReplaceAll(quoted, `\ `, `\s+`)
	}
	return regexp.MustCompile(`(?i)(^|[^[:alpha:]])` + quoted + `($|[^[:alpha:]])`)
}

type docsLocaleQualityError struct {
	locale   string
	relative string
	reason   string
}

func (err *docsLocaleQualityError) Error() string {
	return "locale " + err.locale + " document " + err.relative + " failed quality gate: " + err.reason
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

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func readMarkdownFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(body)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
