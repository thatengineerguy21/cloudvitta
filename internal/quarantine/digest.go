package quarantine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

// DigestEntry represents an aggregated unmapped taxonomy item with closest candidate suggestion.
type DigestEntry struct {
	Provider           string    `json:"provider"`
	Category           string    `json:"category"`
	Kind               string    `json:"kind"` // "region", "product", "storage_class", "transfer_type"
	RawValue           string    `json:"raw_value"`
	Count              int       `json:"count"`
	FirstSeenAt        time.Time `json:"first_seen_at"`
	LastSeenAt         time.Time `json:"last_seen_at"`
	ClosestKey         string    `json:"closest_key"`
	SuggestedCanonical string    `json:"suggested_canonical"`
	Distance           int       `json:"distance"`
	TargetFile         string    `json:"target_file"`
}

// DigestReport aggregates all unmapped items into structured proposals.
type DigestReport struct {
	GeneratedAt time.Time     `json:"generated_at"`
	TotalItems  int           `json:"total_items"`
	UniqueCount int           `json:"unique_count"`
	Entries     []DigestEntry `json:"entries"`
}

// GenerateDigest processes a slice of UnmappedItem records and returns an aggregated DigestReport.
func GenerateDigest(items []UnmappedItem) *DigestReport {
	type key struct {
		provider string
		kind     string
		rawValue string
	}

	groups := make(map[key]*DigestEntry)

	for _, item := range items {
		k := key{
			provider: strings.ToLower(item.Provider),
			kind:     strings.ToLower(item.Kind),
			rawValue: item.RawValue,
		}

		entry, ok := groups[k]
		if !ok {
			entry = &DigestEntry{
				Provider:    k.provider,
				Category:    item.Category,
				Kind:        k.kind,
				RawValue:    k.rawValue,
				Count:       0,
				FirstSeenAt: item.ObservedAt,
				LastSeenAt:  item.ObservedAt,
				TargetFile:  resolveTargetFile(k.provider, k.kind),
			}
			groups[k] = entry
		}

		entry.Count++
		if item.ObservedAt.Before(entry.FirstSeenAt) {
			entry.FirstSeenAt = item.ObservedAt
		}
		if item.ObservedAt.After(entry.LastSeenAt) {
			entry.LastSeenAt = item.ObservedAt
		}
	}

	entries := make([]DigestEntry, 0, len(groups))
	for _, entry := range groups {
		closestKey, canonical, dist := findClosestMatch(entry.Provider, entry.Kind, entry.RawValue)
		entry.ClosestKey = closestKey
		entry.SuggestedCanonical = canonical
		entry.Distance = dist
		entries = append(entries, *entry)
	}

	// Sort entries by Count descending, then Provider, then Kind, then RawValue
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		if entries[i].Provider != entries[j].Provider {
			return entries[i].Provider < entries[j].Provider
		}
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].RawValue < entries[j].RawValue
	})

	return &DigestReport{
		GeneratedAt: time.Now().UTC(),
		TotalItems:  len(items),
		UniqueCount: len(entries),
		Entries:     entries,
	}
}

// GenerateDigestFromStorage scans all jsonl files in storage under prefix (e.g. "quarantine/") and produces a DigestReport.
func GenerateDigestFromStorage(ctx context.Context, st storage.RawStorage, prefix string) (*DigestReport, error) {
	if prefix == "" {
		prefix = "quarantine/"
	}

	files, err := st.ListObjects(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list quarantine objects: %w", err)
	}

	var allItems []UnmappedItem
	for _, filePath := range files {
		if !strings.HasSuffix(filePath, ".jsonl") {
			continue
		}

		rc, err := st.ReadStream(ctx, filePath)
		if err != nil {
			return nil, fmt.Errorf("read quarantine file %s: %w", filePath, err)
		}

		scanner := bufio.NewScanner(rc)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var item UnmappedItem
			if err := json.Unmarshal(line, &item); err == nil {
				allItems = append(allItems, item)
			}
		}
		_ = rc.Close()
	}

	return GenerateDigest(allItems), nil
}

// Markdown formats the digest report as human-readable Markdown with direct links to target map files.
func (r *DigestReport) Markdown() string {
	var sb strings.Builder
	sb.WriteString("# Quarantine Digest Report\n\n")
	fmt.Fprintf(&sb, "**Generated At:** %s  \n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&sb, "**Total Unmapped Items Quarantined:** %d  \n", r.TotalItems)
	fmt.Fprintf(&sb, "**Unique Unmapped Taxonomies:** %d\n\n", r.UniqueCount)

	if len(r.Entries) == 0 {
		sb.WriteString("No unmapped items found in quarantine.\n")
		return sb.String()
	}

	sb.WriteString("| Provider | Kind | Unmapped Raw Value | Count | Suggested Canonical | Closest Match (Distance) | Target File |\n")
	sb.WriteString("|---|---|---|---|---|---|---|\n")

	for _, e := range r.Entries {
		closestInfo := fmt.Sprintf("`%s` (%d)", e.ClosestKey, e.Distance)
		if e.ClosestKey == "" {
			closestInfo = "None"
		}
		suggested := fmt.Sprintf("`%s`", e.SuggestedCanonical)
		if e.SuggestedCanonical == "" {
			suggested = "*(manual review)*"
		}

		fmt.Fprintf(&sb, "| `%s` | `%s` | `%s` | %d | %s | %s | [`%s`](file:///%s) |\n",
			e.Provider, e.Kind, e.RawValue, e.Count, suggested, closestInfo, e.TargetFile, e.TargetFile)
	}

	sb.WriteString("\n## Ready-To-Paste Go Snippets\n\n")
	sb.WriteString("```go\n")
	sb.WriteString(r.GoSnippets())
	sb.WriteString("```\n")

	return sb.String()
}

// GoSnippets outputs ready-to-paste Go map entry snippets grouped by target file.
func (r *DigestReport) GoSnippets() string {
	var sb strings.Builder

	// Group by target file
	byFile := make(map[string][]DigestEntry)
	for _, e := range r.Entries {
		byFile[e.TargetFile] = append(byFile[e.TargetFile], e)
	}

	var files []string
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)

	for _, file := range files {
		fmt.Fprintf(&sb, "// --- %s ---\n", file)
		for _, e := range byFile[file] {
			val := e.SuggestedCanonical
			if val == "" {
				val = "TODO"
			}
			if e.ClosestKey != "" {
				fmt.Fprintf(&sb, "\t%q: %q, // closest: %q (dist=%d, count=%d)\n", e.RawValue, val, e.ClosestKey, e.Distance, e.Count)
			} else {
				fmt.Fprintf(&sb, "\t%q: %q, // count=%d\n", e.RawValue, val, e.Count)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func resolveTargetFile(provider, kind string) string {
	switch kind {
	case "region":
		return fmt.Sprintf("internal/matching/regionmap/%s.go", provider)
	case "product":
		return fmt.Sprintf("internal/matching/catalogmap/%s.go", provider)
	case "storage_class":
		return fmt.Sprintf("internal/matching/storageclassmap/%s.go", provider)
	case "transfer_type":
		return fmt.Sprintf("internal/matching/transfertypemap/%s.go", provider)
	default:
		return fmt.Sprintf("internal/matching/%s/%s.go", kind, provider)
	}
}

func findClosestMatch(provider, kind, rawValue string) (closestKey string, canonical string, minDistance int) {
	var candidates map[string]string

	switch kind {
	case "region":
		switch provider {
		case "aws":
			candidates = regionmap.KnownAWSRegions()
		case "azure":
			candidates = regionmap.KnownAzureRegions()
		case "gcp":
			candidates = regionmap.KnownGCPRegions()
		}
	case "product":
		switch provider {
		case "aws":
			candidates = catalogmap.KnownAWSProducts()
		case "azure":
			candidates = catalogmap.KnownAzureProducts()
		case "gcp":
			candidates = catalogmap.KnownGCPProducts()
		}
	case "storage_class":
		switch provider {
		case "aws":
			candidates = storageclassmap.KnownAWSStorageClasses()
		case "azure":
			candidates = storageclassmap.KnownAzureStorageClasses()
		case "gcp":
			candidates = storageclassmap.KnownGCPStorageClasses()
		}
	case "transfer_type":
		switch provider {
		case "aws":
			candidates = transfertypemap.KnownAWSTransferTypes()
		case "azure":
			candidates = transfertypemap.KnownAzureTransferTypes()
		case "gcp":
			candidates = transfertypemap.KnownGCPTransferTypes()
		}
	}

	if len(candidates) == 0 {
		return "", "", 999
	}

	minDist := 999
	bestKey := ""
	bestCanonical := ""

	rawLower := strings.ToLower(rawValue)

	for k, v := range candidates {
		dist := LevenshteinDistance(rawLower, strings.ToLower(k))
		if dist < minDist {
			minDist = dist
			bestKey = k
			bestCanonical = v
		}
	}

	return bestKey, bestCanonical, minDist
}

// LevenshteinDistance computes the edit distance between two strings.
func LevenshteinDistance(s1, s2 string) int {
	r1, r2 := []rune(s1), []rune(s2)
	n, m := len(r1), len(r2)
	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
		dp[i][0] = i
	}
	for j := 0; j <= m; j++ {
		dp[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			dp[i][j] = min(
				dp[i-1][j]+1,      // deletion
				dp[i][j-1]+1,      // insertion
				dp[i-1][j-1]+cost, // substitution
			)
		}
	}

	return dp[n][m]
}

func min(vals ...int) int {
	m := vals[0]
	for _, v := range vals[1:] {
		if v < m {
			m = v
		}
	}
	return m
}
