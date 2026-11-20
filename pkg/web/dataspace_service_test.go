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
	svc := newDataspaceService(nil, nil)
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

	svc := newDataspaceService(mockSearch, mockInk)
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

	svc := newDataspaceService(mockSearch, nil)
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
