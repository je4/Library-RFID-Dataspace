package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
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
	catalogSearch func(context.Context, string) ([]catalog.Book, error)
	inkSearch     InkSearchFunc
	cache         *otter.Cache[string, DataspaceGraphResponse]
	loader        dataspaceLoader
}

func newDataspaceService(catalogSearch func(context.Context, string) ([]catalog.Book, error), inkSearch InkSearchFunc) *dataspaceService {
	svc := &dataspaceService{
		catalogSearch: catalogSearch,
		inkSearch:     inkSearch,
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

	nodesMap := make(map[string]bool)
	edgesMap := make(map[string]bool)

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
						addEdge(unresolvedID, "-->", "matched in InK", inkID)

						if ink.CollectionTitle != "" {
							collID := "InkColl_" + slugify(ink.CollectionTitle)
							addNode(&inkNodes, collID, fmt.Sprintf("        %s[\"Collection: %s\"]:::nodeInk", collID, sanitizeMermaidLabel(ink.CollectionTitle)))
							addEdge(inkID, "-->", "collection", collID)
						}

						inkPortalID := "InkPortal"
						addNode(&inkNodes, inkPortalID, "        InkPortal[\"Mediathek InK HGK<br/>mediathek.hgk.fhnw.ch/ink/de\"]:::nodeInk")
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
		addEdge(tagNodeID, "-->", "scanned on reader", almaNodeID)

		// Holding libraries & Shelves
		for _, holding := range book.Holdings {
			libName := holding.Library
			if libName == "" {
				libName = "Bibliothek"
			}
			libNodeID := "Lib_" + slugify(libName)
			addNode(&physicalNodes, libNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeInst", libNodeID, sanitizeMermaidLabel(libName)))
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
					addNode(&scholarlyNodes, authNodeID, fmt.Sprintf("        %s{{\"Author: %s\"}}:::nodeAuthor", authNodeID, authName))
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
					addNode(&scholarlyNodes, corpNodeID, fmt.Sprintf("        %s[\"Institution: %s\"]:::nodeInst", corpNodeID, corpName))
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
					addEdge(almaNodeID, "-->", "digital item / media", inkNodeID)

					if ink.CollectionTitle != "" {
						collNodeID := "InkColl_" + slugify(ink.CollectionTitle)
						addNode(&inkNodes, collNodeID, fmt.Sprintf("        %s[\"Collection: %s\"]:::nodeInk", collNodeID, sanitizeMermaidLabel(ink.CollectionTitle)))
						addEdge(inkNodeID, "-->", "collection", collNodeID)
					}

					inkPortalID := "InkPortal"
					addNode(&inkNodes, inkPortalID, "        InkPortal[\"Mediathek InK HGK<br/>mediathek.hgk.fhnw.ch/ink/de\"]:::nodeInk")
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

		// OpenAlex Work Representation
		workNodeID := fmt.Sprintf("Work_%d", i+1)
		workLabel := fmt.Sprintf("OpenAlex Work: %s", titleStr)
		if isbnStr != "" {
			workLabel += fmt.Sprintf("<br/>ISBN: %s", isbnStr)
		}
		addNode(&scholarlyNodes, workNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeWork", workNodeID, sanitizeMermaidLabel(workLabel)))
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
					addEdge(workNodeID, "-->", "published by", pubNodeID)
				}
			}
		}

		// Subjects / Topics
		var bookTopics []string
		for _, topicMap := range book.Subjects.Topic {
			for _, term := range topicMap {
				if term.Label != "" {
					topicLabel := sanitizeMermaidLabel(term.Label)
					topicNodeID := "Topic_" + slugify(term.Label)
					addNode(&taxonomyNodes, topicNodeID, fmt.Sprintf("        %s[(\"Topic: %s\")]:::nodeTopic", topicNodeID, topicLabel))
					addEdge(workNodeID, "-->", "primary topic", topicNodeID)
					bookTopics = append(bookTopics, topicNodeID)

					// Disciplinary Taxonomy Bridge
					subfieldNodeID := "Subfield_" + slugify("subfield_"+term.Label)
					fieldNodeID := "Field_" + slugify("field_"+term.Label)
					domainNodeID := "Domain_Science"

					addNode(&taxonomyNodes, subfieldNodeID, fmt.Sprintf("        %s[[\"Subfield: %s\"]]:::nodeTaxonomy", subfieldNodeID, topicLabel))
					addNode(&taxonomyNodes, fieldNodeID, fmt.Sprintf("        %s[[\"Field: Academic Field\"]]:::nodeTaxonomy", fieldNodeID))
					addNode(&taxonomyNodes, domainNodeID, fmt.Sprintf("        %s[[\"Domain: Interdisciplinary Sciences\"]]:::nodeTaxonomy", domainNodeID))

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
					addNode(&taxonomyNodes, topicNodeID, fmt.Sprintf("        %s[(\"Subject: %s\")]:::nodeTopic", topicNodeID, topicLabel))
					addEdge(workNodeID, "-->", "treats topic", topicNodeID)
					bookTopics = append(bookTopics, topicNodeID)
				}
			}
		}

		if len(bookTopics) == 0 {
			genericTopicID := fmt.Sprintf("Topic_Gen_%d", i+1)
			genericSubfieldID := fmt.Sprintf("Subfield_Gen_%d", i+1)
			addNode(&taxonomyNodes, genericTopicID, fmt.Sprintf("        %s[(\"Research Topic: %s\")]:::nodeTopic", genericTopicID, sanitizeMermaidLabel(titleStr)))
			addNode(&taxonomyNodes, genericSubfieldID, fmt.Sprintf("        %s[[\"Disciplinary Field\"]]:::nodeTaxonomy", genericSubfieldID))
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
