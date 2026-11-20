package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/maypok86/otter/v2"
)

var (
	nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	whitespaceRegex      = regexp.MustCompile(`\s+`)
	gndExtractionRegex   = regexp.MustCompile(`(?i)(?:https?://(?:d-nb\.info|lobid\.org)/gnd/|\(DE-588[a-z]?\)|\(DE-101\)|gnd:?|gnd/)?([0-9X-]{7,15})`)
)

// DataspaceGraphResponse represents the JSON payload returned to the frontend.
type DataspaceGraphResponse struct {
	Mermaid   string   `json:"mermaid"`
	ItemCount int      `json:"itemCount"`
	NodeCount int      `json:"nodeCount"`
	EdgeCount int      `json:"edgeCount"`
	Barcodes  []string `json:"barcodes"`
	Direction string   `json:"direction"`
}

// InkItem represents a digital asset or record from Mediathek InK (fhnw_ink).
type InkItem struct {
	Signature       string      `json:"signature"`
	SignatureOrig   string      `json:"signatureoriginal,omitempty"`
	Source          string      `json:"source,omitempty"`
	Title           string      `json:"title"`
	Type            string      `json:"type,omitempty"`
	Date            string      `json:"date,omitempty"`
	CollectionTitle string      `json:"collectiontitle,omitempty"`
	Catalog         []string    `json:"catalog,omitempty"`
	Category        []string    `json:"category,omitempty"`
	Tags            []string    `json:"tags,omitempty"`
	Persons         []InkPerson `json:"persons,omitempty"`
	URL             string      `json:"url,omitempty"`
	Publisher       string      `json:"publisher,omitempty"`
	Abstract        string      `json:"abstract,omitempty"`
}

// InkPerson represents an author, creator, or contributor in an InK record.
type InkPerson struct {
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// InkSearchFunc searches the fhnw_ink index for matching digital records.
type InkSearchFunc func(ctx context.Context, barcode string, authors []string, title string, isbns []string) ([]InkItem, error)

// OpenAlexWork represents a scholarly work from openalex-works.
type OpenAlexWork struct {
	ID              string               `json:"id"`
	DOI             string               `json:"doi,omitempty"`
	Title           string               `json:"title,omitempty"`
	DisplayName     string               `json:"display_name,omitempty"`
	PublicationYear int                  `json:"publication_year,omitempty"`
	Type            string               `json:"type,omitempty"`
	Authorships     []OpenAlexAuthorship `json:"authorships,omitempty"`
	PrimaryTopic    *OpenAlexTopicRef    `json:"primary_topic,omitempty"`
	Topics          []OpenAlexTopicRef   `json:"topics,omitempty"`
	PrimaryLocation *OpenAlexLocation    `json:"primary_location,omitempty"`
	Locations       []OpenAlexLocation   `json:"locations,omitempty"`
	ReferencedWorks []string             `json:"referenced_works,omitempty"`
	RelatedWorks    []string             `json:"related_works,omitempty"`
}

// OpenAlexAuthorship represents an authorship structure in OpenAlex.
type OpenAlexAuthorship struct {
	Author       OpenAlexAuthorRef        `json:"author"`
	Institutions []OpenAlexInstitutionRef `json:"institutions,omitempty"`
}

// OpenAlexAuthorRef represents an author entity in OpenAlex.
type OpenAlexAuthorRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ORCID       string `json:"orcid,omitempty"`
}

// OpenAlexInstitutionRef represents an institution entity in OpenAlex.
type OpenAlexInstitutionRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ROR         string `json:"ror,omitempty"`
	Type        string `json:"type,omitempty"`
}

// OpenAlexTopicRef represents a research topic in OpenAlex.
type OpenAlexTopicRef struct {
	ID          string               `json:"id"`
	DisplayName string               `json:"display_name"`
	Subfield    *OpenAlexTaxonomyRef `json:"subfield,omitempty"`
	Field       *OpenAlexTaxonomyRef `json:"field,omitempty"`
	Domain      *OpenAlexTaxonomyRef `json:"domain,omitempty"`
}

// OpenAlexTaxonomyRef represents a subfield, field, or domain entity.
type OpenAlexTaxonomyRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// OpenAlexLocation represents a location / hosting venue for a work.
type OpenAlexLocation struct {
	Source *OpenAlexSourceRef `json:"source,omitempty"`
}

// OpenAlexSourceRef represents a journal, repository, or publisher host.
type OpenAlexSourceRef struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	Type             string `json:"type,omitempty"`
	HostOrganization string `json:"host_organization,omitempty"`
	HostOrgName      string `json:"host_organization_name,omitempty"`
}

// OpenAlexSearchFunc searches the openalex-works index for matching scholarly records.
type OpenAlexSearchFunc func(ctx context.Context, isbns []string, title string, authors []string, dois []string) ([]OpenAlexWork, error)

type dataspaceLoader struct {
	svc *dataspaceService
}

func (l dataspaceLoader) Load(ctx context.Context, key string) (DataspaceGraphResponse, error) {
	parts := strings.SplitN(key, ":", 2)
	dir := parts[0]
	var barcodes []string
	if len(parts) > 1 && parts[1] != "" {
		barcodes = strings.Split(parts[1], ",")
	}
	return l.svc.buildGraph(ctx, barcodes, dir)
}

func (l dataspaceLoader) Reload(ctx context.Context, key string, _ DataspaceGraphResponse) (DataspaceGraphResponse, error) {
	return l.Load(ctx, key)
}

type dataspaceService struct {
	catalogSearch  func(context.Context, string) ([]catalog.Book, error)
	inkSearch      InkSearchFunc
	openAlexSearch OpenAlexSearchFunc
	cache          *otter.Cache[string, DataspaceGraphResponse]
	loader         dataspaceLoader
}

func newDataspaceService(catalogSearch func(context.Context, string) ([]catalog.Book, error), inkSearch InkSearchFunc, openAlexSearch OpenAlexSearchFunc) *dataspaceService {
	svc := &dataspaceService{
		catalogSearch:  catalogSearch,
		inkSearch:      inkSearch,
		openAlexSearch: openAlexSearch,
		cache: otter.Must(&otter.Options[string, DataspaceGraphResponse]{
			MaximumSize:      2000,
			ExpiryCalculator: otter.ExpiryWriting[string, DataspaceGraphResponse](5 * time.Minute),
		}),
	}
	svc.loader = dataspaceLoader{svc: svc}
	return svc
}

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

// GenerateMermaid builds a Mermaid flowchart string connecting Alma records and OpenAlex entities.
func (s *dataspaceService) GenerateMermaid(ctx context.Context, barcodes []string, direction string) (DataspaceGraphResponse, error) {
	dir := strings.ToUpper(strings.TrimSpace(direction))
	if dir != "LR" && dir != "TD" {
		dir = "TD"
	}

	cleanBarcodes := make([]string, 0, len(barcodes))
	seenBarcodes := make(map[string]bool)
	for _, b := range barcodes {
		b = strings.TrimSpace(b)
		if b != "" && !seenBarcodes[b] {
			seenBarcodes[b] = true
			cleanBarcodes = append(cleanBarcodes, b)
		}
	}

	if len(cleanBarcodes) == 0 {
		return DataspaceGraphResponse{
			Mermaid:   fmt.Sprintf("flowchart %s\n    Empty([\"Keine aktiven Medien\"])", dir),
			ItemCount: 0,
			NodeCount: 0,
			EdgeCount: 0,
			Barcodes:  cleanBarcodes,
			Direction: dir,
		}, nil
	}

	cacheKey := dir + ":" + strings.Join(cleanBarcodes, ",")
	return s.cache.Get(ctx, cacheKey, s.loader)
}

func (s *dataspaceService) buildGraph(ctx context.Context, cleanBarcodes []string, dir string) (DataspaceGraphResponse, error) {
	// Fetch books for all barcodes
	type itemData struct {
		barcode string
		book    *catalog.Book
	}

	items := make([]itemData, 0, len(cleanBarcodes))
	for _, bc := range cleanBarcodes {
		if s.catalogSearch == nil {
			items = append(items, itemData{barcode: bc, book: nil})
			continue
		}
		books, err := s.catalogSearch(ctx, bc)
		if err != nil || len(books) == 0 {
			items = append(items, itemData{barcode: bc, book: nil})
		} else {
			items = append(items, itemData{barcode: bc, book: &books[0]})
		}
	}

	var physicalNodes []string
	var inkNodes []string
	var scholarlyNodes []string
	var taxonomyNodes []string
	var edges []string
	var clicks []string

	nodesMap := make(map[string]bool)
	edgesMap := make(map[string]bool)
	clicksMap := make(map[string]bool)

	addNode := func(list *[]string, id, def string) {
		if !nodesMap[id] {
			nodesMap[id] = true
			*list = append(*list, def)
		}
	}

	addEdge := func(from, arrow, label, to string) {
		var edgeStr string
		if label != "" {
			edgeStr = fmt.Sprintf("    %s %s|\"%s\"| %s", from, arrow, label, to)
		} else {
			edgeStr = fmt.Sprintf("    %s %s %s", from, arrow, to)
		}
		if !edgesMap[edgeStr] {
			edgesMap[edgeStr] = true
			edges = append(edges, edgeStr)
		}
	}

	addClick := func(nodeID, linkURL, tooltip string) {
		if nodeID == "" || linkURL == "" {
			return
		}
		linkURL = strings.ReplaceAll(linkURL, "\"", "")
		linkURL = strings.TrimSpace(linkURL)
		tooltip = sanitizeMermaidLabel(tooltip)
		clickDef := fmt.Sprintf("    click %s href \"%s\" \"%s\" _blank", nodeID, linkURL, tooltip)
		if !clicksMap[nodeID] {
			clicksMap[nodeID] = true
			clicks = append(clicks, clickDef)
		}
	}

	// Helper to track works across books for citation/overlap linking
	type workRef struct {
		bookIdx int
		workID  string
		title   string
		authors []string
		topics  []string
	}
	var discoveredWorks []workRef

	for i, it := range items {
		tagNodeID := fmt.Sprintf("Tag_%d", i+1)
		tagLabel := fmt.Sprintf("NFC Tag: %s", it.barcode)
		addNode(&physicalNodes, tagNodeID, fmt.Sprintf("        %s([\"%s\"]):::nodeTag", tagNodeID, sanitizeMermaidLabel(tagLabel)))

		if it.book == nil {
			// No catalog record found
			unresolvedID := fmt.Sprintf("Unknown_%d", i+1)
			addNode(&physicalNodes, unresolvedID, fmt.Sprintf("        %s[\"Medium %s<br/>(Kein Katalogtreffer)\"]:::nodeAlma", unresolvedID, sanitizeMermaidLabel(it.barcode)))
			addEdge(tagNodeID, "-->", "erfasst", unresolvedID)

			// Try to search InK directly by barcode if available
			if s.inkSearch != nil {
				inkItems, err := s.inkSearch(ctx, it.barcode, nil, "", nil)
				if err == nil && len(inkItems) > 0 {
					for j, ink := range inkItems {
						if j >= 2 {
							break
						}
						inkID := fmt.Sprintf("Ink_%d_%d", i+1, j+1)
						inkTitle := truncateText(ink.Title, 40)
						inkLabel := fmt.Sprintf("InK: %s", inkTitle)
						if ink.Type != "" {
							inkLabel += fmt.Sprintf("<br/>Typ: %s", ink.Type)
						}
						if ink.Signature != "" {
							inkLabel += fmt.Sprintf("<br/>Sig: %s", ink.Signature)
						}
						addNode(&inkNodes, inkID, fmt.Sprintf("        %s[\"%s\"]:::nodeInk", inkID, sanitizeMermaidLabel(inkLabel)))
						addClick(inkID, buildInkDetailURL(ink.Signature, ink.URL), fmt.Sprintf("InK Mediathek HGK: %s", inkTitle))
						addEdge(unresolvedID, "-->", "matched in InK", inkID)

						if ink.CollectionTitle != "" {
							collID := "InkColl_" + slugify(ink.CollectionTitle)
							addNode(&inkNodes, collID, fmt.Sprintf("        %s[\"Collection: %s\"]:::nodeInk", collID, sanitizeMermaidLabel(ink.CollectionTitle)))
							addClick(collID, "https://mediathek.hgk.fhnw.ch/ink/de", fmt.Sprintf("InK Sammlung: %s", ink.CollectionTitle))
							addEdge(inkID, "-->", "collection", collID)
						}

						inkPortalID := "InkPortal"
						addNode(&inkNodes, inkPortalID, "        InkPortal[\"Mediathek InK HGK<br/>mediathek.hgk.fhnw.ch/ink/de\"]:::nodeInk")
						addClick(inkPortalID, "https://mediathek.hgk.fhnw.ch/ink/de", "Mediathek InK & Archive HGK")
						addEdge(inkID, "-.->", "portal", inkPortalID)
					}
				}
			}
			continue
		}

		book := it.book
		titleStr := "Ohne Titel"
		if len(book.Titles) > 0 && book.Titles[0].Title != "" {
			titleStr = book.Titles[0].Title
		}
		titleStr = truncateText(titleStr, 45)

		var pubYear string
		if len(book.Publications) > 0 && book.Publications[0].DateString != "" {
			pubYear = book.Publications[0].DateString
		}
		var isbnStr string
		var isbnList []string
		if len(book.ISBNs) > 0 {
			for _, isb := range book.ISBNs {
				if isb.Enriched != "" {
					isbnList = append(isbnList, isb.Enriched)
				}
				if isb.ID != "" {
					isbnList = append(isbnList, isb.ID)
				}
			}
			if len(isbnList) > 0 {
				isbnStr = isbnList[0]
			}
		}

		almaNodeID := fmt.Sprintf("Alma_%d", i+1)
		almaLabel := fmt.Sprintf("Alma: %s", titleStr)
		if pubYear != "" {
			almaLabel += fmt.Sprintf(" (%s)", pubYear)
		}
		if isbnStr != "" {
			almaLabel += fmt.Sprintf("<br/>ISBN: %s", isbnStr)
		}
		addNode(&physicalNodes, almaNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeAlma", almaNodeID, sanitizeMermaidLabel(almaLabel)))
		addClick(almaNodeID, buildSwisscoveryURL(book, it.barcode), fmt.Sprintf("Swisscovery: %s", titleStr))
		addEdge(tagNodeID, "-->", "scanned on reader", almaNodeID)

		// Holding libraries & Shelves
		for _, holding := range book.Holdings {
			libName := holding.Library
			if libName == "" {
				libName = "Bibliothek"
			}
			libNodeID := "Lib_" + slugify(libName)
			addNode(&physicalNodes, libNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeInst", libNodeID, sanitizeMermaidLabel(libName)))
			addClick(libNodeID, buildOpenAlexInstitutionURL("", "", libName), fmt.Sprintf("Bibliothek / Institution: %s", libName))
			addEdge(almaNodeID, "-->", "held at", libNodeID)

			if holding.CallNumber != "" {
				shelfNodeID := "Shelf_" + slugify(libName+"_"+holding.CallNumber)
				shelfLabel := fmt.Sprintf("Shelf: %s", holding.CallNumber)
				if holding.Location != "" {
					shelfLabel += fmt.Sprintf(" / %s", holding.Location)
				}
				addNode(&physicalNodes, shelfNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeAlma", shelfNodeID, sanitizeMermaidLabel(shelfLabel)))
				addEdge(almaNodeID, "-->", "located at", shelfNodeID)
			}
		}

		// Authors (Personal & Corporate)
		var bookAuthors []string
		var rawAuthorNames []string
		for _, nameMap := range book.PersonalNames {
			for _, name := range nameMap {
				if name.NamePart != "" {
					rawAuthorNames = append(rawAuthorNames, name.NamePart)
					authName := sanitizeMermaidLabel(name.NamePart)
					authNodeID := "Auth_" + slugify(name.NamePart)
					authURL, authTip := buildAuthorOrGNDURL(name.Identifier, name.NamePart)
					addNode(&scholarlyNodes, authNodeID, fmt.Sprintf("        %s{{\"Author: %s\"}}:::nodeAuthor", authNodeID, authName))
					addClick(authNodeID, authURL, authTip)
					addEdge(almaNodeID, "-->", "written by", authNodeID)
					bookAuthors = append(bookAuthors, authNodeID)
				}
			}
		}
		for _, nameMap := range book.CorporateNames {
			for _, name := range nameMap {
				if name.NamePart != "" {
					rawAuthorNames = append(rawAuthorNames, name.NamePart)
					corpName := sanitizeMermaidLabel(name.NamePart)
					corpNodeID := "Corp_" + slugify(name.NamePart)
					corpURL, corpTip := buildAuthorOrGNDURL(name.Identifier, name.NamePart)
					if !strings.Contains(corpURL, "lobid.org") {
						corpURL = buildOpenAlexInstitutionURL("", "", name.NamePart)
						corpTip = fmt.Sprintf("Institution: %s", name.NamePart)
					}
					addNode(&scholarlyNodes, corpNodeID, fmt.Sprintf("        %s[\"Institution: %s\"]:::nodeInst", corpNodeID, corpName))
					addClick(corpNodeID, corpURL, corpTip)
					addEdge(almaNodeID, "-->", "contributed by", corpNodeID)
				}
			}
		}

		// Search & Link InK (Mediathek HGK) items
		if s.inkSearch != nil {
			inkItems, err := s.inkSearch(ctx, it.barcode, rawAuthorNames, titleStr, isbnList)
			if err == nil && len(inkItems) > 0 {
				for j, ink := range inkItems {
					if j >= 3 {
						break
					}
					inkNodeID := fmt.Sprintf("Ink_%d_%d", i+1, j+1)
					if ink.Signature != "" {
						inkNodeID = "Ink_" + slugify(ink.Signature)
					}
					inkTitle := truncateText(ink.Title, 40)
					inkLabel := fmt.Sprintf("InK: %s", inkTitle)
					if ink.Type != "" {
						inkLabel += fmt.Sprintf("<br/>Typ: %s", ink.Type)
					}
					if ink.Signature != "" {
						inkLabel += fmt.Sprintf("<br/>Sig: %s", ink.Signature)
					}
					addNode(&inkNodes, inkNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeInk", inkNodeID, sanitizeMermaidLabel(inkLabel)))
					addClick(inkNodeID, buildInkDetailURL(ink.Signature, ink.URL), fmt.Sprintf("InK Mediathek HGK: %s", inkTitle))
					addEdge(almaNodeID, "-->", "digital item / media", inkNodeID)

					if ink.CollectionTitle != "" {
						collNodeID := "InkColl_" + slugify(ink.CollectionTitle)
						addNode(&inkNodes, collNodeID, fmt.Sprintf("        %s[\"Collection: %s\"]:::nodeInk", collNodeID, sanitizeMermaidLabel(ink.CollectionTitle)))
						addClick(collNodeID, "https://mediathek.hgk.fhnw.ch/ink/de", fmt.Sprintf("InK Sammlung: %s", ink.CollectionTitle))
						addEdge(inkNodeID, "-->", "collection", collNodeID)
					}

					inkPortalID := "InkPortal"
					addNode(&inkNodes, inkPortalID, "        InkPortal[\"Mediathek InK HGK<br/>mediathek.hgk.fhnw.ch/ink/de\"]:::nodeInk")
					addClick(inkPortalID, "https://mediathek.hgk.fhnw.ch/ink/de", "Mediathek InK & Archive HGK")
					addEdge(inkNodeID, "-.->", "portal", inkPortalID)

					// Connect author node if matched
					for _, p := range ink.Persons {
						for _, authID := range bookAuthors {
							if strings.Contains(strings.ToLower(authID), strings.ToLower(slugify(p.Name))) ||
								strings.Contains(strings.ToLower(p.Name), strings.ToLower(authID)) {
								addEdge(authID, "-->", "author in InK", inkNodeID)
							}
						}
					}
				}
			}
		}

		// OpenAlex Work Representation: Query openalex-works if search is available
		var matchedOAWorks []OpenAlexWork
		if s.openAlexSearch != nil {
			var searchDOIs []string
			if len(book.Identifiers) > 0 {
				for _, oid := range book.Identifiers {
					if strings.Contains(strings.ToLower(oid), "doi") || strings.HasPrefix(oid, "10.") {
						searchDOIs = append(searchDOIs, oid)
					}
				}
			}
			oaWorks, err := s.openAlexSearch(ctx, isbnList, titleStr, rawAuthorNames, searchDOIs)
			if err == nil && len(oaWorks) > 0 {
				matchedOAWorks = oaWorks
			}
		}

		var bookTopics []string

		if len(matchedOAWorks) > 0 {
			for oaIdx, oaWork := range matchedOAWorks {
				if oaIdx >= 2 {
					break
				}
				oaWorkNodeID := fmt.Sprintf("Work_%d", i+1)
				if oaIdx > 0 {
					oaWorkNodeID = fmt.Sprintf("Work_%d_%d", i+1, oaIdx+1)
				}
				if oaWork.ID != "" {
					oaWorkNodeID = "Work_" + slugify(cleanEntityID(oaWork.ID, "works"))
				}
				oaTitle := oaWork.DisplayName
				if oaTitle == "" {
					oaTitle = oaWork.Title
				}
				if oaTitle == "" {
					oaTitle = titleStr
				}
				oaDisplayTitle := truncateText(oaTitle, 40)
				oaLabel := fmt.Sprintf("OpenAlex: %s", oaDisplayTitle)
				if oaWork.PublicationYear > 0 {
					oaLabel += fmt.Sprintf(" (%d)", oaWork.PublicationYear)
				}
				if oaWork.Type != "" {
					oaLabel += fmt.Sprintf("<br/>Type: %s", oaWork.Type)
				}
				addNode(&scholarlyNodes, oaWorkNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeWork", oaWorkNodeID, sanitizeMermaidLabel(oaLabel)))
				addClick(oaWorkNodeID, buildOpenAlexWorkURL(oaWork.ID, oaWork.DOI, isbnStr, oaTitle), fmt.Sprintf("OpenAlex Work: %s", oaTitle))
				addEdge(almaNodeID, "-.->", "resolves to", oaWorkNodeID)

				// OpenAlex Authors & Institutions
				for _, authorship := range oaWork.Authorships {
					if authorship.Author.DisplayName != "" || authorship.Author.ID != "" {
						authName := authorship.Author.DisplayName
						authID := authorship.Author.ID
						oaAuthNodeID := "OAAuth_" + slugify(authName)
						if authID != "" {
							oaAuthNodeID = "OAAuth_" + slugify(cleanEntityID(authID, "authors"))
						}
						oaAuthURL, oaAuthTip := buildOpenAlexAuthorURL(authID, authorship.Author.ORCID, authName)
						addNode(&scholarlyNodes, oaAuthNodeID, fmt.Sprintf("        %s{{\"Author: %s\"}}:::nodeAuthor", oaAuthNodeID, sanitizeMermaidLabel(authName)))
						addClick(oaAuthNodeID, oaAuthURL, oaAuthTip)
						addEdge(oaWorkNodeID, "-->", "authored by", oaAuthNodeID)
						bookAuthors = append(bookAuthors, oaAuthNodeID)

						for _, inst := range authorship.Institutions {
							if inst.DisplayName != "" || inst.ID != "" {
								instName := inst.DisplayName
								instID := inst.ID
								oaInstNodeID := "OAInst_" + slugify(instName)
								if instID != "" {
									oaInstNodeID = "OAInst_" + slugify(cleanEntityID(instID, "institutions"))
								}
								oaInstURL := buildOpenAlexInstitutionURL(instID, inst.ROR, instName)
								addNode(&scholarlyNodes, oaInstNodeID, fmt.Sprintf("        %s[\"Institution: %s\"]:::nodeInst", oaInstNodeID, sanitizeMermaidLabel(instName)))
								addClick(oaInstNodeID, oaInstURL, fmt.Sprintf("OpenAlex Institution: %s", instName))
								addEdge(oaAuthNodeID, "-->", "affiliated with", oaInstNodeID)
							}
						}
					}
				}

				// Sources / Publishers
				if oaWork.PrimaryLocation != nil && oaWork.PrimaryLocation.Source != nil {
					src := oaWork.PrimaryLocation.Source
					if src.DisplayName != "" || src.ID != "" {
						srcName := src.DisplayName
						srcID := src.ID
						oaSrcNodeID := "OASrc_" + slugify(srcName)
						if srcID != "" {
							oaSrcNodeID = "OASrc_" + slugify(cleanEntityID(srcID, "sources"))
						}
						oaSrcURL := buildOpenAlexSourceURL(srcID, srcName)
						addNode(&scholarlyNodes, oaSrcNodeID, fmt.Sprintf("        %s[/\"Source: %s\"/]:::nodeSource", oaSrcNodeID, sanitizeMermaidLabel(srcName)))
						addClick(oaSrcNodeID, oaSrcURL, fmt.Sprintf("OpenAlex Source: %s", srcName))
						addEdge(oaWorkNodeID, "-->", "published in", oaSrcNodeID)

						if src.HostOrganization != "" || src.HostOrgName != "" {
							pubID := src.HostOrganization
							pubName := src.HostOrgName
							if pubName == "" {
								pubName = "Publisher"
							}
							oaPubNodeID := "OAPub_" + slugify(pubName)
							if pubID != "" {
								oaPubNodeID = "OAPub_" + slugify(cleanEntityID(pubID, "publishers"))
							}
							oaPubURL := buildOpenAlexPublisherURL(pubID, pubName)
							addNode(&scholarlyNodes, oaPubNodeID, fmt.Sprintf("        %s[/\"Publisher: %s\"/]:::nodeSource", oaPubNodeID, sanitizeMermaidLabel(pubName)))
							addClick(oaPubNodeID, oaPubURL, fmt.Sprintf("OpenAlex Publisher: %s", pubName))
							addEdge(oaSrcNodeID, "-->", "hosted by", oaPubNodeID)
						}
					}
				}

				// Topics & Taxonomy Hierarchy
				if oaWork.PrimaryTopic != nil && (oaWork.PrimaryTopic.DisplayName != "" || oaWork.PrimaryTopic.ID != "") {
					top := oaWork.PrimaryTopic
					topName := top.DisplayName
					topID := top.ID
					oaTopicNodeID := "Topic_" + slugify(topName)
					if topID != "" {
						oaTopicNodeID = "Topic_" + slugify(cleanEntityID(topID, "topics"))
					}
					oaTopicURL, oaTopicTip := buildOpenAlexTopicURL(topID, topName)
					addNode(&taxonomyNodes, oaTopicNodeID, fmt.Sprintf("        %s[(\"Topic: %s\")]:::nodeTopic", oaTopicNodeID, sanitizeMermaidLabel(topName)))
					addClick(oaTopicNodeID, oaTopicURL, oaTopicTip)
					addEdge(oaWorkNodeID, "-->", "primary topic", oaTopicNodeID)
					bookTopics = append(bookTopics, oaTopicNodeID)

					if top.Subfield != nil && (top.Subfield.DisplayName != "" || top.Subfield.ID != "") {
						sub := top.Subfield
						subName := sub.DisplayName
						subID := sub.ID
						oaSubNodeID := "Subfield_" + slugify(subName)
						if subID != "" {
							oaSubNodeID = "Subfield_" + slugify(cleanEntityID(subID, "subfields"))
						}
						oaSubURL := buildOpenAlexSubfieldURL(subID, subName)
						addNode(&taxonomyNodes, oaSubNodeID, fmt.Sprintf("        %s[[\"Subfield: %s\"]]:::nodeTaxonomy", oaSubNodeID, sanitizeMermaidLabel(subName)))
						addClick(oaSubNodeID, oaSubURL, fmt.Sprintf("OpenAlex Subfield: %s", subName))
						addEdge(oaTopicNodeID, "-->", "subfield of", oaSubNodeID)

						if top.Field != nil && (top.Field.DisplayName != "" || top.Field.ID != "") {
							field := top.Field
							fieldName := field.DisplayName
							fieldID := field.ID
							oaFieldNodeID := "Field_" + slugify(fieldName)
							if fieldID != "" {
								oaFieldNodeID = "Field_" + slugify(cleanEntityID(fieldID, "fields"))
							}
							oaFieldURL := buildOpenAlexFieldURL(fieldID, fieldName)
							addNode(&taxonomyNodes, oaFieldNodeID, fmt.Sprintf("        %s[[\"Field: %s\"]]:::nodeTaxonomy", oaFieldNodeID, sanitizeMermaidLabel(fieldName)))
							addClick(oaFieldNodeID, oaFieldURL, fmt.Sprintf("OpenAlex Field: %s", fieldName))
							addEdge(oaSubNodeID, "-->", "field of", oaFieldNodeID)

							if top.Domain != nil && (top.Domain.DisplayName != "" || top.Domain.ID != "") {
								domain := top.Domain
								domainName := domain.DisplayName
								domainID := domain.ID
								oaDomainNodeID := "Domain_" + slugify(domainName)
								if domainID != "" {
									oaDomainNodeID = "Domain_" + slugify(cleanEntityID(domainID, "domains"))
								}
								oaDomainURL := buildOpenAlexDomainURL(domainID, domainName)
								addNode(&taxonomyNodes, oaDomainNodeID, fmt.Sprintf("        %s[[\"Domain: %s\"]]:::nodeTaxonomy", oaDomainNodeID, sanitizeMermaidLabel(domainName)))
								addClick(oaDomainNodeID, oaDomainURL, fmt.Sprintf("OpenAlex Domain: %s", domainName))
								addEdge(oaFieldNodeID, "-->", "domain of", oaDomainNodeID)
							}
						}
					}
				}

				// Secondary Topics
				for _, secTopic := range oaWork.Topics {
					if secTopic.DisplayName != "" && (oaWork.PrimaryTopic == nil || secTopic.ID != oaWork.PrimaryTopic.ID) {
						secNodeID := "Topic_" + slugify(secTopic.DisplayName)
						if secTopic.ID != "" {
							secNodeID = "Topic_" + slugify(cleanEntityID(secTopic.ID, "topics"))
						}
						secURL, secTip := buildOpenAlexTopicURL(secTopic.ID, secTopic.DisplayName)
						addNode(&taxonomyNodes, secNodeID, fmt.Sprintf("        %s[(\"Topic: %s\")]:::nodeTopic", secNodeID, sanitizeMermaidLabel(secTopic.DisplayName)))
						addClick(secNodeID, secURL, secTip)
						addEdge(oaWorkNodeID, "-->", "treats topic", secNodeID)
						bookTopics = append(bookTopics, secNodeID)
					}
				}

				discoveredWorks = append(discoveredWorks, workRef{
					bookIdx: i,
					workID:  oaWorkNodeID,
					title:   oaTitle,
					authors: bookAuthors,
					topics:  bookTopics,
				})
			}
		} else {
			// Fallback: OpenAlex Work representation derived from Alma catalog metadata
			workNodeID := fmt.Sprintf("Work_%d", i+1)
			workLabel := fmt.Sprintf("OpenAlex Work: %s", titleStr)
			if isbnStr != "" {
				workLabel += fmt.Sprintf("<br/>ISBN: %s", isbnStr)
			}
			addNode(&scholarlyNodes, workNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeWork", workNodeID, sanitizeMermaidLabel(workLabel)))
			addClick(workNodeID, buildOpenAlexWorkURL("", "", isbnStr, titleStr), fmt.Sprintf("OpenAlex Work: %s", titleStr))
			addEdge(almaNodeID, "-.->", "resolves to", workNodeID)

			for _, authID := range bookAuthors {
				addEdge(workNodeID, "-->", "authored by", authID)
			}

			// Publishers / Sources
			for _, pub := range book.Publications {
				for _, publisher := range pub.Publisher {
					if publisher != "" {
						pubNodeID := "Pub_" + slugify(publisher)
						addNode(&scholarlyNodes, pubNodeID, fmt.Sprintf("        %s[/\"Publisher: %s\"/]:::nodeSource", pubNodeID, sanitizeMermaidLabel(publisher)))
						addClick(pubNodeID, buildOpenAlexPublisherURL("", publisher), fmt.Sprintf("OpenAlex Publisher: %s", publisher))
						addEdge(workNodeID, "-->", "published by", pubNodeID)
					}
				}
			}

			// Subjects / Topics
			for _, topicMap := range book.Subjects.Topic {
				for _, term := range topicMap {
					if term.Label != "" {
						topicLabel := sanitizeMermaidLabel(term.Label)
						topicNodeID := "Topic_" + slugify(term.Label)
						topicURL, topicTip := buildTopicOrGNDURL(term.Identifier, term.Label)
						addNode(&taxonomyNodes, topicNodeID, fmt.Sprintf("        %s[(\"Topic: %s\")]:::nodeTopic", topicNodeID, topicLabel))
						addClick(topicNodeID, topicURL, topicTip)
						addEdge(workNodeID, "-->", "primary topic", topicNodeID)
						bookTopics = append(bookTopics, topicNodeID)

						// Disciplinary Taxonomy Bridge
						subfieldNodeID := "Subfield_" + slugify("subfield_"+term.Label)
						fieldNodeID := "Field_" + slugify("field_"+term.Label)
						domainNodeID := "Domain_Science"

						addNode(&taxonomyNodes, subfieldNodeID, fmt.Sprintf("        %s[[\"Subfield: %s\"]]:::nodeTaxonomy", subfieldNodeID, topicLabel))
						addClick(subfieldNodeID, "https://openalex.org/subfields", fmt.Sprintf("OpenAlex Subfield: %s", topicLabel))

						addNode(&taxonomyNodes, fieldNodeID, fmt.Sprintf("        %s[[\"Field: Academic Field\"]]:::nodeTaxonomy", fieldNodeID))
						addClick(fieldNodeID, "https://openalex.org/fields", "OpenAlex Academic Fields")

						addNode(&taxonomyNodes, domainNodeID, fmt.Sprintf("        %s[[\"Domain: Interdisciplinary Sciences\"]]:::nodeTaxonomy", domainNodeID))
						addClick(domainNodeID, "https://openalex.org/domains", "OpenAlex Domains: Science")

						addEdge(topicNodeID, "-->", "subfield of", subfieldNodeID)
						addEdge(subfieldNodeID, "-->", "field of", fieldNodeID)
						addEdge(fieldNodeID, "-->", "domain of", domainNodeID)
					}
				}
			}

			// Fallback topic if no specific subjects
			if len(bookTopics) == 0 && len(book.Subjects.Local.All) > 0 {
				for _, localSub := range book.Subjects.Local.All {
					if localSub != "" {
						topicLabel := sanitizeMermaidLabel(localSub)
						topicNodeID := "Topic_" + slugify(localSub)
						topicURL, topicTip := buildTopicOrGNDURL("", localSub)
						addNode(&taxonomyNodes, topicNodeID, fmt.Sprintf("        %s[(\"Subject: %s\")]:::nodeTopic", topicNodeID, topicLabel))
						addClick(topicNodeID, topicURL, topicTip)
						addEdge(workNodeID, "-->", "treats topic", topicNodeID)
						bookTopics = append(bookTopics, topicNodeID)
					}
				}
			}

			if len(bookTopics) == 0 {
				genericTopicID := fmt.Sprintf("Topic_Gen_%d", i+1)
				genericSubfieldID := fmt.Sprintf("Subfield_Gen_%d", i+1)
				genericURL, genericTip := buildTopicOrGNDURL("", titleStr)
				addNode(&taxonomyNodes, genericTopicID, fmt.Sprintf("        %s[(\"Research Topic: %s\")]:::nodeTopic", genericTopicID, sanitizeMermaidLabel(titleStr)))
				addClick(genericTopicID, genericURL, genericTip)

				addNode(&taxonomyNodes, genericSubfieldID, fmt.Sprintf("        %s[[\"Disciplinary Field\"]]:::nodeTaxonomy", genericSubfieldID))
				addClick(genericSubfieldID, "https://openalex.org/subfields", "OpenAlex Disciplinary Field")

				addEdge(workNodeID, "-->", "primary topic", genericTopicID)
				addEdge(genericTopicID, "-->", "subfield of", genericSubfieldID)
			}

			discoveredWorks = append(discoveredWorks, workRef{
				bookIdx: i,
				workID:  workNodeID,
				title:   titleStr,
				authors: bookAuthors,
				topics:  bookTopics,
			})
		}
	}

	// Multi-item correlation: discover shared authors, topics, or citation paths
	if len(discoveredWorks) > 1 {
		for i := 0; i < len(discoveredWorks); i++ {
			for j := i + 1; j < len(discoveredWorks); j++ {
				w1 := discoveredWorks[i]
				w2 := discoveredWorks[j]

				// If they share authors, they are already linked by the shared author node!
				// If they don't share authors or topics, we can check co-occurrence relation:
				hasSharedAuthor := false
				for _, a1 := range w1.authors {
					for _, a2 := range w2.authors {
						if a1 == a2 {
							hasSharedAuthor = true
							break
						}
					}
				}

				hasSharedTopic := false
				for _, t1 := range w1.topics {
					for _, t2 := range w2.topics {
						if t1 == t2 {
							hasSharedTopic = true
							break
						}
					}
				}

				if !hasSharedAuthor && !hasSharedTopic {
					// Connect through co-presence on the same reader session
					addEdge(w1.workID, "-.->", "co-present on reader", w2.workID)
				} else {
					addEdge(w1.workID, "==>", "RELATED WORK", w2.workID)
				}
			}
		}
	}

	// Assemble complete Mermaid flowchart markup
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("flowchart %s\n", dir))

	if len(physicalNodes) > 0 {
		sb.WriteString("    subgraph PhysicalSpace [\"Physical Library Space (Alma)\"]\n")
		for _, n := range physicalNodes {
			sb.WriteString(n + "\n")
		}
		sb.WriteString("    end\n\n")
	}

	if len(inkNodes) > 0 {
		sb.WriteString("    subgraph InkSpace [\"Mediathek InK & Archive (HGK FHNW)\"]\n")
		for _, n := range inkNodes {
			sb.WriteString(n + "\n")
		}
		sb.WriteString("    end\n\n")
	}

	if len(scholarlyNodes) > 0 {
		sb.WriteString("    subgraph ScholarlyGraph [\"Scholarly Knowledge Graph (OpenAlex)\"]\n")
		for _, n := range scholarlyNodes {
			sb.WriteString(n + "\n")
		}
		sb.WriteString("    end\n\n")
	}

	if len(taxonomyNodes) > 0 {
		sb.WriteString("    subgraph Taxonomy [\"Disciplinary Hierarchy (OpenAlex)\"]\n")
		for _, n := range taxonomyNodes {
			sb.WriteString(n + "\n")
		}
		sb.WriteString("    end\n\n")
	}

	for _, e := range edges {
		sb.WriteString(e + "\n")
	}

	if len(clicks) > 0 {
		sb.WriteString("\n")
		for _, c := range clicks {
			sb.WriteString(c + "\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString("    classDef nodeTag fill:#10b981,stroke:#047857,stroke-width:3px,color:#ffffff,font-weight:bold;\n")
	sb.WriteString("    classDef nodeAlma fill:#2563eb,stroke:#1d4ed8,stroke-width:2px,color:#ffffff;\n")
	sb.WriteString("    classDef nodeInk fill:#db2777,stroke:#9d174d,stroke-width:2px,color:#ffffff;\n")
	sb.WriteString("    classDef nodeWork fill:#4f46e5,stroke:#4338ca,stroke-width:2px,color:#ffffff;\n")
	sb.WriteString("    classDef nodeAuthor fill:#d97706,stroke:#b45309,stroke-width:2px,color:#ffffff;\n")
	sb.WriteString("    classDef nodeTopic fill:#7c3aed,stroke:#6d28d9,stroke-width:2px,color:#ffffff;\n")
	sb.WriteString("    classDef nodeTaxonomy fill:#0891b2,stroke:#0e7490,stroke-width:2px,color:#ffffff;\n")
	sb.WriteString("    classDef nodeInst fill:#475569,stroke:#334155,stroke-width:2px,color:#ffffff;\n")
	sb.WriteString("    classDef nodeSource fill:#0d9488,stroke:#0f766e,stroke-width:2px,color:#ffffff;\n")

	resp := DataspaceGraphResponse{
		Mermaid:   sb.String(),
		ItemCount: len(cleanBarcodes),
		NodeCount: len(nodesMap),
		EdgeCount: len(edgesMap),
		Barcodes:  cleanBarcodes,
		Direction: dir,
	}

	return resp, nil
}

func (ctrl *Controller) handleDataspaceGraph(c *gin.Context) {
	if ctrl.dataspaceService == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	barcodesParam := strings.TrimSpace(c.Query("barcodes"))
	var barcodes []string
	if barcodesParam != "" {
		for _, b := range strings.Split(barcodesParam, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				barcodes = append(barcodes, b)
			}
		}
	} else {
		// Use active tags on reader channel
		channel := c.Param("channel")
		if channel == "" && len(ctrl.cfg.Channels) > 0 {
			channel = ctrl.cfg.Channels[0]
		}
		activeTags := ctrl.hub.GetActiveTagsForChannel(channel, "*")
		for _, t := range activeTags {
			if t.ItemID != "" {
				barcodes = append(barcodes, t.ItemID)
			}
		}
	}

	direction := c.DefaultQuery("direction", "TD")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := ctrl.dataspaceService.GenerateMermaid(ctx, barcodes, direction)
	if err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Warn().Err(err).Msg("dataspace graph generation failed")
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}
