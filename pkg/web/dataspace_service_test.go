package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
)

func TestDataspaceService_Empty(t *testing.T) {
	svc := newDataspaceService(nil, nil, nil)
	resp, err := svc.GenerateMermaid(context.Background(), nil, "TD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ItemCount != 0 {
		t.Errorf("expected 0 items, got %d", resp.ItemCount)
	}
	if !strings.HasPrefix(resp.Mermaid, "flowchart TD") {
		t.Errorf("expected flowchart TD prefix, got: %s", resp.Mermaid)
	}
}

func TestDataspaceService_SingleBook(t *testing.T) {
	mockSearch := func(_ context.Context, barcode string) ([]catalog.Book, error) {
		return []catalog.Book{
			{
				ID: "oai:alma.41SLSP_FNW:990049881560205518",
				Titles: []catalog.Title{
					{Title: "Heinrich Hertz: Erinnerungen, Briefe, Tagebücher"},
				},
				ISBNs: []catalog.ISBN{
					{Enriched: "978-3-8171-3420-1"},
				},
				PersonalNames: []map[string]catalog.Name{
					{"aut": {NamePart: "Fölsing, Albrecht", Identifier: "(DE-588)118772295"}},
				},
				Holdings: []catalog.Holding{
					{
						Library:    "FHNW Campus Muttenz",
						CallNumber: "530.092 HER",
						Location:   "EG-104",
					},
				},
				Subjects: catalog.Subject{
					Topic: []map[string]catalog.SubjectTerm{
						{"top": {Label: "Electromagnetism", Identifier: "(DE-588)4002851-3"}},
					},
				},
			},
		}, nil
	}

	mockInk := func(_ context.Context, barcode string, authors []string, title string, isbns []string) ([]InkItem, error) {
		return []InkItem{
			{
				Signature:       "zotero2-2068924.UHBBY3JP",
				Title:           "Classic Design – Future Design Classics",
				Type:            "thesis",
				CollectionTitle: "Archiv Institut Industrial Design",
				Persons: []InkPerson{
					{Name: "Fölsing, Albrecht", Role: "creator"},
				},
			},
		}, nil
	}

	svc := newDataspaceService(mockSearch, mockInk, nil)
	resp, err := svc.GenerateMermaid(context.Background(), []string{"0001847294"}, "TD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ItemCount != 1 {
		t.Errorf("expected 1 item, got %d", resp.ItemCount)
	}
	if !strings.Contains(resp.Mermaid, "PhysicalSpace") {
		t.Errorf("expected PhysicalSpace subgraph in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "InkSpace") {
		t.Errorf("expected InkSpace subgraph in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "ScholarlyGraph") {
		t.Errorf("expected ScholarlyGraph subgraph in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "Taxonomy") {
		t.Errorf("expected Taxonomy subgraph in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "classDef nodeTag") {
		t.Errorf("expected classDef nodeTag in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "classDef nodeAlma") {
		t.Errorf("expected classDef nodeAlma in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "classDef nodeInk") {
		t.Errorf("expected classDef nodeInk in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "Classic Design") {
		t.Errorf("expected InK title in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "Archiv Institut Industrial Design") {
		t.Errorf("expected InK collection in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "mediathek.hgk.fhnw.ch/ink/de") {
		t.Errorf("expected InK portal url in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "Fölsing, Albrecht") {
		t.Errorf("expected author name in Mermaid")
	}
	if !strings.Contains(resp.Mermaid, "Electromagnetism") {
		t.Errorf("expected topic name in Mermaid")
	}

	// Verify deep links in Mermaid flowchart
	if !strings.Contains(resp.Mermaid, "https://swisscovery.ch/discovery/search?query=any,contains,990049881560205518") {
		t.Errorf("expected Swisscovery click deep link in Mermaid, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://lobid.org/gnd/118772295") {
		t.Errorf("expected GND lobid click deep link on author in Mermaid, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://mediathek.hgk.fhnw.ch/ink/detail/zotero2-2068924.UHBBY3JP/de") {
		t.Errorf("expected InK item detail click deep link in Mermaid, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://lobid.org/gnd/4002851-3") {
		t.Errorf("expected GND lobid click deep link on topic in Mermaid, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/works?search=978-3-8171-3420-1") {
		t.Errorf("expected OpenAlex Work click deep link in Mermaid, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/institutions?search=FHNW+Campus+Muttenz") {
		t.Errorf("expected OpenAlex Institution click deep link in Mermaid, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/subfields") {
		t.Errorf("expected OpenAlex Subfields click deep link in Mermaid, got: %s", resp.Mermaid)
	}
}

func TestDataspaceService_URLHelpers(t *testing.T) {
	// GND Extraction
	if id := extractGNDID("(DE-588)118772295"); id != "118772295" {
		t.Errorf("expected 118772295, got %s", id)
	}
	if id := extractGNDID("https://lobid.org/gnd/118772295"); id != "118772295" {
		t.Errorf("expected 118772295, got %s", id)
	}
	if id := extractGNDID("4002851-3"); id != "4002851-3" {
		t.Errorf("expected 4002851-3, got %s", id)
	}

	// OpenAlex Direct Work URLs
	if url := buildOpenAlexWorkURL("https://openalex.org/W642892681", "", "", ""); url != "https://openalex.org/works/W642892681" {
		t.Errorf("expected https://openalex.org/works/W642892681, got %s", url)
	}
	if url := buildOpenAlexWorkURL("W642892681", "", "", ""); url != "https://openalex.org/works/W642892681" {
		t.Errorf("expected https://openalex.org/works/W642892681, got %s", url)
	}

	// OpenAlex Direct Author URLs
	if url, _ := buildOpenAlexAuthorURL("https://openalex.org/A5090064378", "", "Albrecht Fölsing"); url != "https://openalex.org/authors/A5090064378" {
		t.Errorf("expected https://openalex.org/authors/A5090064378, got %s", url)
	}

	// OpenAlex Direct Institution URLs
	if url := buildOpenAlexInstitutionURL("https://openalex.org/I118019719", "", "University of Hamburg"); url != "https://openalex.org/institutions/I118019719" {
		t.Errorf("expected https://openalex.org/institutions/I118019719, got %s", url)
	}

	// OpenAlex Direct Source URLs
	if url := buildOpenAlexSourceURL("https://openalex.org/S7407052672", "Physikalische Blätter"); url != "https://openalex.org/sources/S7407052672" {
		t.Errorf("expected https://openalex.org/sources/S7407052672, got %s", url)
	}

	// OpenAlex Direct Topic & Taxonomy URLs
	if url, _ := buildOpenAlexTopicURL("https://openalex.org/T10123", "Electromagnetic Theory"); url != "https://openalex.org/topics/T10123" {
		t.Errorf("expected https://openalex.org/topics/T10123, got %s", url)
	}
	if url := buildOpenAlexSubfieldURL("https://openalex.org/subfields/1702", "Artificial Intelligence"); url != "https://openalex.org/subfields/1702" {
		t.Errorf("expected https://openalex.org/subfields/1702, got %s", url)
	}
	if url := buildOpenAlexFieldURL("https://openalex.org/fields/17", "Computer Science"); url != "https://openalex.org/fields/17" {
		t.Errorf("expected https://openalex.org/fields/17, got %s", url)
	}
	if url := buildOpenAlexDomainURL("https://openalex.org/domains/3", "Physical Sciences"); url != "https://openalex.org/domains/3" {
		t.Errorf("expected https://openalex.org/domains/3, got %s", url)
	}

	// OpenAlex Types
	if url := buildOpenAlexTypeURL("article"); url != "https://openalex.org/types/article" {
		t.Errorf("expected https://openalex.org/types/article, got %s", url)
	}
	// OpenAlex InstitutionTypes
	if url := buildOpenAlexInstitutionTypeURL("education"); url != "https://openalex.org/institution-types/education" {
		t.Errorf("expected https://openalex.org/institution-types/education, got %s", url)
	}
	// OpenAlex SourceTypes
	if url := buildOpenAlexSourceTypeURL("repository"); url != "https://openalex.org/source-types/repository" {
		t.Errorf("expected https://openalex.org/source-types/repository, got %s", url)
	}
	// OpenAlex Indexes
	if url := buildOpenAlexIndexURL("crossref"); url != "https://openalex.org/indexes/crossref" {
		t.Errorf("expected https://openalex.org/indexes/crossref, got %s", url)
	}
	// OpenAlex SourceLists
	if url := buildOpenAlexSourceListURL("cwts-core"); url != "https://openalex.org/source-lists/cwts-core" {
		t.Errorf("expected https://openalex.org/source-lists/cwts-core, got %s", url)
	}
	// OpenAlex Publishers
	if url := buildOpenAlexPublisherURL("P4310320990", ""); url != "https://openalex.org/publishers/P4310320990" {
		t.Errorf("expected https://openalex.org/publishers/P4310320990, got %s", url)
	}
	if url := buildOpenAlexPublisherURL("", "Springer Nature"); url != "https://openalex.org/publishers?search=Springer+Nature" {
		t.Errorf("expected https://openalex.org/publishers?search=Springer+Nature, got %s", url)
	}
}

func TestDataspaceService_OpenAlexDirectMatching(t *testing.T) {
	mockSearch := func(_ context.Context, barcode string) ([]catalog.Book, error) {
		return []catalog.Book{
			{
				ID: "oai:alma.41SLSP_FNW:990049881560205518",
				Titles: []catalog.Title{
					{Title: "Heinrich Hertz: Erinnerungen, Briefe, Tagebücher"},
				},
				ISBNs: []catalog.ISBN{
					{Enriched: "978-3-8171-3420-1"},
				},
				PersonalNames: []map[string]catalog.Name{
					{"aut": {NamePart: "Fölsing, Albrecht"}},
				},
			},
		}, nil
	}

	mockOpenAlex := func(_ context.Context, isbns []string, title string, authors []string, dois []string) ([]OpenAlexWork, error) {
		return []OpenAlexWork{
			{
				ID:              "https://openalex.org/W642892681",
				DisplayName:     "Heinrich Hertz und die elektromagnetischen Wellen",
				PublicationYear: 1997,
				Type:            "article",
				Authorships: []OpenAlexAuthorship{
					{
						Author: OpenAlexAuthorRef{
							ID:          "https://openalex.org/A5090064378",
							DisplayName: "Albrecht Fölsing",
							ORCID:       "https://orcid.org/0000-0002-1825-0097",
						},
						Institutions: []OpenAlexInstitutionRef{
							{
								ID:          "https://openalex.org/I118019719",
								DisplayName: "University of Hamburg",
							},
						},
					},
				},
				PrimaryLocation: &OpenAlexLocation{
					Source: &OpenAlexSourceRef{
						ID:               "https://openalex.org/S7407052672",
						DisplayName:      "Physikalische Blätter",
						HostOrganization: "https://openalex.org/P4310320990",
						HostOrgName:      "Wiley-VCH",
					},
				},
				PrimaryTopic: &OpenAlexTopicRef{
					ID:          "https://openalex.org/T10123",
					DisplayName: "Electromagnetic Theory",
					Subfield: &OpenAlexTaxonomyRef{
						ID:          "https://openalex.org/subfields/3104",
						DisplayName: "Condensed Matter Physics",
					},
					Field: &OpenAlexTaxonomyRef{
						ID:          "https://openalex.org/fields/31",
						DisplayName: "Physics and Astronomy",
					},
					Domain: &OpenAlexTaxonomyRef{
						ID:          "https://openalex.org/domains/3",
						DisplayName: "Physical Sciences",
					},
				},
			},
		}, nil
	}

	svc := newDataspaceService(mockSearch, nil, mockOpenAlex)
	resp, err := svc.GenerateMermaid(context.Background(), []string{"0001847294"}, "TD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify exact direct OpenAlex deep links
	if !strings.Contains(resp.Mermaid, "https://openalex.org/works/W642892681") {
		t.Errorf("expected direct OpenAlex Work URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/authors/A5090064378") {
		t.Errorf("expected direct OpenAlex Author URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/institutions/I118019719") {
		t.Errorf("expected direct OpenAlex Institution URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/sources/S7407052672") {
		t.Errorf("expected direct OpenAlex Source URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/publishers/P4310320990") {
		t.Errorf("expected direct OpenAlex Publisher URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/topics/T10123") {
		t.Errorf("expected direct OpenAlex Topic URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/subfields/3104") {
		t.Errorf("expected direct OpenAlex Subfield URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/fields/31") {
		t.Errorf("expected direct OpenAlex Field URL, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "https://openalex.org/domains/3") {
		t.Errorf("expected direct OpenAlex Domain URL, got: %s", resp.Mermaid)
	}
}

func TestDataspaceService_MultiBook(t *testing.T) {
	mockSearch := func(_ context.Context, barcode string) ([]catalog.Book, error) {
		switch barcode {
		case "BC-01":
			return []catalog.Book{
				{
					Titles: []catalog.Title{{Title: "Quantum Mechanics Vol 1"}},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Dirac, Paul"}},
					},
				},
			}, nil
		case "BC-02":
			return []catalog.Book{
				{
					Titles: []catalog.Title{{Title: "Principles of Quantum Mechanics"}},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Dirac, Paul"}},
					},
				},
			}, nil
		default:
			return nil, nil
		}
	}

	svc := newDataspaceService(mockSearch, nil, nil)
	resp, err := svc.GenerateMermaid(context.Background(), []string{"BC-01", "BC-02"}, "LR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ItemCount != 2 {
		t.Errorf("expected 2 items, got %d", resp.ItemCount)
	}
	if !strings.HasPrefix(resp.Mermaid, "flowchart LR") {
		t.Errorf("expected flowchart LR prefix, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "Dirac, Paul") {
		t.Errorf("expected shared author Dirac, Paul in Mermaid")
	}
}

func TestDataspaceEndpoint(t *testing.T) {
	ctrl, err := NewController(Config{
		Hub:      desk.NewHub(10),
		Channels: []string{"mediathek01"},
		CatalogSearch: func(_ context.Context, barcode string) ([]catalog.Book, error) {
			return []catalog.Book{
				{Titles: []catalog.Title{{Title: "Test Book"}}},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Request /dataspace/graph
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dataspace/graph?barcodes=123,456&direction=TD", nil)
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "flowchart TD") {
		t.Errorf("expected flowchart TD in response, got %s", body)
	}
	if !strings.Contains(body, "Test Book") {
		t.Errorf("expected Test Book in response, got %s", body)
	}
}

func TestDataspaceService_Report(t *testing.T) {
	mockSearch := func(_ context.Context, barcode string) ([]catalog.Book, error) {
		switch barcode {
		case "BC-01":
			return []catalog.Book{
				{
					ID: "oai:alma.41SLSP_FNW:990049881560205518",
					Titles: []catalog.Title{
						{Title: "Heinrich Hertz: Erinnerungen, Briefe, Tagebücher", SubTitle: "Dokumente und Briefwechsel"},
					},
					Responsibility: []catalog.Note{
						{Main: "Hrsg. von Albrecht Fölsing"},
					},
					ISBNs: []catalog.ISBN{
						{Enriched: "978-3-8171-3420-1"},
					},
					Publications: []catalog.Publication{
						{Publisher: []string{"Harri Deutsch"}, DateString: "1997"},
					},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Fölsing, Albrecht", Identifier: "(DE-588)118772295"}},
					},
					Holdings: []catalog.Holding{
						{CallNumber: "530.092 HER", Library: "FHNW Campus Muttenz"},
					},
					Subjects: catalog.Subject{
						Topic: []map[string]catalog.SubjectTerm{
							{"top": {Label: "Electromagnetism", Identifier: "(DE-588)4002851-3"}},
						},
					},
				},
			}, nil
		case "BC-02":
			return []catalog.Book{
				{
					ID: "oai:alma.41SLSP_FNW:990049881560205519",
					Titles: []catalog.Title{
						{Title: "Albert Einstein: Biographie"},
					},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Fölsing, Albrecht", Identifier: "(DE-588)118772295"}},
					},
					Subjects: catalog.Subject{
						Topic: []map[string]catalog.SubjectTerm{
							{"top": {Label: "Physics", Identifier: "(DE-588)4045956-1"}},
						},
					},
				},
			}, nil
		default:
			return nil, nil
		}
	}

	mockInk := func(_ context.Context, barcode string, authors []string, title string, isbns []string) ([]InkItem, error) {
		if barcode == "BC-01" {
			return []InkItem{
				{
					Signature:       "zotero2-2068924.UHBBY3JP",
					Title:           "Classic Design – Future Design Classics",
					Type:            "thesis",
					CollectionTitle: "Archiv Institut Industrial Design",
					Date:            "2021",
				},
			}, nil
		}
		return nil, nil
	}

	mockOpenAlex := func(_ context.Context, isbns []string, title string, authors []string, dois []string) ([]OpenAlexWork, error) {
		if len(isbns) > 0 && isbns[0] == "978-3-8171-3420-1" {
			return []OpenAlexWork{
				{
					ID:              "https://openalex.org/W642892681",
					DisplayName:     "Heinrich Hertz und die elektromagnetischen Wellen",
					PublicationYear: 1997,
					Type:            "article",
					Authorships: []OpenAlexAuthorship{
						{
							Author: OpenAlexAuthorRef{
								ID:          "https://openalex.org/A5090064378",
								DisplayName: "Albrecht Fölsing",
							},
							Institutions: []OpenAlexInstitutionRef{
								{
									ID:          "https://openalex.org/I118019719",
									DisplayName: "University of Hamburg",
								},
							},
						},
					},
					PrimaryTopic: &OpenAlexTopicRef{
						ID:          "https://openalex.org/T10123",
						DisplayName: "Electromagnetic Theory",
						Subfield: &OpenAlexTaxonomyRef{
							ID:          "https://openalex.org/subfields/3104",
							DisplayName: "Condensed Matter Physics",
						},
						Field: &OpenAlexTaxonomyRef{
							ID:          "https://openalex.org/fields/31",
							DisplayName: "Physics and Astronomy",
						},
						Domain: &OpenAlexTaxonomyRef{
							ID:          "https://openalex.org/domains/3",
							DisplayName: "Physical Sciences",
						},
					},
				},
			}, nil
		}
		return nil, nil
	}

	svc := newDataspaceService(mockSearch, mockInk, mockOpenAlex)
	report, err := svc.GenerateReport(context.Background(), []string{"BC-01", "BC-02"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.ItemCount != 2 {
		t.Errorf("expected 2 items, got %d", report.ItemCount)
	}
	if report.Summary.CatalogHits != 2 {
		t.Errorf("expected 2 catalog hits, got %d", report.Summary.CatalogHits)
	}
	if report.Summary.InkHits != 1 {
		t.Errorf("expected 1 InK hit, got %d", report.Summary.InkHits)
	}
	if report.Summary.OpenAlexHits != 1 {
		t.Errorf("expected 1 OpenAlex hit, got %d", report.Summary.OpenAlexHits)
	}
	if len(report.SharedLinks) == 0 {
		t.Errorf("expected shared links between BC-01 and BC-02 for shared author Fölsing")
	}

	// Verify item 0 details
	it0 := report.Items[0]
	if it0.Barcode != "BC-01" {
		t.Errorf("expected BC-01, got %s", it0.Barcode)
	}
	if it0.Title != "Heinrich Hertz: Erinnerungen, Briefe, Tagebücher" {
		t.Errorf("unexpected title: %s", it0.Title)
	}
	if it0.Subtitle != "Dokumente und Briefwechsel" {
		t.Errorf("unexpected subtitle: %s", it0.Subtitle)
	}
	if it0.Responsibility != "Hrsg. von Albrecht Fölsing" {
		t.Errorf("unexpected responsibility: %s", it0.Responsibility)
	}
	if it0.CallNumber != "530.092 HER" {
		t.Errorf("unexpected call number: %s", it0.CallNumber)
	}
	if len(it0.InkItems) != 1 || it0.InkItems[0].Signature != "zotero2-2068924.UHBBY3JP" {
		t.Errorf("expected InK item attached to item 0")
	}
	if len(it0.OpenAlexWorks) != 1 || it0.OpenAlexWorks[0].DisplayName != "Heinrich Hertz und die elektromagnetischen Wellen" {
		t.Errorf("expected OpenAlex work attached to item 0")
	}
}

func TestDataspaceReportEndpoint(t *testing.T) {
	ctrl, err := NewController(Config{
		Hub:      desk.NewHub(10),
		Channels: []string{"mediathek01"},
		CatalogSearch: func(_ context.Context, barcode string) ([]catalog.Book, error) {
			return []catalog.Book{
				{
					Titles: []catalog.Title{{Title: "Test Report Book"}},
					ISBNs:  []catalog.ISBN{{Enriched: "978-0-123456-47-2"}},
				},
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Request /dataspace/report
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dataspace/report?barcodes=123", nil)
	ctrl.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Test Report Book") {
		t.Errorf("expected Test Report Book in response, got %s", body)
	}
	if !strings.Contains(body, "978-0-123456-47-2") {
		t.Errorf("expected ISBN in response, got %s", body)
	}
}
