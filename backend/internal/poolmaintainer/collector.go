package poolmaintainer

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	htmlTagPattern    = regexp.MustCompile(`(?is)<[^>]+>`)
	rateValuePattern  = regexp.MustCompile(`(?i)(?:倍率\s*|[:=x×]\s*|\s+)(\d+(?:\.\d+)?)`)
	spacePattern      = regexp.MustCompile(`\s+`)
	loginMarkers      = []string{"login", "sign in", "登录", "登陆", "验证码"}
	errProfileMissing = errors.New("browser profile root is required")
)

func ExtractRatesFromHTML(upstream UpstreamConfig, raw string) CollectionResult {
	result := CollectionResult{
		UpstreamID: upstream.ID,
		Status:     CollectionStatusFailed,
	}

	text := normalizeHTMLText(raw)
	if text == "" {
		result.Error = "snapshot is empty"
		return result
	}

	aliases := aliasLookup(upstream.GroupNameAliases)
	type hit struct {
		group string
		rate  float64
		pos   int
	}
	var hits []hit
	warnings := make([]string, 0)
	seen := make(map[string]struct{}, len(aliases))

	for _, alias := range aliases {
		pos, rate, ok := findRateForAlias(text, alias.alias)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("group %q not found in snapshot", alias.group))
			continue
		}
		if _, exists := seen[alias.group]; exists {
			continue
		}
		seen[alias.group] = struct{}{}
		hits = append(hits, hit{group: alias.group, rate: rate, pos: pos})
	}

	if len(hits) == 0 {
		result.Warnings = append(result.Warnings, warnings...)
		if hasLoginMarker(text) {
			result.Status = CollectionStatusNeedLogin
			return result
		}
		result.Error = "no upstream rates found"
		return result
	}

	sort.Slice(hits, func(i, j int) bool {
		return hits[i].pos < hits[j].pos
	})
	result.Status = CollectionStatusOK
	result.Warnings = append(result.Warnings, warnings...)
	result.Rates = make([]CollectedRate, 0, len(hits))
	for _, item := range hits {
		result.Rates = append(result.Rates, CollectedRate{
			UpstreamID:    upstream.ID,
			UpstreamGroup: item.group,
			Rate:          item.rate,
			Source:        "html_snapshot",
		})
	}
	return result
}

func ReadCollectionSnapshots(cfg *Config, htmlDir string, now time.Time) []CollectionResult {
	if cfg == nil {
		return nil
	}
	results := make([]CollectionResult, 0, len(cfg.Upstreams))
	for _, upstream := range cfg.Upstreams {
		snapshotPath := filepath.Join(htmlDir, upstream.ID+".html")
		raw, err := os.ReadFile(snapshotPath)
		if err != nil {
			results = append(results, CollectionResult{
				UpstreamID:  upstream.ID,
				Status:      CollectionStatusFailed,
				Error:       err.Error(),
				Snapshot:    snapshotPath,
				CollectedAt: now,
			})
			continue
		}
		result := ExtractRatesFromHTML(upstream, string(raw))
		result.Snapshot = snapshotPath
		result.CollectedAt = now
		for i := range result.Rates {
			result.Rates[i].CollectedAt = now
		}
		results = append(results, result)
	}
	return results
}

func OpenBrowserProfile(profileRoot string, upstream UpstreamConfig) error {
	pageURL := strings.TrimSpace(upstream.PricingPageURL)
	if pageURL == "" {
		pageURL = strings.TrimSpace(upstream.BaseURL)
	}
	if pageURL == "" {
		return fmt.Errorf("upstream %q has no pricing page url", upstream.ID)
	}
	if _, err := url.ParseRequestURI(pageURL); err != nil {
		return fmt.Errorf("invalid pricing page url for upstream %q: %w", upstream.ID, err)
	}

	if profile := strings.TrimSpace(upstream.BrowserProfile); profile != "" {
		if strings.TrimSpace(profileRoot) == "" {
			return errProfileMissing
		}
		profileDir := filepath.Join(profileRoot, "profiles", profile)
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return fmt.Errorf("create browser profile dir for upstream %q: %w", upstream.ID, err)
		}
		readmePath := filepath.Join(profileDir, "README.txt")
		readme := "Manual login/snapshot profile. First version opens the upstream pricing page for manual login and HTML snapshot capture.\n"
		if err := os.WriteFile(readmePath, []byte(readme), 0o644); err != nil {
			return fmt.Errorf("write browser profile note for upstream %q: %w", upstream.ID, err)
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", pageURL)
	} else {
		cmd = exec.Command("xdg-open", pageURL)
	}
	return cmd.Start()
}

type groupAlias struct {
	group string
	alias string
}

func aliasLookup(configured map[string][]string) []groupAlias {
	keys := make([]string, 0, len(configured))
	for group := range configured {
		keys = append(keys, group)
	}
	sort.Strings(keys)
	aliases := make([]groupAlias, 0, len(keys))
	for _, group := range keys {
		names := configured[group]
		if len(names) == 0 {
			aliases = append(aliases, groupAlias{group: group, alias: group})
			continue
		}
		aliases = append(aliases, groupAlias{group: group, alias: group})
		for _, alias := range names {
			trimmed := strings.TrimSpace(alias)
			if trimmed == "" || strings.EqualFold(trimmed, group) {
				continue
			}
			aliases = append(aliases, groupAlias{group: group, alias: trimmed})
		}
	}
	return aliases
}

func normalizeHTMLText(raw string) string {
	text := htmlTagPattern.ReplaceAllString(raw, " ")
	text = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">").Replace(text)
	text = spacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func findRateForAlias(text string, alias string) (int, float64, bool) {
	needle := strings.ToLower(strings.TrimSpace(alias))
	if needle == "" {
		return 0, 0, false
	}
	lower := strings.ToLower(text)
	start := 0
	for {
		idx := strings.Index(lower[start:], needle)
		if idx < 0 {
			return 0, 0, false
		}
		absolute := start + idx
		if rate, ok := parseRateNearAlias(text, absolute, len(needle)); ok {
			return absolute, rate, true
		}
		start = absolute + len(needle)
		if start >= len(lower) {
			return 0, 0, false
		}
	}
}

func parseRateNearAlias(text string, pos int, aliasLen int) (float64, bool) {
	end := pos + aliasLen
	if end > len(text) {
		end = len(text)
	}
	windowEnd := end + 48
	if windowEnd > len(text) {
		windowEnd = len(text)
	}
	match := rateValuePattern.FindStringSubmatch(text[end:windowEnd])
	if len(match) < 2 {
		return 0, false
	}
	rate, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, false
	}
	return rate, true
}

func hasLoginMarker(text string) bool {
	lower := strings.ToLower(text)
	for _, marker := range loginMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
