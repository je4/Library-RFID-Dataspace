package web

import (
	"context"
	"strings"
	"time"
)

// GenerateReport creates a structured dossier / report summarizing the bibliographic,
// digital repository, and scholarly knowledge graph data for the given barcodes.
func (s *dataspaceService) GenerateReport(ctx context.Context, barcodes []string) (DataspaceReportResponse, error) {
	cleanBarcodes := cleanStrings(barcodes)

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

// buildReport aggregates all bibliographic, archive, and citation data into a structured dossier.
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

			repItem.Title, repItem.Subtitle = extractBookTitle(book, "Medium "+it.barcode)
			if len(book.Responsibility) > 0 {
				repItem.Responsibility = book.Responsibility[0].Main
			}
			repItem.ISBN = extractPrimaryISBN(book)
			repItem.CallNumber = extractBookCallNumber(book)

			if book.ID != "" {
				repItem.MMSID = book.ID
			}

			repItem.SwisscoveryURL = buildSwisscoveryURL(book, it.barcode)
			repItem.PublicationYear, repItem.Publisher = extractBookPublisher(book)

			// Authors from Alma Catalog
			catalogAuthors := extractCatalogAuthors(book)
			for _, auth := range catalogAuthors {
				repItem.Authors = append(repItem.Authors, DataspaceAuthorEntry{
					Name:        auth.Name,
					Role:        auth.Role,
					GNDID:       auth.GNDID,
					GNDURL:      auth.GNDURL,
					OpenAlexURL: auth.OpenAlexURL,
				})
				allAuthorsMap[auth.Name] = true
				authorOccurrences[auth.Name]++
			}

			// Subjects & Topics from Alma Catalog
			catalogTopics := extractCatalogTopics(book)
			for _, top := range catalogTopics {
				repItem.Subjects = append(repItem.Subjects, DataspaceTopicEntry{
					Label:      top.Label,
					Identifier: top.Identifier,
					URL:        top.URL,
				})
				allTopicsMap[top.Label] = true
			}
		} else {
			repItem.Title = "Medium " + it.barcode + " (Kein Katalogeintrag)"
			repItem.SwisscoveryURL = buildSwisscoveryURL(nil, it.barcode)
		}

		// InK matches from Mediathek HGK
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

		// OpenAlex scholarly works matches
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

	// Calculate Top Domains, Fields, Prominent Authors using generic frequency ranking
	topDomains := topKeysByFrequency(allDomainsMap, 3)
	topFields := topKeysByFrequency(allFieldsMap, 4)
	prominentAuthors := topKeysByFrequency(authorOccurrences, 5)

	// Detect Shared Cross-Links between items
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
