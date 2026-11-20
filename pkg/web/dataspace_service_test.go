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
	svc := newDataspaceService(nil, nil, nil, nil, nil)
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

	svc := newDataspaceService(mockSearch, nil, mockInk, nil, nil)
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

	svc := newDataspaceService(mockSearch, nil, nil, mockOpenAlex, nil)
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

	svc := newDataspaceService(mockSearch, nil, nil, nil, nil)
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

	svc := newDataspaceService(mockSearch, nil, mockInk, mockOpenAlex, nil)
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

func TestGenericUtilities(t *testing.T) {
	// cleanAndDeduplicate
	ints := cleanAndDeduplicate([]int{1, 2, 2, 3, 1, 4, 3})
	if len(ints) != 4 || ints[0] != 1 || ints[1] != 2 || ints[2] != 3 || ints[3] != 4 {
		t.Errorf("unexpected dedup ints: %v", ints)
	}

	// cleanStrings
	strs := cleanStrings([]string{"  a ", "", "b", "  ", "a", "c", "b "})
	if len(strs) != 3 || strs[0] != "a" || strs[1] != "b" || strs[2] != "c" {
		t.Errorf("unexpected cleanStrings: %v", strs)
	}

	// mapSlice
	mapped := mapSlice([]int{1, 2, 3}, func(x int) int { return x * 10 })
	if len(mapped) != 3 || mapped[0] != 10 || mapped[1] != 20 || mapped[2] != 30 {
		t.Errorf("unexpected mapSlice: %v", mapped)
	}

	// filterSlice
	filtered := filterSlice([]int{1, 2, 3, 4, 5, 6}, func(x int) bool { return x%2 == 0 })
	if len(filtered) != 3 || filtered[0] != 2 || filtered[1] != 4 || filtered[2] != 6 {
		t.Errorf("unexpected filterSlice: %v", filtered)
	}

	// topKeysByFrequency
	freq := map[string]int{
		"Physics":          10,
		"Computer Science": 25,
		"Mathematics":      15,
		"Biology":          5,
	}
	top2 := topKeysByFrequency(freq, 2)
	if len(top2) != 2 || top2[0] != "Computer Science" || top2[1] != "Mathematics" {
		t.Errorf("unexpected topKeysByFrequency: %v", top2)
	}
}

func TestCatalogExtractionHelpers(t *testing.T) {
	book := &catalog.Book{
		ID: "oai:alma.41SLSP_FNW:123456",
		Titles: []catalog.Title{
			{Title: "Quantum Mechanics", SubTitle: "A Modern Development"},
		},
		ISBNs: []catalog.ISBN{
			{Enriched: "978-981-02-4105-6"},
			{ID: "9810241054"},
		},
		Publications: []catalog.Publication{
			{
				DateString: "2000",
				Publisher:  []string{"World Scientific", "Singapore"},
			},
		},
		Identifiers: []string{
			"doi:10.1142/4113",
			"other-identifier",
		},
		Holdings: []catalog.Holding{
			{CallNumber: "530.12 BAL"},
		},
		PersonalNames: []map[string]catalog.Name{
			{"aut": {NamePart: "Ballentine, Leslie E.", Identifier: "(DE-588)1145355609"}},
		},
		Subjects: catalog.Subject{
			Topic: []map[string]catalog.SubjectTerm{
				{"top": {Label: "Quantum theory", Identifier: "(DE-588)4047992-4"}},
			},
		},
	}

	title, subtitle := extractBookTitle(book, "fallback")
	if title != "Quantum Mechanics" || subtitle != "A Modern Development" {
		t.Errorf("unexpected title/subtitle: %s, %s", title, subtitle)
	}

	primaryISBN := extractPrimaryISBN(book)
	if primaryISBN != "978-981-02-4105-6" {
		t.Errorf("unexpected primary ISBN: %s", primaryISBN)
	}

	allISBNs := extractAllISBNs(book)
	if len(allISBNs) != 2 || allISBNs[0] != "978-981-02-4105-6" || allISBNs[1] != "9810241054" {
		t.Errorf("unexpected all ISBNs: %v", allISBNs)
	}

	callNum := extractBookCallNumber(book)
	if callNum != "530.12 BAL" {
		t.Errorf("unexpected call number: %s", callNum)
	}

	pubYear, pubName := extractBookPublisher(book)
	if pubYear != "2000" || pubName != "World Scientific, Singapore" {
		t.Errorf("unexpected publisher: %s, %s", pubYear, pubName)
	}

	dois := extractBookDOIs(book)
	if len(dois) != 1 || dois[0] != "doi:10.1142/4113" {
		t.Errorf("unexpected dois: %v", dois)
	}

	authors := extractCatalogAuthors(book)
	if len(authors) != 1 || authors[0].Name != "Ballentine, Leslie E." || authors[0].GNDID != "1145355609" {
		t.Errorf("unexpected authors: %+v", authors)
	}

	topics := extractCatalogTopics(book)
	if len(topics) != 1 || topics[0].Label != "Quantum theory" || topics[0].Identifier != "(DE-588)4047992-4" {
		t.Errorf("unexpected topics: %+v", topics)
	}
}

func TestDataspaceService_CitedBy_Phases1To3(t *testing.T) {
	mockSearch := func(_ context.Context, barcode string) ([]catalog.Book, error) {
		switch barcode {
		case "BC-CITE-1":
			return []catalog.Book{
				{
					Titles: []catalog.Title{{Title: "Electrodynamics of Moving Bodies"}},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Einstein, Albert"}},
					},
				},
			}, nil
		case "BC-CITE-2":
			return []catalog.Book{
				{
					Titles: []catalog.Title{{Title: "Electric Waves"}},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Hertz, Heinrich"}},
					},
				},
			}, nil
		}
		return nil, nil
	}

	mockOpenAlex := func(_ context.Context, isbns []string, title string, authors []string, dois []string) ([]OpenAlexWork, error) {
		if strings.Contains(title, "Electrodynamics") {
			return []OpenAlexWork{
				{
					ID:              "https://openalex.org/W1001",
					DisplayName:     "Zur Elektrodynamik bewegter Körper",
					PublicationYear: 1905,
					CitedByCount:    4500,
					ReferencedWorks: []string{"https://openalex.org/W2002"}, // References Hertz
				},
			}, nil
		}
		if strings.Contains(title, "Electric Waves") {
			return []OpenAlexWork{
				{
					ID:              "https://openalex.org/W2002",
					DisplayName:     "Untersuchungen über die Ausbreitung der elektrischen Kraft",
					PublicationYear: 1892,
					CitedByCount:    1200,
				},
			}, nil
		}
		return nil, nil
	}

	mockCitingSearch := func(_ context.Context, workID string, limit int) ([]OpenAlexWork, error) {
		if strings.Contains(workID, "W2002") {
			return []OpenAlexWork{
				{
					ID:              "https://openalex.org/W3003",
					DisplayName:     "Experimental Foundations of Quantum Optics",
					PublicationYear: 1980,
					CitedByCount:    350,
				},
			}, nil
		}
		return nil, nil
	}

	svc := newDataspaceService(mockSearch, nil, nil, mockOpenAlex, mockCitingSearch)

	// 1. Test Graph Generation
	resp, err := svc.GenerateMermaid(context.Background(), []string{"BC-CITE-1", "BC-CITE-2"}, "TD")
	if err != nil {
		t.Fatalf("graph generation failed: %v", err)
	}

	// Verify Phase 1: Citation count badge in node & tooltip
	if !strings.Contains(resp.Mermaid, "[4500 Citations]") {
		t.Errorf("expected citation count badge in graph, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "4500 Citations") {
		t.Errorf("expected citation count in tooltip, got: %s", resp.Mermaid)
	}

	// Verify Phase 2: Direct citation edge from W1001 to W2002
	if !strings.Contains(resp.Mermaid, "==>|\"CITES WORK\"|") {
		t.Errorf("expected CITES WORK edge between scanned items, got: %s", resp.Mermaid)
	}

	// Verify Phase 3: Citing work node and incoming citation edge
	if !strings.Contains(resp.Mermaid, "Cited in: Experimental Foundations") {
		t.Errorf("expected citing work node, got: %s", resp.Mermaid)
	}
	if !strings.Contains(resp.Mermaid, "==>|\"CITES\"|") {
		t.Errorf("expected CITES edge from citing work node, got: %s", resp.Mermaid)
	}

	// 2. Test Report Generation
	report, err := svc.GenerateReport(context.Background(), []string{"BC-CITE-1", "BC-CITE-2"})
	if err != nil {
		t.Fatalf("report generation failed: %v", err)
	}

	// Phase 1 verification in report
	if report.Summary.TotalCitations != 5700 {
		t.Errorf("expected total citations 5700, got %d", report.Summary.TotalCitations)
	}
	if len(report.Items[0].OpenAlexWorks) == 0 || report.Items[0].OpenAlexWorks[0].CitedByCount != 4500 {
		t.Errorf("expected item 0 CitedByCount 4500")
	}
	if !strings.Contains(report.Items[0].OpenAlexWorks[0].CitedByURL, "filter=cites:W1001") {
		t.Errorf("expected filter=cites:W1001 in CitedByURL, got: %s", report.Items[0].OpenAlexWorks[0].CitedByURL)
	}

	// Phase 2 verification in report: SharedLinks includes Cites Work
	foundCiteLink := false
	for _, link := range report.SharedLinks {
		if link.RelationType == "Cites Work" {
			foundCiteLink = true
			break
		}
	}
	if !foundCiteLink {
		t.Errorf("expected Cites Work relation in report SharedLinks, got: %+v", report.SharedLinks)
	}

	// Phase 3 verification in report: CitingWorks populated
	if len(report.Items[1].OpenAlexWorks) == 0 || len(report.Items[1].OpenAlexWorks[0].CitingWorks) == 0 {
		t.Errorf("expected citing works populated on item 1")
	} else {
		cw := report.Items[1].OpenAlexWorks[0].CitingWorks[0]
		if cw.DisplayName != "Experimental Foundations of Quantum Optics" {
			t.Errorf("unexpected citing work title: %s", cw.DisplayName)
		}
		if cw.CitedByCount != 350 {
			t.Errorf("unexpected citing work cited_by_count: %d", cw.CitedByCount)
		}
	}
}

func TestDataspaceService_AlmaReverseMatchForCitingWorks(t *testing.T) {
	mockSearch := func(_ context.Context, barcode string) ([]catalog.Book, error) {
		return []catalog.Book{
			{
				ID:     "oai:alma.41SLSP_FNW:99001",
				Titles: []catalog.Title{{Title: "Principles of Optics"}},
				PersonalNames: []map[string]catalog.Name{
					{"aut": {NamePart: "Born, Max"}},
				},
			},
		}, nil
	}

	mockOpenAlex := func(_ context.Context, isbns []string, title string, authors []string, dois []string) ([]OpenAlexWork, error) {
		return []OpenAlexWork{
			{
				ID:              "https://openalex.org/W100",
				DisplayName:     "Principles of Optics",
				PublicationYear: 1959,
				CitedByCount:    10000,
			},
		}, nil
	}

	mockCitingSearch := func(_ context.Context, workID string, limit int) ([]OpenAlexWork, error) {
		return []OpenAlexWork{
			{
				ID:              "https://openalex.org/W200",
				DOI:             "https://doi.org/10.1000/182",
				DisplayName:     "Quantum Theory of Optical Coherence",
				PublicationYear: 1963,
				CitedByCount:    3200,
				Authorships: []OpenAlexAuthorship{
					{Author: OpenAlexAuthorRef{DisplayName: "Roy J. Glauber"}},
				},
			},
		}, nil
	}

	mockAlmaWorkSearch := func(_ context.Context, dois []string, isbns []string, title string, authors []string) ([]catalog.Book, error) {
		if strings.Contains(title, "Optical Coherence") || (len(dois) > 0 && strings.Contains(dois[0], "10.1000/182")) {
			return []catalog.Book{
				{
					ID:     "oai:alma.41SLSP_FNW:9900223344",
					Titles: []catalog.Title{{Title: "Quantum Theory of Optical Coherence"}},
					Holdings: []catalog.Holding{
						{
							Library:    "FHNW Campus Muttenz",
							CallNumber: "535.2 GLA",
						},
					},
				},
			}, nil
		}
		return nil, nil
	}

	svc := newDataspaceService(mockSearch, mockAlmaWorkSearch, nil, mockOpenAlex, mockCitingSearch)

	// 1. Verify Graph Generation with Alma Citing node
	graphResp, err := svc.GenerateMermaid(context.Background(), []string{"0001"}, "TD")
	if err != nil {
		t.Fatalf("graph generation failed: %v", err)
	}

	if !strings.Contains(graphResp.Mermaid, "[Alma: 535.2 GLA]") {
		t.Errorf("expected [Alma: 535.2 GLA] in citing work label, got: %s", graphResp.Mermaid)
	}
	if !strings.Contains(graphResp.Mermaid, "Library Inventory: Quantum Theory") {
		t.Errorf("expected Library Inventory node in graph, got: %s", graphResp.Mermaid)
	}
	if !strings.Contains(graphResp.Mermaid, "-.->|\"in library catalog\"|") {
		t.Errorf("expected 'in library catalog' edge in graph, got: %s", graphResp.Mermaid)
	}
	if !strings.Contains(graphResp.Mermaid, "https://swisscovery.ch/discovery/search?query=any,contains,9900223344") {
		t.Errorf("expected Swisscovery link for citing work in graph, got: %s", graphResp.Mermaid)
	}

	// 2. Verify Report Generation with InLibraryCollection and Alma metadata
	reportResp, err := svc.GenerateReport(context.Background(), []string{"0001"})
	if err != nil {
		t.Fatalf("report generation failed: %v", err)
	}

	if len(reportResp.Items) != 1 || len(reportResp.Items[0].OpenAlexWorks) != 1 || len(reportResp.Items[0].OpenAlexWorks[0].CitingWorks) != 1 {
		t.Fatalf("expected 1 citing work in report")
	}

	cwRep := reportResp.Items[0].OpenAlexWorks[0].CitingWorks[0]
	if !cwRep.InLibraryCollection {
		t.Errorf("expected InLibraryCollection to be true")
	}
	if cwRep.AlmaMMSID != "9900223344" {
		t.Errorf("expected AlmaMMSID 9900223344, got %s", cwRep.AlmaMMSID)
	}
	if cwRep.AlmaCallNumber != "535.2 GLA" {
		t.Errorf("expected AlmaCallNumber '535.2 GLA', got %s", cwRep.AlmaCallNumber)
	}
	if cwRep.AlmaLibrary != "FHNW Campus Muttenz" {
		t.Errorf("expected AlmaLibrary 'FHNW Campus Muttenz', got %s", cwRep.AlmaLibrary)
	}
	if !strings.Contains(cwRep.AlmaSwisscoveryURL, "9900223344") {
		t.Errorf("expected AlmaSwisscoveryURL with MMS-ID, got %s", cwRep.AlmaSwisscoveryURL)
	}
}

func TestDataspaceService_CoCitationInReportAndGraph(t *testing.T) {
	mockSearch := func(_ context.Context, barcode string) ([]catalog.Book, error) {
		if barcode == "BARCODE_A" {
			return []catalog.Book{
				{
					ID:     "oai:alma.41SLSP_FNW:99001",
					Titles: []catalog.Title{{Title: "Quantum Mechanics"}},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Dirac, Paul"}},
					},
				},
			}, nil
		}
		if barcode == "BARCODE_B" {
			return []catalog.Book{
				{
					ID:     "oai:alma.41SLSP_FNW:99002",
					Titles: []catalog.Title{{Title: "Information Theory"}},
					PersonalNames: []map[string]catalog.Name{
						{"aut": {NamePart: "Shannon, Claude"}},
					},
				},
			}, nil
		}
		return nil, nil
	}

	mockOpenAlex := func(_ context.Context, isbns []string, title string, authors []string, dois []string) ([]OpenAlexWork, error) {
		if strings.Contains(title, "Quantum") {
			return []OpenAlexWork{
				{
					ID:              "https://openalex.org/W101",
					DisplayName:     "The Principles of Quantum Mechanics",
					PublicationYear: 1930,
					CitedByCount:    25000,
				},
			}, nil
		}
		if strings.Contains(title, "Information") {
			return []OpenAlexWork{
				{
					ID:              "https://openalex.org/W102",
					DisplayName:     "A Mathematical Theory of Communication",
					PublicationYear: 1948,
					CitedByCount:    80000,
				},
			}, nil
		}
		return nil, nil
	}

	// Both W101 and W102 are cited by W999
	mockCitingSearch := func(_ context.Context, workID string, limit int) ([]OpenAlexWork, error) {
		return []OpenAlexWork{
			{
				ID:              "https://openalex.org/W999",
				DisplayName:     "Quantum Computation and Quantum Information",
				PublicationYear: 2000,
				CitedByCount:    50000,
			},
		}, nil
	}

	svc := newDataspaceService(mockSearch, nil, nil, mockOpenAlex, mockCitingSearch)

	// Verify Report contains Co-Citation SharedLink
	report, err := svc.GenerateReport(context.Background(), []string{"BARCODE_A", "BARCODE_B"})
	if err != nil {
		t.Fatalf("report generation failed: %v", err)
	}

	if len(report.SharedLinks) == 0 {
		t.Fatalf("expected at least 1 shared link for co-citation, got 0")
	}

	foundCoCitation := false
	for _, link := range report.SharedLinks {
		if link.RelationType == "Co-Citation" {
			foundCoCitation = true
			if !strings.Contains(link.Description, "Quantum Computation and Quantum Information") {
				t.Errorf("unexpected co-citation description: %s", link.Description)
			}
		}
	}
	if !foundCoCitation {
		t.Errorf("expected Co-Citation relation type in SharedLinks")
	}

	// Verify Graph merges citing node for both books
	graph, err := svc.GenerateMermaid(context.Background(), []string{"BARCODE_A", "BARCODE_B"}, "TD")
	if err != nil {
		t.Fatalf("graph generation failed: %v", err)
	}

	if !strings.Contains(graph.Mermaid, "Citing_W999") {
		t.Errorf("expected Citing_W999 node in graph, got: %s", graph.Mermaid)
	}
	// Check that Citing_W999 points to both works
	if !strings.Contains(graph.Mermaid, "==>|\"CITES\"| OAWork_1_1") {
		t.Errorf("expected Citing_W999 to cite OAWork_1_1, got: %s", graph.Mermaid)
	}
	if !strings.Contains(graph.Mermaid, "==>|\"CITES\"| OAWork_2_1") {
		t.Errorf("expected Citing_W999 to cite OAWork_2_1, got: %s", graph.Mermaid)
	}
}
