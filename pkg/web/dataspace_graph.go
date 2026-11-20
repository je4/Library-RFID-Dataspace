package web

import (
	"context"
	"fmt"
	"strings"
)

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

	cacheKey := "graph:" + dir + ":" + strings.Join(cleanBarcodes, ",")
	return s.graphCache.Get(ctx, cacheKey, s.graphLoader)
}

func (s *dataspaceService) buildGraph(ctx context.Context, cleanBarcodes []string, dir string) (DataspaceGraphResponse, error) {
	items, err := s.resolveDataspaceData(ctx, cleanBarcodes)
	if err != nil {
		return DataspaceGraphResponse{}, err
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

			if len(it.inkItems) > 0 {
				for j, ink := range it.inkItems {
					if j >= 2 {
						break
					}
					inkID := fmt.Sprintf("Ink_%d_%d", i+1, j+1)
					inkTitle := truncateText(ink.Title, 40)
					inkLabel := fmt.Sprintf("InK: %s", inkTitle)
					if ink.Type != "" {
						inkLabel += fmt.Sprintf("<br/>Typ: %s", ink.Type)
					}
					addNode(&inkNodes, inkID, fmt.Sprintf("        %s[\"%s\"]:::nodeInk", inkID, sanitizeMermaidLabel(inkLabel)))
					addClick(inkID, buildInkDetailURL(ink.Signature, ink.URL), fmt.Sprintf("Mediathek InK: %s", ink.Title))
					addEdge(unresolvedID, "-.->", "InK Treffer", inkID)
				}
			}
			continue
		}

		book := it.book
		// Determine title and identifiers
		var titleStr string
		if len(book.Titles) > 0 {
			titleStr = book.Titles[0].Title
		}
		if titleStr == "" {
			titleStr = fmt.Sprintf("Item %s", it.barcode)
		}
		shortTitle := truncateText(titleStr, 35)

		var isbnStr string
		if len(book.ISBNs) > 0 {
			if book.ISBNs[0].Enriched != "" {
				isbnStr = book.ISBNs[0].Enriched
			} else {
				isbnStr = book.ISBNs[0].ID
			}
		}

		almaNodeID := fmt.Sprintf("Alma_%d", i+1)
		almaLabel := fmt.Sprintf("Alma: %s", shortTitle)
		if isbnStr != "" {
			almaLabel += fmt.Sprintf("<br/>ISBN: %s", isbnStr)
		}
		addNode(&physicalNodes, almaNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeAlma", almaNodeID, sanitizeMermaidLabel(almaLabel)))
		addClick(almaNodeID, buildSwisscoveryURL(book, it.barcode), fmt.Sprintf("Swisscovery: %s", titleStr))
		addEdge(tagNodeID, "-->", "erfasst", almaNodeID)

		// Digital repository assets (InK - Mediathek HGK)
		if len(it.inkItems) > 0 {
			for j, ink := range it.inkItems {
				if j >= 3 {
					break
				}
				inkID := fmt.Sprintf("Ink_%d_%d", i+1, j+1)
				inkTitle := truncateText(ink.Title, 35)
				inkLabel := fmt.Sprintf("InK: %s", inkTitle)
				if ink.Type != "" {
					inkLabel += fmt.Sprintf("<br/>Typ: %s", ink.Type)
				}
				addNode(&inkNodes, inkID, fmt.Sprintf("        %s[\"%s\"]:::nodeInk", inkID, sanitizeMermaidLabel(inkLabel)))
				addClick(inkID, buildInkDetailURL(ink.Signature, ink.URL), fmt.Sprintf("Mediathek InK: %s (%s)", ink.Title, ink.Signature))
				addEdge(almaNodeID, "-->", "digital asset", inkID)

				if ink.CollectionTitle != "" {
					colNodeID := "Col_" + slugify(ink.CollectionTitle)
					colLabel := fmt.Sprintf("Collection: %s", truncateText(ink.CollectionTitle, 30))
					addNode(&inkNodes, colNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeInk", colNodeID, sanitizeMermaidLabel(colLabel)))
					addClick(colNodeID, "https://mediathek.hgk.fhnw.ch/ink/de", fmt.Sprintf("InK Collection: %s", ink.CollectionTitle))
					addEdge(inkID, "-->", "part of", colNodeID)
				}
			}
		}

		// Authors & GND Links
		var bookAuthors []string
		for _, nameMap := range book.PersonalNames {
			for role, nameEntry := range nameMap {
				if nameEntry.NamePart != "" {
					authorLabel := sanitizeMermaidLabel(nameEntry.NamePart)
					authorNodeID := "Auth_" + slugify(nameEntry.NamePart)
					authorURL, authorTip := buildAuthorOrGNDURL(nameEntry.Identifier, nameEntry.NamePart)
					addNode(&scholarlyNodes, authorNodeID, fmt.Sprintf("        %s{{\"Author: %s\"}}:::nodeAuthor", authorNodeID, authorLabel))
					addClick(authorNodeID, authorURL, authorTip)

					edgeLabel := "created by"
					if role == "aut" {
						edgeLabel = "author"
					} else if role == "edt" {
						edgeLabel = "editor"
					}
					addEdge(almaNodeID, "-->", edgeLabel, authorNodeID)
					bookAuthors = append(bookAuthors, authorNodeID)
				}
			}
		}

		var bookTopics []string

		// OpenAlex Scholarly Knowledge Graph
		if len(it.openAlexWorks) > 0 {
			for j, oaWork := range it.openAlexWorks {
				if j >= 2 {
					break
				}
				oaWorkNodeID := fmt.Sprintf("OAWork_%d_%d", i+1, j+1)
				oaTitle := oaWork.DisplayName
				if oaTitle == "" {
					oaTitle = oaWork.Title
				}
				if oaTitle == "" {
					oaTitle = titleStr
				}
				oaLabel := fmt.Sprintf("OpenAlex: %s", truncateText(oaTitle, 35))
				if oaWork.PublicationYear > 0 {
					oaLabel += fmt.Sprintf("<br/>(%d)", oaWork.PublicationYear)
				}
				oaWorkURL := buildOpenAlexWorkURL(oaWork.ID, oaWork.DOI, isbnStr, oaTitle)

				addNode(&scholarlyNodes, oaWorkNodeID, fmt.Sprintf("        %s[\"%s\"]:::nodeWork", oaWorkNodeID, sanitizeMermaidLabel(oaLabel)))
				addClick(oaWorkNodeID, oaWorkURL, fmt.Sprintf("OpenAlex Work: %s", oaTitle))
				addEdge(almaNodeID, "-.->", "matches work", oaWorkNodeID)

				// OpenAlex Authors & Affiliations
				for _, authorship := range oaWork.Authorships {
					if authorship.Author.DisplayName != "" || authorship.Author.ID != "" {
						authorName := authorship.Author.DisplayName
						authID := authorship.Author.ID
						oaAuthNodeID := "Auth_" + slugify(authorName)
						if authID != "" {
							oaAuthNodeID = "Auth_" + slugify(cleanEntityID(authID, "authors"))
						}
						oaAuthURL, oaAuthTip := buildOpenAlexAuthorURL(authID, authorship.Author.ORCID, authorName)
						addNode(&scholarlyNodes, oaAuthNodeID, fmt.Sprintf("        %s{{\"Author: %s\"}}:::nodeAuthor", oaAuthNodeID, sanitizeMermaidLabel(authorName)))
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

			// Holdings / Institutions
			for _, h := range book.Holdings {
				if h.Library != "" {
					instNodeID := "Inst_" + slugify(h.Library)
					addNode(&scholarlyNodes, instNodeID, fmt.Sprintf("        %s[\"Institution: %s\"]:::nodeInst", instNodeID, sanitizeMermaidLabel(h.Library)))
					addClick(instNodeID, buildOpenAlexInstitutionURL("", "", h.Library), fmt.Sprintf("OpenAlex Institution: %s", h.Library))
					addEdge(almaNodeID, "-->", "holding in", instNodeID)
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

				// Check shared authors
				for _, a1 := range w1.authors {
					for _, a2 := range w2.authors {
						if a1 == a2 {
							addEdge(w1.workID, "-.->", "co-authored with", a1)
							addEdge(w2.workID, "-.->", "co-authored with", a2)
						}
					}
				}

				// Check shared topics
				for _, t1 := range w1.topics {
					for _, t2 := range w2.topics {
						if t1 == t2 {
							addEdge(w1.workID, "===", "shared topic", t1)
							addEdge(w2.workID, "===", "shared topic", t2)
						}
					}
				}

				if i == 0 && j == 1 {
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
