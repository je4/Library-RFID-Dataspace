package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestElasticClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "APIKey test-key" {
			t.Errorf("unexpected authorization header: %q", got)
		}
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"test","cluster_name":"test","cluster_uuid":"test","version":{"number":"8.19.7"},"tagline":"You Know, for Search"}`))
	}))
	defer server.Close()

	t.Setenv("TEST_ELASTIC_APIKEY", "test-key")
	path := filepath.Join(t.TempDir(), "dataspace.toml")
	data := "[elasticsearch]\nendpoint = [\"" + server.URL + "\"]\nindex = \"test-index\"\napikey = \"%%TEST_ELASTIC_APIKEY%%\"\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	conf, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if conf.ElasticSearch.Index != "test-index" {
		t.Fatalf("unexpected index: %q", conf.ElasticSearch.Index)
	}
	client, err := newElasticClient(conf.ElasticSearch)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Info().Do(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestElasticClientDisabled(t *testing.T) {
	client, err := newElasticClient(ElasticSearchConfig{})
	if err != nil || client != nil {
		t.Fatalf("expected no client and no error, got %v, %v", client, err)
	}
}

func TestSearchInkItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		responseJSON := `{
			"took": 2,
			"timed_out": false,
			"_shards": {"total": 1, "successful": 1, "skipped": 0, "failed": 0},
			"hits": {
				"total": {"value": 1, "relation": "eq"},
				"max_score": 5.0,
				"hits": [
					{
						"_index": "fhnw_ink",
						"_id": "forms2-2018.971913",
						"_score": 5.0,
						"_source": {
							"signature": "forms2-2018.971913",
							"title": "Genderneutrales Design",
							"type": "thesis",
							"collectiontitle": "Diplom HGK",
							"persons": [{"name": "Benjamin Frank Neuenschwander", "role": "creator"}],
							"acl": {"meta": ["global/guest"]}
						}
					}
				]
			}
		}`
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer server.Close()

	client, err := newElasticClient(ElasticSearchConfig{
		Endpoint: []string{server.URL},
		APIKey:   "dummy",
	})
	if err != nil {
		t.Fatal(err)
	}

	items, err := searchInkItems(context.Background(), client, "fhnw_ink", "12345", []string{"Neuenschwander"}, "Genderneutrales Design", []string{"978-3-00-000000-0"})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].Title != "Genderneutrales Design" {
		t.Errorf("expected title 'Genderneutrales Design', got %q", items[0].Title)
	}
	if items[0].CollectionTitle != "Diplom HGK" {
		t.Errorf("expected collection 'Diplom HGK', got %q", items[0].CollectionTitle)
	}
}

func TestSearchOpenAlexWorks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		responseJSON := `{
			"took": 3,
			"timed_out": false,
			"_shards": {"total": 1, "successful": 1, "skipped": 0, "failed": 0},
			"hits": {
				"total": {"value": 1, "relation": "eq"},
				"max_score": 4.5,
				"hits": [
					{
						"_index": "openalex-works",
						"_id": "W642892681",
						"_score": 4.5,
						"_source": {
							"id": "https://openalex.org/W642892681",
							"doi": "https://doi.org/10.1002/phbl.19970530707",
							"display_name": "Heinrich Hertz und die elektromagnetischen Wellen",
							"publication_year": 1997,
							"type": "article",
							"authorships": [
								{
									"author": {
										"id": "https://openalex.org/A5090064378",
										"display_name": "Albrecht Fölsing",
										"orcid": "https://orcid.org/0000-0002-1825-0097"
									},
									"institutions": [
										{
											"id": "https://openalex.org/I118019719",
											"display_name": "University of Hamburg"
										}
									]
								}
							],
							"primary_location": {
								"source": {
									"id": "https://openalex.org/S7407052672",
									"display_name": "Physikalische Blätter",
									"host_organization": "https://openalex.org/P4310320990",
									"host_organization_name": "Wiley-VCH"
								}
							},
							"primary_topic": {
								"id": "https://openalex.org/T10123",
								"display_name": "Electromagnetic Theory",
								"subfield": {
									"id": "https://openalex.org/subfields/3104",
									"display_name": "Condensed Matter Physics"
								},
								"field": {
									"id": "https://openalex.org/fields/31",
									"display_name": "Physics and Astronomy"
								},
								"domain": {
									"id": "https://openalex.org/domains/3",
									"display_name": "Physical Sciences"
								}
							}
						}
					}
				]
			}
		}`
		_, _ = w.Write([]byte(responseJSON))
	}))
	defer server.Close()

	client, err := newElasticClient(ElasticSearchConfig{
		Endpoint: []string{server.URL},
		APIKey:   "dummy",
	})
	if err != nil {
		t.Fatal(err)
	}

	works, err := searchOpenAlexWorks(context.Background(), client, "openalex-works", []string{"978-3-8171-3420-1"}, "Heinrich Hertz", []string{"Fölsing"}, []string{"10.1002/phbl.19970530707"})
	if err != nil {
		t.Fatalf("unexpected search error: %v", err)
	}

	if len(works) != 1 {
		t.Fatalf("expected 1 work, got %d", len(works))
	}
	if works[0].ID != "https://openalex.org/W642892681" {
		t.Errorf("expected ID 'https://openalex.org/W642892681', got %q", works[0].ID)
	}
	if works[0].DisplayName != "Heinrich Hertz und die elektromagnetischen Wellen" {
		t.Errorf("expected title 'Heinrich Hertz und die elektromagnetischen Wellen', got %q", works[0].DisplayName)
	}
	if len(works[0].Authorships) != 1 || works[0].Authorships[0].Author.ID != "https://openalex.org/A5090064378" {
		t.Errorf("expected author ID 'https://openalex.org/A5090064378'")
	}
	if works[0].PrimaryTopic == nil || works[0].PrimaryTopic.ID != "https://openalex.org/T10123" {
		t.Errorf("expected primary topic ID 'https://openalex.org/T10123'")
	}
}
