package web

import (
	"context"
	"strings"
	"time"
)

// GenerateReport creates a structured dossier / report summarizing the bibliographic, repository, and scholarly data.
func (s *dataspaceService) GenerateReport(ctx context.Context, barcodes []string) (DataspaceReportResponse, error) {
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
		return DataspaceReportResponse{
			GeneratedAt: time.Now(),
			ItemCount:   0,
			WorkCount:   0,
			AuthorCount: 0,
			TopicCount:  0,
			InkCount:    0,
			Barcodes:    cleanBarcodes,
			Items:       []DataspaceReportItem{},
			Summary: DataspaceSummaryStats{
				TotalBarcodes:    0,
				CatalogHits:      0,
				InkHits:          0,
				OpenAlexHits:     0,
				PrimaryDomains:   []string{},
				PrimaryFields:    []string{},
				ProminentAuthors: []string{},
			},
			SharedLinks: []DataspaceReportLink{},
		}, nil
	}

	cacheKey := "report:" + strings.Join(cleanBarcodes, ",")
	return s.reportCache.Get(ctx, cacheKey, s.reportLoader)
}

func (s *dataspaceService) buildReport(ctx context.Context, cleanBarcodes []string) (DataspaceReportResponse, error) {
	items, err := s.resolveDataspaceData(ctx, cleanBarcodes)
	if err != nil {
		return DataspaceReportResponse{}, err
	}

	reportItems := make([]DataspaceReportItem, 0, len(items))

	allAuthorsMap := make(map[string]bool)
	allTopicsMap := make(map[string]bool)
	allDomainsMap := make(map[string]int)
	allFieldsMap := make(map[string]int)
	authorOccurrences := make(map[string]int)

	catalogHits := 0
	inkHits := 0
	openAlexHits := 0

	for _, it := range items {
		repItem := DataspaceReportItem{
			Barcode:  it.barcode,
			InkItems: make([]InkItemReport, 0, len(it.inkItems)),
		}

		if it.book != nil {
			catalogHits++
			repItem.HasCatalog = true
			book := it.book

			if len(book.Titles) > 0 {
				repItem.Title = book.Titles[0].Title
				repItem.Subtitle = book.Titles[0].SubTitle
			}
			if len(book.Responsibility) > 0 {
				repItem.Responsibility = book.Responsibility[0].Main
			}
			if repItem.Title == "" {
				repItem.Title = "Medium " + it.barcode
			}

			if len(book.ISBNs) > 0 {
				if book.ISBNs[0].Enriched != "" {
					repItem.ISBN = book.ISBNs[0].Enriched
				} else {
					repItem.ISBN = book.ISBNs[0].ID
				}
			}

			if len(book.Holdings) > 0 {
				if book.Holdings[0].CallNumber != "" {
					repItem.CallNumber = book.Holdings[0].CallNumber
				} else if len(book.Holdings[0].Items) > 0 && book.Holdings[0].Items[0].ItemCallNumber != "" {
					repItem.CallNumber = book.Holdings[0].Items[0].ItemCallNumber
				}
			}

			if book.ID != "" {
				repItem.MMSID = book.ID
			}

			repItem.SwisscoveryURL = buildSwisscoveryURL(book, it.barcode)

			// Publication info
			if len(book.Publications) > 0 {
				pub := book.Publications[0]
				repItem.PublicationYear = pub.DateString
				if len(pub.Publisher) > 0 {
					repItem.Publisher = strings.Join(pub.Publisher, ", ")
				}
			}

			// Authors
			for _, nameMap := range book.PersonalNames {
				for role, nameEntry := range nameMap {
					if nameEntry.NamePart != "" {
						gndID := extractGNDID(nameEntry.Identifier)
						var gndURL string
						if gndID != "" {
							gndURL = "https://lobid.org/gnd/" + gndID
						}
						oaURL, _ := buildOpenAlexAuthorURL(nameEntry.Identifier, "", nameEntry.NamePart)

						repItem.Authors = append(repItem.Authors, DataspaceAuthorEntry{
							Name:        nameEntry.NamePart,
							Role:        role,
							GNDID:       gndID,
							GNDURL:      gndURL,
							OpenAlexURL: oaURL,
						})
						allAuthorsMap[nameEntry.NamePart] = true
						authorOccurrences[nameEntry.NamePart]++
					}
				}
			}

			// Subjects / Topics
			for _, topicMap := range book.Subjects.Topic {
				for _, term := range topicMap {
					if term.Label != "" {
						topicURL, _ := buildTopicOrGNDURL(term.Identifier, term.Label)
						repItem.Subjects = append(repItem.Subjects, DataspaceTopicEntry{
							Label:      term.Label,
							Identifier: term.Identifier,
							URL:        topicURL,
						})
						allTopicsMap[term.Label] = true
					}
				}
			}
		} else {
			repItem.Title = "Medium " + it.barcode + " (Kein Katalogeintrag)"
			repItem.SwisscoveryURL = buildSwisscoveryURL(nil, it.barcode)
		}

		// InK matches
		for _, ink := range it.inkItems {
			inkHits++
			repItem.InkItems = append(repItem.InkItems, InkItemReport{
				Signature:       ink.Signature,
				Title:           ink.Title,
				Type:            ink.Type,
				Date:            ink.Date,
				CollectionTitle: ink.CollectionTitle,
				Publisher:       ink.Publisher,
				Abstract:        ink.Abstract,
				DetailURL:       buildInkDetailURL(ink.Signature, ink.URL),
				Persons:         ink.Persons,
				Tags:            ink.Tags,
			})
		}

		// OpenAlex matches
		for _, oa := range it.openAlexWorks {
			openAlexHits++
			workTitle := oa.DisplayName
			if workTitle == "" {
				workTitle = oa.Title
			}
			oaRep := OpenAlexWorkReport{
				ID:              oa.ID,
				WorkURL:         buildOpenAlexWorkURL(oa.ID, oa.DOI, repItem.ISBN, workTitle),
				DOI:             oa.DOI,
				DisplayName:     workTitle,
				PublicationYear: oa.PublicationYear,
				Type:            oa.Type,
				TypeURL:         buildOpenAlexTypeURL(oa.Type),
			}
			if oa.DOI != "" {
				doiClean := strings.TrimPrefix(oa.DOI, "https://doi.org/")
				doiClean = strings.TrimPrefix(doiClean, "doi:")
				oaRep.DOIURL = "https://doi.org/" + doiClean
			}

			if oa.PrimaryLocation != nil && oa.PrimaryLocation.Source != nil {
				oaRep.SourceName = oa.PrimaryLocation.Source.DisplayName
				oaRep.SourceURL = buildOpenAlexSourceURL(oa.PrimaryLocation.Source.ID, oa.PrimaryLocation.Source.DisplayName)
				oaRep.PublisherName = oa.PrimaryLocation.Source.HostOrgName
				oaRep.PublisherURL = buildOpenAlexPublisherURL(oa.PrimaryLocation.Source.HostOrganization, oa.PrimaryLocation.Source.HostOrgName)
			}

			if oa.PrimaryTopic != nil && oa.PrimaryTopic.DisplayName != "" {
				top := oa.PrimaryTopic
				topURL, _ := buildOpenAlexTopicURL(top.ID, top.DisplayName)
				oaRep.PrimaryTopic = &DataspaceTopicEntry{
					Label: top.DisplayName,
					URL:   topURL,
				}
				allTopicsMap[top.DisplayName] = true

				if top.Subfield != nil && top.Subfield.DisplayName != "" {
					oaRep.PrimaryTopic.Subfield = top.Subfield.DisplayName
					oaRep.PrimaryTopic.SubfieldURL = buildOpenAlexSubfieldURL(top.Subfield.ID, top.Subfield.DisplayName)
				}
				if top.Field != nil && top.Field.DisplayName != "" {
					oaRep.PrimaryTopic.Field = top.Field.DisplayName
					oaRep.PrimaryTopic.FieldURL = buildOpenAlexFieldURL(top.Field.ID, top.Field.DisplayName)
					allFieldsMap[top.Field.DisplayName]++
				}
				if top.Domain != nil && top.Domain.DisplayName != "" {
					oaRep.PrimaryTopic.Domain = top.Domain.DisplayName
					oaRep.PrimaryTopic.DomainURL = buildOpenAlexDomainURL(top.Domain.ID, top.Domain.DisplayName)
					allDomainsMap[top.Domain.DisplayName]++
				}
			}

			for _, secTopic := range oa.Topics {
				if secTopic.DisplayName != "" && (oa.PrimaryTopic == nil || secTopic.ID != oa.PrimaryTopic.ID) {
					secURL, _ := buildOpenAlexTopicURL(secTopic.ID, secTopic.DisplayName)
					oaRep.SecondaryTopics = append(oaRep.SecondaryTopics, DataspaceTopicEntry{
						Label: secTopic.DisplayName,
						URL:   secURL,
					})
					allTopicsMap[secTopic.DisplayName] = true
				}
			}

			for _, auth := range oa.Authorships {
				authName := auth.Author.DisplayName
				if authName != "" {
					allAuthorsMap[authName] = true
					authorOccurrences[authName]++

					authURL, _ := buildOpenAlexAuthorURL(auth.Author.ID, auth.Author.ORCID, authName)
					authItem := OpenAlexAuthorshipItem{
						AuthorName: authName,
						AuthorID:   auth.Author.ID,
						AuthorURL:  authURL,
						ORCID:      auth.Author.ORCID,
					}
					for _, inst := range auth.Institutions {
						if inst.DisplayName != "" {
							authItem.Institutions = append(authItem.Institutions, DataspaceInstitution{
								Name: inst.DisplayName,
								ID:   inst.ID,
								URL:  buildOpenAlexInstitutionURL(inst.ID, inst.ROR, inst.DisplayName),
								ROR:  inst.ROR,
								Type: inst.Type,
							})
						}
					}
					oaRep.Authorships = append(oaRep.Authorships, authItem)
				}
			}

			repItem.OpenAlexWorks = append(repItem.OpenAlexWorks, oaRep)
		}

		reportItems = append(reportItems, repItem)
	}

	// Calculate Top Domains, Fields, Prominent Authors
	topDomains := getTopKeys(allDomainsMap, 3)
	topFields := getTopKeys(allFieldsMap, 4)
	prominentAuthors := getTopKeys(authorOccurrences, 5)

	// Detect Shared Cross-Links
	var sharedLinks []DataspaceReportLink
	if len(reportItems) > 1 {
		for i := 0; i < len(reportItems); i++ {
			for j := i + 1; j < len(reportItems); j++ {
				itemA := reportItems[i]
				itemB := reportItems[j]

				// Check shared authors
				for _, a1 := range itemA.Authors {
					for _, a2 := range itemB.Authors {
						if strings.EqualFold(a1.Name, a2.Name) {
							sharedLinks = append(sharedLinks, DataspaceReportLink{
								SourceBarcode: itemA.Barcode,
								SourceTitle:   itemA.Title,
								TargetBarcode: itemB.Barcode,
								TargetTitle:   itemB.Title,
								RelationType:  "Shared Author",
								Description:   "Gemeinsamer Autor / Herausgeber: " + a1.Name,
							})
						}
					}
				}

				// Check shared topics
				for _, t1 := range itemA.Subjects {
					for _, t2 := range itemB.Subjects {
						if strings.EqualFold(t1.Label, t2.Label) {
							sharedLinks = append(sharedLinks, DataspaceReportLink{
								SourceBarcode: itemA.Barcode,
								SourceTitle:   itemA.Title,
								TargetBarcode: itemB.Barcode,
								TargetTitle:   itemB.Title,
								RelationType:  "Shared Topic",
								Description:   "Gemeinsames Thema: " + t1.Label,
							})
						}
					}
				}
			}
		}
	}

	return DataspaceReportResponse{
		GeneratedAt: time.Now(),
		ItemCount:   len(cleanBarcodes),
		WorkCount:   openAlexHits,
		AuthorCount: len(allAuthorsMap),
		TopicCount:  len(allTopicsMap),
		InkCount:    inkHits,
		Barcodes:    cleanBarcodes,
		Items:       reportItems,
		Summary: DataspaceSummaryStats{
			TotalBarcodes:    len(cleanBarcodes),
			CatalogHits:      catalogHits,
			InkHits:          inkHits,
			OpenAlexHits:     openAlexHits,
			PrimaryDomains:   topDomains,
			PrimaryFields:    topFields,
			ProminentAuthors: prominentAuthors,
		},
		SharedLinks: sharedLinks,
	}, nil
}

func getTopKeys(m map[string]int, limit int) []string {
	if len(m) == 0 {
		return []string{}
	}
	type pair struct {
		k string
		v int
	}
	pairs := make([]pair, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, pair{k: k, v: v})
	}
	// Sort descending by count
	for i := 0; i < len(pairs); i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].v > pairs[i].v {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}
	res := make([]string, 0, limit)
	for i, p := range pairs {
		if i >= limit {
			break
		}
		res = append(res, p.k)
	}
	return res
}
