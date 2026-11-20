package main

import (
	"context"
	"encoding/json"
	"strings"

	"emperror.dev/errors"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/sortorder"
	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/je4/Library-RFID-Dataspace/pkg/web"
)

func newElasticClient(conf ElasticSearchConfig) (*elasticsearch.TypedClient, error) {
	if len(conf.Endpoint) == 0 {
		return nil, nil
	}

	return elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses:     conf.Endpoint,
		APIKey:        string(conf.APIKey),
		RetryOnStatus: []int{502, 503, 504, 429},
		MaxRetries:    5,
	})
}

// searchInkItems queries the fhnw_ink index for records matching the scanned item,
// strictly applying the guest ACL filter (acl.meta = "global/guest").
func searchInkItems(ctx context.Context, client *elasticsearch.TypedClient, index string, barcode string, authors []string, title string, isbns []string) ([]web.InkItem, error) {
	if client == nil || index == "" {
		return nil, nil
	}

	var shouldQueries []types.Query

	if barcode != "" {
		shouldQueries = append(shouldQueries,
			types.Query{
				Match: map[string]types.MatchQuery{
					"meta.value": {Query: barcode},
				},
			},
			types.Query{
				Match: map[string]types.MatchQuery{
					"signature": {Query: barcode},
				},
			},
			types.Query{
				Match: map[string]types.MatchQuery{
					"signatureoriginal": {Query: barcode},
				},
			},
		)
	}

	for _, isbn := range isbns {
		isbn = strings.TrimSpace(isbn)
		if isbn != "" {
			shouldQueries = append(shouldQueries, types.Query{
				Match: map[string]types.MatchQuery{
					"meta.value": {Query: isbn},
				},
			})
		}
	}

	for _, author := range authors {
		author = strings.TrimSpace(author)
		if author != "" {
			shouldQueries = append(shouldQueries, types.Query{
				MatchPhrase: map[string]types.MatchPhraseQuery{
					"persons.name": {Query: author},
				},
			})
		}
	}

	if title != "" && title != "Ohne Titel" {
		shouldQueries = append(shouldQueries, types.Query{
			Match: map[string]types.MatchQuery{
				"title": {Query: title},
			},
		})
	}

	if len(shouldQueries) == 0 {
		return nil, nil
	}

	aclFilter := types.Query{
		Term: map[string]types.TermQuery{
			"acl.meta.keyword": {Value: "global/guest"},
		},
	}

	size := 4
	req := &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Filter:             []types.Query{aclFilter},
				Should:             shouldQueries,
				MinimumShouldMatch: "1",
			},
		},
		Size: &size,
	}

	res, err := client.Search().Index(index).Request(req).Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot search ink index %s", index)
	}

	items := make([]web.InkItem, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var item web.InkItem
		if err := json.Unmarshal(hit.Source_, &item); err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// searchOpenAlexWorks queries the openalex-works index for scholarly records matching the scanned item.
func searchOpenAlexWorks(ctx context.Context, client *elasticsearch.TypedClient, index string, isbns []string, title string, authors []string, dois []string) ([]web.OpenAlexWork, error) {
	if client == nil || index == "" {
		return nil, nil
	}

	var shouldQueries []types.Query

	for _, doi := range dois {
		doi = strings.TrimSpace(doi)
		if doi != "" {
			cleanDOI := strings.TrimPrefix(doi, "https://doi.org/")
			cleanDOI = strings.TrimPrefix(cleanDOI, "http://doi.org/")
			cleanDOI = strings.TrimPrefix(cleanDOI, "doi:")
			shouldQueries = append(shouldQueries,
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"doi": {Query: doi},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"doi": {Query: cleanDOI},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"ids.doi": {Query: cleanDOI},
					},
				},
			)
		}
	}

	for _, isbn := range isbns {
		isbn = strings.TrimSpace(isbn)
		if isbn != "" {
			cleanISBN := strings.ReplaceAll(isbn, "-", "")
			shouldQueries = append(shouldQueries,
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"ids.isbn": {Query: isbn},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"ids.isbn": {Query: cleanISBN},
					},
				},
				types.Query{
					Match: map[string]types.MatchQuery{
						"title": {Query: isbn},
					},
				},
			)
		}
	}

	for _, author := range authors {
		author = strings.TrimSpace(author)
		if author != "" {
			shouldQueries = append(shouldQueries, types.Query{
				MatchPhrase: map[string]types.MatchPhraseQuery{
					"authorships.author.display_name": {Query: author},
				},
			})
		}
	}

	if title != "" && title != "Ohne Titel" {
		shouldQueries = append(shouldQueries,
			types.Query{
				Match: map[string]types.MatchQuery{
					"title": {Query: title},
				},
			},
			types.Query{
				Match: map[string]types.MatchQuery{
					"display_name": {Query: title},
				},
			},
		)
	}

	if len(shouldQueries) == 0 {
		return nil, nil
	}

	size := 5
	req := &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Should:             shouldQueries,
				MinimumShouldMatch: "1",
			},
		},
		Size: &size,
	}

	res, err := client.Search().Index(index).Request(req).Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot search openalex works index %s", index)
	}

	works := make([]web.OpenAlexWork, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var work web.OpenAlexWork
		if err := json.Unmarshal(hit.Source_, &work); err != nil {
			continue
		}
		if work.ID == "" && hit.Id_ != nil && *hit.Id_ != "" {
			work.ID = *hit.Id_
		}
		works = append(works, work)
	}

	return works, nil
}

// searchOpenAlexCitingWorks queries openalex-works for works that cite the specified work ID.
func searchOpenAlexCitingWorks(ctx context.Context, client *elasticsearch.TypedClient, index string, workID string, limit int) ([]web.OpenAlexWork, error) {
	if client == nil || index == "" || strings.TrimSpace(workID) == "" {
		return nil, nil
	}

	cleanID := strings.TrimSpace(workID)
	cleanID = strings.TrimPrefix(cleanID, "https://openalex.org/")
	cleanID = strings.TrimPrefix(cleanID, "http://openalex.org/")
	cleanID = strings.TrimPrefix(cleanID, "works/")
	fullID := "https://openalex.org/" + cleanID

	var shouldQueries []types.Query
	shouldQueries = append(shouldQueries,
		types.Query{
			Term: map[string]types.TermQuery{
				"referenced_works.keyword": {Value: fullID},
			},
		},
		types.Query{
			Term: map[string]types.TermQuery{
				"referenced_works.keyword": {Value: cleanID},
			},
		},
		types.Query{
			MatchPhrase: map[string]types.MatchPhraseQuery{
				"referenced_works": {Query: fullID},
			},
		},
		types.Query{
			MatchPhrase: map[string]types.MatchPhraseQuery{
				"referenced_works": {Query: cleanID},
			},
		},
	)

	if limit <= 0 {
		limit = 3
	}

	req := &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Should:             shouldQueries,
				MinimumShouldMatch: "1",
			},
		},
		Sort: []types.SortCombinations{
			types.SortOptions{
				SortOptions: map[string]types.FieldSort{
					"cited_by_count": {Order: &sortorder.Desc},
				},
			},
		},
		Size: &limit,
	}

	res, err := client.Search().Index(index).Request(req).Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot search openalex citing works in index %s for work %s", index, workID)
	}

	works := make([]web.OpenAlexWork, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var work web.OpenAlexWork
		if err := json.Unmarshal(hit.Source_, &work); err != nil {
			continue
		}
		if work.ID == "" && hit.Id_ != nil && *hit.Id_ != "" {
			work.ID = *hit.Id_
		}
		works = append(works, work)
	}

	return works, nil
}

// searchAlmaWorks queries the fhnw_alma-full index for records matching an OpenAlex work (by DOIs, ISBNs, title, authors).
func searchAlmaWorks(ctx context.Context, client *elasticsearch.TypedClient, index string, dois []string, isbns []string, title string, authors []string) ([]catalog.Book, error) {
	if client == nil || index == "" {
		return nil, nil
	}

	var shouldQueries []types.Query

	for _, doi := range dois {
		doi = strings.TrimSpace(doi)
		if doi != "" {
			cleanDOI := strings.TrimPrefix(doi, "https://doi.org/")
			cleanDOI = strings.TrimPrefix(cleanDOI, "http://doi.org/")
			cleanDOI = strings.TrimPrefix(cleanDOI, "doi:")
			shouldQueries = append(shouldQueries,
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"024.a": {Query: doi},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"024.a": {Query: cleanDOI},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"search.identifier.keyword": {Query: cleanDOI},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"mapping.recordIdentifier": {Query: cleanDOI},
					},
				},
			)
		}
	}

	for _, isbn := range isbns {
		isbn = strings.TrimSpace(isbn)
		if isbn != "" {
			cleanISBN := strings.ReplaceAll(isbn, "-", "")
			shouldQueries = append(shouldQueries,
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"020.a": {Query: isbn},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"020.a": {Query: cleanISBN},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"mapping.identifier.isbn.id": {Query: isbn},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"mapping.identifier.isbn.enriched": {Query: isbn},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"search.identifier.keyword": {Query: cleanISBN},
					},
				},
			)
		}
	}

	for _, author := range authors {
		author = strings.TrimSpace(author)
		if author != "" {
			shouldQueries = append(shouldQueries,
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"100.a": {Query: author},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"700.a": {Query: author},
					},
				},
				types.Query{
					MatchPhrase: map[string]types.MatchPhraseQuery{
						"mapping.name.personal.aut.namePart": {Query: author},
					},
				},
			)
		}
	}

	if title != "" && title != "Ohne Titel" {
		shouldQueries = append(shouldQueries,
			types.Query{
				Match: map[string]types.MatchQuery{
					"mapping.titleInfo.main.title": {Query: title},
				},
			},
			types.Query{
				Match: map[string]types.MatchQuery{
					"245.a": {Query: title},
				},
			},
		)
	}

	if len(shouldQueries) == 0 {
		return nil, nil
	}

	size := 2
	req := &search.Request{
		Query: &types.Query{
			Bool: &types.BoolQuery{
				Should:             shouldQueries,
				MinimumShouldMatch: "1",
			},
		},
		Size: &size,
	}

	res, err := client.Search().Index(index).Request(req).Do(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot search alma works index %s", index)
	}

	books := make([]catalog.Book, 0, len(res.Hits.Hits))
	for _, hit := range res.Hits.Hits {
		var book catalog.Book
		if err := json.Unmarshal(hit.Source_, &book); err != nil {
			continue
		}
		if book.ID == "" && hit.Id_ != nil && *hit.Id_ != "" {
			book.ID = *hit.Id_
		}
		books = append(books, book)
	}

	return books, nil
}
