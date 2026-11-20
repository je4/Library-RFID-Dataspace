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
	nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	whitespaceRegex      = regexp.MustCompile(`\s+`)
	gndExtractionRegex   = regexp.MustCompile(`(?i)(?:https?://(?:d-nb\.info|lobid\.org)/gnd/|\(DE-588[a-z]?\)|\(DE-101\)|gnd:?|gnd/)?([0-9X-]{7,15})`)
)

func sanitizeMermaidLabel(text string) string {
	text = strings.ReplaceAll(text, "\"", "'")
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = whitespaceRegex.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	return text
}

func truncateText(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen-1]) + "…"
}

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

func buildInkDetailURL(signature, fallbackURL string) string {
	if signature != "" {
		return fmt.Sprintf("https://mediathek.hgk.fhnw.ch/ink/detail/%s/de", url.PathEscape(signature))
	}
	if fallbackURL != "" {
		return fallbackURL
	}
	return "https://mediathek.hgk.fhnw.ch/ink/de"
}

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

func buildAuthorOrGNDURL(identifier, name string) (urlStr string, tooltip string) {
	if gndID := extractGNDID(identifier); gndID != "" {
		return fmt.Sprintf("https://lobid.org/gnd/%s", url.PathEscape(gndID)), fmt.Sprintf("GND: %s (lobid.org)", gndID)
	}
	return buildOpenAlexAuthorURL(identifier, "", name)
}

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

func buildTopicOrGNDURL(identifier, label string) (urlStr string, tooltip string) {
	if gndID := extractGNDID(identifier); gndID != "" {
		return fmt.Sprintf("https://lobid.org/gnd/%s", url.PathEscape(gndID)), fmt.Sprintf("GND: %s (lobid.org)", gndID)
	}
	return buildOpenAlexTopicURL(identifier, label)
}

func buildOpenAlexSubfieldURL(subfieldID, label string) string {
	if id := cleanEntityID(subfieldID, "subfields"); id != "" {
		return fmt.Sprintf("https://openalex.org/subfields/%s", id)
	}
	return "https://openalex.org/subfields"
}

func buildOpenAlexFieldURL(fieldID, label string) string {
	if id := cleanEntityID(fieldID, "fields"); id != "" {
		return fmt.Sprintf("https://openalex.org/fields/%s", id)
	}
	return "https://openalex.org/fields"
}

func buildOpenAlexDomainURL(domainID, label string) string {
	if id := cleanEntityID(domainID, "domains"); id != "" {
		return fmt.Sprintf("https://openalex.org/domains/%s", id)
	}
	return "https://openalex.org/domains"
}

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

func buildOpenAlexSourceURL(sourceID, name string) string {
	if id := cleanEntityID(sourceID, "sources"); id != "" && (strings.HasPrefix(id, "S") || len(id) > 2) {
		return fmt.Sprintf("https://openalex.org/sources/%s", id)
	}
	if name != "" {
		return fmt.Sprintf("https://openalex.org/sources?search=%s", url.QueryEscape(name))
	}
	return "https://openalex.org/sources"
}

func buildOpenAlexPublisherURL(pubID, name string) string {
	if id := cleanEntityID(pubID, "publishers"); id != "" && (strings.HasPrefix(id, "P") || len(id) > 2) {
		return fmt.Sprintf("https://openalex.org/publishers/%s", id)
	}
	if name != "" {
		return fmt.Sprintf("https://openalex.org/publishers?search=%s", url.QueryEscape(name))
	}
	return "https://openalex.org/publishers"
}

func buildOpenAlexTypeURL(workType string) string {
	if workType != "" {
		return fmt.Sprintf("https://openalex.org/types/%s", url.PathEscape(strings.ToLower(strings.TrimSpace(workType))))
	}
	return "https://openalex.org/types"
}

func buildOpenAlexInstitutionTypeURL(instType string) string {
	if instType != "" {
		return fmt.Sprintf("https://openalex.org/institution-types/%s", url.PathEscape(strings.ToLower(strings.TrimSpace(instType))))
	}
	return "https://openalex.org/institution-types"
}

func buildOpenAlexSourceTypeURL(sourceType string) string {
	if sourceType != "" {
		return fmt.Sprintf("https://openalex.org/source-types/%s", url.PathEscape(strings.ToLower(strings.TrimSpace(sourceType))))
	}
	return "https://openalex.org/source-types"
}

func buildOpenAlexIndexURL(indexName string) string {
	if indexName != "" {
		return fmt.Sprintf("https://openalex.org/indexes/%s", url.PathEscape(strings.ToLower(strings.TrimSpace(indexName))))
	}
	return "https://openalex.org/indexes"
}

func buildOpenAlexSourceListURL(listName string) string {
	if listName != "" {
		return fmt.Sprintf("https://openalex.org/source-lists/%s", url.PathEscape(strings.ToLower(strings.TrimSpace(listName))))
	}
	return "https://openalex.org/source-lists"
}
