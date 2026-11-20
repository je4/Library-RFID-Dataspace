package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/je4/Library-RFID-Apps/pkg/catalog"
)

var (
	// nonAlphanumericRegex matches any characters outside basic alphanumeric characters and underscores.
	nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	// whitespaceRegex matches sequences of one or more whitespace characters.
	whitespaceRegex = regexp.MustCompile(`\s+`)
	// gndExtractionRegex matches GND identifiers in various standard formats (e.g. Lobid URLs, (DE-588) prefixes, raw IDs).
	gndExtractionRegex = regexp.MustCompile(`(?i)(?:https?://(?:d-nb\.info|lobid\.org)/gnd/|\(DE-588[a-z]?\)|\(DE-101\)|gnd:?|gnd/)?([0-9X-]{7,15})`)
)

// ============================================================================
// String Formatting & Sanitization Helpers
// ============================================================================

// sanitizeMermaidLabel sanitizes strings for inclusion in Mermaid flowchart node labels,
// replacing quotes and line breaks with safe equivalents to avoid syntax breaks.
func sanitizeMermaidLabel(text string) string {
	text = strings.ReplaceAll(text, "\"", "'")
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = whitespaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// truncateText trims text to maxLen characters (measured in Unicode runes) and appends
// an ellipsis ('…') if the text exceeds the limit.
func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen-1]) + "…"
}

// slugify generates a safe, deterministic Mermaid identifier from an arbitrary input string.
// It combines an alphanumeric prefix with a short SHA-256 hash to prevent collisions.
func slugify(text string) string {
	if text == "" {
		return "unknown"
	}
	h := sha256.Sum256([]byte(text))
	hashPrefix := hex.EncodeToString(h[:4])
	clean := nonAlphanumericRegex.ReplaceAllString(text, "_")
	clean = strings.Trim(clean, "_")
	if len(clean) > 20 {
		clean = clean[:20]
	}
	if clean == "" {
		return "n_" + hashPrefix
	}
	return clean + "_" + hashPrefix
}

// extractGNDID attempts to extract a clean German Integrated Authority File (GND) identifier
// from strings containing prefixes such as "(DE-588)", "d-nb.info", or "lobid.org".
func extractGNDID(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return ""
	}
	m := gndExtractionRegex.FindStringSubmatch(identifier)
	if len(m) > 1 && len(m[1]) >= 7 {
		return m[1]
	}
	return ""
}

// cleanEntityID strips OpenAlex URL and type prefixes from an entity ID (e.g. converting
// "https://openalex.org/works/W642892681" or "works/W642892681" into "W642892681").
func cleanEntityID(rawID, defaultPrefix string) string {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return ""
	}
	rawID = strings.TrimPrefix(rawID, "https://openalex.org/")
	rawID = strings.TrimPrefix(rawID, "http://openalex.org/")
	if defaultPrefix != "" {
		rawID = strings.TrimPrefix(rawID, defaultPrefix+"/")
	}
	return rawID
}

// ============================================================================
// URI & Deep Link Generators
// ============================================================================

// buildSwisscoveryURL constructs a search deep link to the Swisscovery discovery network.
func buildSwisscoveryURL(book *catalog.Book, fallbackQuery string) string {
	queryVal := fallbackQuery
	if book != nil {
		if book.ID != "" {
			id := book.ID
			if idx := strings.LastIndex(id, ":"); idx != -1 {
				id = id[idx+1:]
			}
			queryVal = id
		} else if len(book.ISBNs) > 0 {
			if book.ISBNs[0].Enriched != "" {
				queryVal = book.ISBNs[0].Enriched
			} else if book.ISBNs[0].ID != "" {
				queryVal = book.ISBNs[0].ID
			}
		} else if len(book.Titles) > 0 && book.Titles[0].Title != "" {
			queryVal = book.Titles[0].Title
		}
	}
	return fmt.Sprintf("https://swisscovery.ch/discovery/search?query=any,contains,%s&tab=41SLSP_NETWORK&search_scope=DN_and_CI&vid=41SLSP_NETWORK:VU1_UNION", url.QueryEscape(queryVal))
}

// buildInkDetailURL constructs a deep link into the Mediathek InK digital repository portal.
func buildInkDetailURL(signature, fallbackURL string) string {
	if signature != "" {
		return fmt.Sprintf("https://mediathek.hgk.fhnw.ch/ink/detail/%s/de", url.PathEscape(signature))
	}
	if fallbackURL != "" {
		return fallbackURL
	}
	return "https://mediathek.hgk.fhnw.ch/ink/de"
}

// buildInkEstateURL constructs a deep link into the Mediathek InK portal for an estate / collection.
func buildInkEstateURL(estateName string) string {
	if estateName != "" {
		return fmt.Sprintf("https://mediathek.hgk.fhnw.ch/ink/grid/de?searchtype=estate&search=%s", url.QueryEscape(estateName))
	}
	return "https://mediathek.hgk.fhnw.ch/ink/de"
}

// buildInkCategoryURL constructs a deep link into the Mediathek InK portal for a category search.
func buildInkCategoryURL(category string) string {
	cleaned := strings.Trim(strings.TrimSpace(category), "\"")
	if cleaned != "" {
		escaped := strings.ReplaceAll(url.QueryEscape(cleaned), "%21", "!")
		return fmt.Sprintf("https://mediathek.hgk.fhnw.ch/ink/grid/de?search=category:%%22%s%%22", escaped)
	}
	return "https://mediathek.hgk.fhnw.ch/ink/de"
}

// buildInkTagURL constructs a deep link into the Mediathek InK portal for a tag or keyword search.
func buildInkTagURL(tag string) string {
	cleaned := strings.TrimSpace(tag)
	if cleaned != "" {
		return fmt.Sprintf("https://mediathek.hgk.fhnw.ch/ink/grid/de?search=%s", url.QueryEscape(cleaned))
	}
	return "https://mediathek.hgk.fhnw.ch/ink/de"
}

// formatInkCategoryLabel formats hierarchical InK category strings (e.g. "zotero2!!Videonetzwerke!!Videogenossenschaft")
// into a clean, human-readable display label ("Videonetzwerke › Videogenossenschaft") by omitting the first hierarchy level.
func formatInkCategoryLabel(category string) string {
	category = strings.TrimSpace(category)
	if category == "" {
		return ""
	}
	parts := strings.Split(category, "!!")
	if len(parts) > 1 {
		return strings.Join(parts[1:], " › ")
	}
	return ""
}

// extractInkCategories processes raw InK category strings, stripping the root hierarchy level
// and pruning redundant parent prefix paths so only the most specific category branches are shown.
func extractInkCategories(rawCategories []string) []InkCategoryReport {
	if len(rawCategories) == 0 {
		return nil
	}

	var validCats []string
	for _, c := range rawCategories {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		parts := strings.Split(c, "!!")
		if len(parts) > 1 {
			validCats = append(validCats, c)
		}
	}

	var result []InkCategoryReport
	for _, c := range validCats {
		isAncestor := false
		prefix := c + "!!"
		for _, other := range validCats {
			if other != c && strings.HasPrefix(other, prefix) {
				isAncestor = true
				break
			}
		}
		if !isAncestor {
			label := formatInkCategoryLabel(c)
			if label != "" {
				result = append(result, InkCategoryReport{
					Raw:   c,
					Label: label,
					URL:   buildInkCategoryURL(c),
				})
			}
		}
	}
	return result
}

// extractInkCategoryGroups groups InK categories by common hierarchical parent prefixes
// so duplicate prefixes are avoided and leaf categories are cleanly organized under their parents.
func extractInkCategoryGroups(rawCategories []string) []InkCategoryGroup {
	categories := extractInkCategories(rawCategories)
	if len(categories) == 0 {
		return nil
	}

	var groups []InkCategoryGroup
	groupIndex := make(map[string]int)

	for _, c := range categories {
		parts := strings.Split(c.Raw, "!!")
		if len(parts) <= 1 {
			continue
		}

		var prefix, prefixURL, leafLabel string
		if len(parts) == 2 {
			prefix = ""
			prefixURL = ""
			leafLabel = parts[1]
		} else {
			prefixParts := parts[1 : len(parts)-1]
			prefix = strings.Join(prefixParts, " › ")
			rawPrefix := strings.Join(parts[0:len(parts)-1], "!!")
			prefixURL = buildInkCategoryURL(rawPrefix)
			leafLabel = parts[len(parts)-1]
		}

		item := InkCategoryReport{
			Raw:   c.Raw,
			Label: leafLabel,
			URL:   c.URL,
		}

		if idx, exists := groupIndex[prefix]; exists {
			groups[idx].Items = append(groups[idx].Items, item)
		} else {
			groupIndex[prefix] = len(groups)
			groups = append(groups, InkCategoryGroup{
				Prefix:    prefix,
				PrefixURL: prefixURL,
				Items:     []InkCategoryReport{item},
			})
		}
	}

	return groups
}

// buildOpenAlexCategoryURL builds a direct URL for OpenAlex taxonomy and category concepts
// (e.g. types, subfields, fields, domains, source-types, institution-types, indexes).
func buildOpenAlexCategoryURL(category, rawValue string) string {
	cleaned := cleanEntityID(rawValue, category)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned != "" {
		return fmt.Sprintf("https://openalex.org/%s/%s", category, url.PathEscape(strings.ToLower(cleaned)))
	}
	return fmt.Sprintf("https://openalex.org/%s", category)
}

// buildOpenAlexWorkURL generates a direct OpenAlex work URL or a targeted search fallback URL.
func buildOpenAlexWorkURL(workID, doi, isbn, title string) string {
	if id := cleanEntityID(workID, "works"); id != "" && (strings.HasPrefix(id, "W") || len(id) > 2) {
		return fmt.Sprintf("https://openalex.org/works/%s", id)
	}
	if doi != "" {
		cleanDOI := strings.TrimPrefix(doi, "https://doi.org/")
		cleanDOI = strings.TrimPrefix(cleanDOI, "http://doi.org/")
		cleanDOI = strings.TrimPrefix(cleanDOI, "doi:")
		return fmt.Sprintf("https://openalex.org/works?search=%s", url.QueryEscape(cleanDOI))
	}
	if isbn != "" {
		return fmt.Sprintf("https://openalex.org/works?search=%s", url.QueryEscape(isbn))
	}
	if title != "" {
		return fmt.Sprintf("https://openalex.org/works?search=%s", url.QueryEscape(title))
	}
	return "https://openalex.org/works"
}

// buildOpenAlexCitingWorksURL generates a direct URL to the OpenAlex list of works citing the given work ID.
func buildOpenAlexCitingWorksURL(workID string) string {
	if id := cleanEntityID(workID, "works"); id != "" && (strings.HasPrefix(id, "W") || len(id) > 2) {
		return fmt.Sprintf("https://openalex.org/works?filter=cites:%s", id)
	}
	return "https://openalex.org/works"
}

// buildOpenAlexAuthorURL generates an author profile URL (OpenAlex or ORCID) and an informative tooltip.
func buildOpenAlexAuthorURL(authorID, orcid, name string) (urlStr string, tooltip string) {
	if id := cleanEntityID(authorID, "authors"); id != "" && (strings.HasPrefix(id, "A") || len(id) > 2) {
		tip := fmt.Sprintf("OpenAlex Author: %s", id)
		if name != "" {
			tip = fmt.Sprintf("OpenAlex Author: %s (%s)", name, id)
		}
		return fmt.Sprintf("https://openalex.org/authors/%s", id), tip
	}
	if orcid != "" {
		cleanORCID := strings.TrimPrefix(orcid, "https://orcid.org/")
		cleanORCID = strings.TrimPrefix(cleanORCID, "http://orcid.org/")
		return fmt.Sprintf("https://orcid.org/%s", cleanORCID), fmt.Sprintf("ORCID: %s", cleanORCID)
	}
	if name != "" {
		return fmt.Sprintf("https://openalex.org/authors?search=%s", url.QueryEscape(name)), fmt.Sprintf("OpenAlex Author: %s", name)
	}
	return "https://openalex.org/authors", "OpenAlex Authors"
}

// buildAuthorOrGNDURL checks whether the identifier is a GND authority ID; if found,
// it returns a lobid.org GND deep link. Otherwise, it delegates to buildOpenAlexAuthorURL.
func buildAuthorOrGNDURL(identifier, name string) (urlStr string, tooltip string) {
	if gndID := extractGNDID(identifier); gndID != "" {
		return fmt.Sprintf("https://lobid.org/gnd/%s", url.PathEscape(gndID)), fmt.Sprintf("GND: %s (lobid.org)", gndID)
	}
	return buildOpenAlexAuthorURL(identifier, "", name)
}

// buildOpenAlexTopicURL builds a direct OpenAlex topic URL and tooltip or a topic search URL.
func buildOpenAlexTopicURL(topicID, label string) (urlStr string, tooltip string) {
	if id := cleanEntityID(topicID, "topics"); id != "" && (strings.HasPrefix(id, "T") || len(id) > 2) {
		tip := fmt.Sprintf("OpenAlex Topic: %s", id)
		if label != "" {
			tip = fmt.Sprintf("OpenAlex Topic: %s (%s)", label, id)
		}
		return fmt.Sprintf("https://openalex.org/topics/%s", id), tip
	}
	if label != "" {
		return fmt.Sprintf("https://openalex.org/topics?search=%s", url.QueryEscape(label)), fmt.Sprintf("OpenAlex Topic: %s", label)
	}
	return "https://openalex.org/topics", "OpenAlex Topics"
}

// buildTopicOrGNDURL checks whether the subject identifier is a GND ID; if found,
// it returns a lobid.org GND deep link. Otherwise, it delegates to buildOpenAlexTopicURL.
func buildTopicOrGNDURL(identifier, label string) (urlStr string, tooltip string) {
	if gndID := extractGNDID(identifier); gndID != "" {
		return fmt.Sprintf("https://lobid.org/gnd/%s", url.PathEscape(gndID)), fmt.Sprintf("GND: %s (lobid.org)", gndID)
	}
	return buildOpenAlexTopicURL(identifier, label)
}

// buildOpenAlexSubfieldURL builds a direct URL to an OpenAlex subfield entity.
func buildOpenAlexSubfieldURL(subfieldID, label string) string {
	return buildOpenAlexCategoryURL("subfields", subfieldID)
}

// buildOpenAlexFieldURL builds a direct URL to an OpenAlex field entity.
func buildOpenAlexFieldURL(fieldID, label string) string {
	return buildOpenAlexCategoryURL("fields", fieldID)
}

// buildOpenAlexDomainURL builds a direct URL to an OpenAlex domain entity.
func buildOpenAlexDomainURL(domainID, label string) string {
	return buildOpenAlexCategoryURL("domains", domainID)
}

// buildOpenAlexInstitutionURL generates a direct OpenAlex institution URL, ROR link, or search URL.
func buildOpenAlexInstitutionURL(instID, ror, name string) string {
	if id := cleanEntityID(instID, "institutions"); id != "" && (strings.HasPrefix(id, "I") || len(id) > 2) {
		return fmt.Sprintf("https://openalex.org/institutions/%s", id)
	}
	if ror != "" {
		cleanROR := strings.TrimPrefix(ror, "https://ror.org/")
		return fmt.Sprintf("https://ror.org/%s", cleanROR)
	}
	if name != "" {
		return fmt.Sprintf("https://openalex.org/institutions?search=%s", url.QueryEscape(name))
	}
	return "https://openalex.org/institutions"
}

// buildOpenAlexSourceURL generates a direct OpenAlex source (journal, repository) profile URL or search URL.
func buildOpenAlexSourceURL(sourceID, name string) string {
	if id := cleanEntityID(sourceID, "sources"); id != "" && (strings.HasPrefix(id, "S") || len(id) > 2) {
		return fmt.Sprintf("https://openalex.org/sources/%s", id)
	}
	if name != "" {
		return fmt.Sprintf("https://openalex.org/sources?search=%s", url.QueryEscape(name))
	}
	return "https://openalex.org/sources"
}

// buildOpenAlexPublisherURL generates a direct OpenAlex publisher entity URL or search URL.
func buildOpenAlexPublisherURL(pubID, name string) string {
	if id := cleanEntityID(pubID, "publishers"); id != "" && (strings.HasPrefix(id, "P") || len(id) > 2) {
		return fmt.Sprintf("https://openalex.org/publishers/%s", id)
	}
	if name != "" {
		return fmt.Sprintf("https://openalex.org/publishers?search=%s", url.QueryEscape(name))
	}
	return "https://openalex.org/publishers"
}

// buildOpenAlexTypeURL generates a direct OpenAlex work type taxonomy URL.
func buildOpenAlexTypeURL(workType string) string {
	return buildOpenAlexCategoryURL("types", workType)
}

// buildOpenAlexInstitutionTypeURL generates a direct OpenAlex institution type taxonomy URL.
func buildOpenAlexInstitutionTypeURL(instType string) string {
	return buildOpenAlexCategoryURL("institution-types", instType)
}

// buildOpenAlexSourceTypeURL generates a direct OpenAlex source type taxonomy URL.
func buildOpenAlexSourceTypeURL(sourceType string) string {
	return buildOpenAlexCategoryURL("source-types", sourceType)
}

// buildOpenAlexIndexURL generates a direct OpenAlex index identifier taxonomy URL.
func buildOpenAlexIndexURL(indexName string) string {
	return buildOpenAlexCategoryURL("indexes", indexName)
}

// buildOpenAlexSourceListURL generates a direct OpenAlex source list taxonomy URL.
func buildOpenAlexSourceListURL(listName string) string {
	return buildOpenAlexCategoryURL("source-lists", listName)
}
