package main

import (
	"context"
	"encoding/json"
	"strings"

	"emperror.dev/errors"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
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
