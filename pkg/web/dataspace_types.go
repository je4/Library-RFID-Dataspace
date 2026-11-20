package web

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/catalog"
)

// ============================================================================
// API Response Types for Dataspace
// ============================================================================

// DataspaceGraphResponse represents the JSON payload returned to the frontend
// containing the generated Mermaid flowchart and structural graph metrics.
type DataspaceGraphResponse struct {
	// Mermaid contains the complete text markup of the Mermaid.js flowchart.
	Mermaid string `json:"mermaid"`
	// ItemCount is the total number of distinct barcode media items included in the graph.
	ItemCount int `json:"itemCount"`
	// NodeCount is the total number of unique graph nodes (books, authors, topics, repositories, etc.).
	NodeCount int `json:"nodeCount"`
	// EdgeCount is the total number of relationships connecting the graph nodes.
	EdgeCount int `json:"edgeCount"`
	// Barcodes lists the cleaned, deduplicated barcodes analyzed in this graph.
	Barcodes []string `json:"barcodes"`
	// Direction indicates the layout orientation ("TD" for top-down or "LR" for left-to-right).
	Direction string `json:"direction"`
}

// DataspaceReportResponse represents the structured bibliographic and scholarly dossier
// returned for the active RFID media items on the desk or requested via API.
type DataspaceReportResponse struct {
	// GeneratedAt is the timestamp when this report was created.
	GeneratedAt time.Time `json:"generatedAt"`
	// ItemCount is the total number of barcode items processed.
	ItemCount int `json:"itemCount"`
	// WorkCount is the total number of matched OpenAlex scholarly works.
	WorkCount int `json:"workCount"`
	// AuthorCount is the total number of unique creators and researchers discovered.
	AuthorCount int `json:"authorCount"`
	// TopicCount is the total number of unique subject terms and research topics.
	TopicCount int `json:"topicCount"`
	// InkCount is the total number of matched digital assets in the Mediathek InK archive.
	InkCount int `json:"inkCount"`
	// Barcodes lists the cleaned, deduplicated barcodes for the items in the report.
	Barcodes []string `json:"barcodes"`
	// Items contains detailed bibliographic, repository, and scholarly records per scanned barcode.
	Items []DataspaceReportItem `json:"items"`
	// Summary provides aggregated metrics and top domain/field/author statistics.
	Summary DataspaceSummaryStats `json:"summary"`
	// SharedLinks contains cross-connections (shared authors, common topics) discovered between items.
	SharedLinks []DataspaceReportLink `json:"sharedLinks"`
}

// DataspaceSummaryStats holds aggregate overview numbers and prominent entities
// for display in the report dashboard header.
type DataspaceSummaryStats struct {
	// TotalBarcodes is the number of distinct barcodes queried.
	TotalBarcodes int `json:"totalBarcodes"`
	// CatalogHits is the number of items successfully resolved in the Alma library catalog.
	CatalogHits int `json:"catalogHits"`
	// InkHits is the number of digital assets matched in Mediathek InK.
	InkHits int `json:"inkHits"`
	// OpenAlexHits is the number of matched scholarly publications in OpenAlex.
	OpenAlexHits int `json:"openAlexHits"`
	// TotalCitations is the accumulated number of citations for all matched works.
	TotalCitations int `json:"totalCitations"`
	// PrimaryDomains lists the most frequent high-level research domains (e.g. "Physical Sciences").
	PrimaryDomains []string `json:"primaryDomains"`
	// PrimaryFields lists the most frequent research fields (e.g. "Computer Science", "Physics").
	PrimaryFields []string `json:"primaryFields"`
	// ProminentAuthors lists the most frequently appearing authors across all items.
	ProminentAuthors []string `json:"prominentAuthors"`
}

// DataspaceReportItem contains all resolved bibliographic, repository, and scholarly data
// for a single physical barcode item.
type DataspaceReportItem struct {
	// Barcode is the RFID barcode identifier of the physical medium.
	Barcode string `json:"barcode"`
	// HasCatalog indicates whether the item was found in the Alma catalog.
	HasCatalog bool `json:"hasCatalog"`
	// Title is the main title of the publication.
	Title string `json:"title"`
	// Subtitle is the subtitle or title continuation.
	Subtitle string `json:"subtitle,omitempty"`
	// Responsibility describes the statement of responsibility (authors, editors).
	Responsibility string `json:"responsibility,omitempty"`
	// PublicationYear is the year or date string of publication.
	PublicationYear string `json:"publicationYear,omitempty"`
	// Publisher is the publishing house or issuing body.
	Publisher string `json:"publisher,omitempty"`
	// ISBN is the primary International Standard Book Number.
	ISBN string `json:"isbn,omitempty"`
	// CallNumber is the library shelf location / signature.
	CallNumber string `json:"callNumber,omitempty"`
	// MMSID is the unique bibliographic record ID in Alma / SLSP.
	MMSID string `json:"mmsId,omitempty"`
	// SwisscoveryURL is the direct deep link to the search record in Swisscovery.
	SwisscoveryURL string `json:"swisscoveryUrl,omitempty"`
	// Authors lists the authors and contributors associated with the catalog record.
	Authors []DataspaceAuthorEntry `json:"authors,omitempty"`
	// Subjects lists the subject headings and classifications associated with the record.
	Subjects []DataspaceTopicEntry `json:"subjects,omitempty"`
	// InkItems contains matched digital repository assets from Mediathek InK (HGK FHNW).
	InkItems []InkItemReport `json:"inkItems,omitempty"`
	// OpenAlexWorks contains matched scholarly works and citation metadata from OpenAlex.
	OpenAlexWorks []OpenAlexWorkReport `json:"openAlexWorks,omitempty"`
}

// DataspaceAuthorEntry represents an author with GND authority identifiers and OpenAlex links.
type DataspaceAuthorEntry struct {
	// Name is the formatted personal or corporate name.
	Name string `json:"name"`
	// Role is the MARC relator role code (e.g. "aut", "edt", "art").
	Role string `json:"role,omitempty"`
	// GNDID is the German Integrated Authority File identifier (Gemeinsame Normdatei).
	GNDID string `json:"gndId,omitempty"`
	// GNDURL is the deep link to the GND entity on lobid.org / DNB.
	GNDURL string `json:"gndUrl,omitempty"`
	// OpenAlexID is the OpenAlex author entity ID (e.g. "A5090064378").
	OpenAlexID string `json:"openAlexId,omitempty"`
	// OpenAlexURL is the deep link to the OpenAlex author profile or search page.
	OpenAlexURL string `json:"openAlexUrl,omitempty"`
	// ORCID is the persistent digital identifier for researchers (e.g. "0000-0002-1825-0097").
	ORCID string `json:"orcid,omitempty"`
}

// DataspaceTopicEntry represents a topic, subject term, or academic classification.
type DataspaceTopicEntry struct {
	// Label is the human-readable subject name or research topic.
	Label string `json:"label"`
	// Identifier is the authority or taxonomy ID (e.g. GND ID or OpenAlex Topic ID).
	Identifier string `json:"identifier,omitempty"`
	// URL is the deep link to the topic in OpenAlex or lobid.org.
	URL string `json:"url,omitempty"`
	// Subfield is the academic subfield name in OpenAlex (e.g. "Artificial Intelligence").
	Subfield string `json:"subfield,omitempty"`
	// SubfieldURL is the deep link to the academic subfield in OpenAlex.
	SubfieldURL string `json:"subfieldUrl,omitempty"`
	// Field is the academic field name in OpenAlex (e.g. "Computer Science").
	Field string `json:"field,omitempty"`
	// FieldURL is the deep link to the academic field in OpenAlex.
	FieldURL string `json:"fieldUrl,omitempty"`
	// Domain is the highest-level research domain in OpenAlex (e.g. "Physical Sciences").
	Domain string `json:"domain,omitempty"`
	// DomainURL is the deep link to the research domain in OpenAlex.
	DomainURL string `json:"domainUrl,omitempty"`
}

// InkCategoryReport represents an InK archive category with label and deep link.
type InkCategoryReport struct {
	// Raw is the original category string (e.g. "zotero2!!Videonetzwerke!!Videogenossenschaft").
	Raw string `json:"raw"`
	// Label is the clean, human-readable category name.
	Label string `json:"label"`
	// URL is the deep link to the category grid in Mediathek InK.
	URL string `json:"url"`
}

// InkItemReport represents a formatted InK repository item for the report dossier.
type InkItemReport struct {
	// Signature is the unique archive signature (e.g. "zotero2-2068924.UHBBY3JP").
	Signature string `json:"signature"`
	// Title is the title of the digital asset, thesis, or collection work.
	Title string `json:"title"`
	// Type is the publication type (e.g. "thesis", "article", "video").
	Type string `json:"type,omitempty"`
	// Date is the creation or publication date string.
	Date string `json:"date,omitempty"`
	// CollectionTitle is the parent archive collection or repository name.
	CollectionTitle string `json:"collectionTitle,omitempty"`
	// CollectionURL is the deep link to the estate / holdings search in Mediathek InK.
	CollectionURL string `json:"collectionUrl,omitempty"`
	// Categories lists the InK archive categories with deep links.
	Categories []InkCategoryReport `json:"categories,omitempty"`
	// Publisher is the issuing institute, publisher, or degree programme.
	Publisher string `json:"publisher,omitempty"`
	// Abstract contains a brief summary of the asset content.
	Abstract string `json:"abstract,omitempty"`
	// DetailURL is the direct deep link to the asset in the Mediathek InK web portal.
	DetailURL string `json:"detailUrl"`
	// Persons lists the creators, advisors, and contributors in InK.
	Persons []InkPerson `json:"persons,omitempty"`
	// Tags contains subject and catalog keyword tags.
	Tags []string `json:"tags,omitempty"`
}

// OpenAlexWorkReport represents a formatted OpenAlex work entry for the report dossier.
type OpenAlexWorkReport struct {
	// ID is the OpenAlex work identifier (e.g. "https://openalex.org/W642892681").
	ID string `json:"id"`
	// WorkURL is the deep link to the work in OpenAlex.
	WorkURL string `json:"workUrl"`
	// DOI is the Digital Object Identifier string (e.g. "10.1002/phbl.19940501211").
	DOI string `json:"doi,omitempty"`
	// DOIURL is the direct resolver URL (e.g. "https://doi.org/10.1002/phbl.19940501211").
	DOIURL string `json:"doiUrl,omitempty"`
	// DisplayName is the official title of the scholarly work.
	DisplayName string `json:"displayName"`
	// PublicationYear is the year when the work was published.
	PublicationYear int `json:"publicationYear,omitempty"`
	// Type is the work genre (e.g. "article", "book", "dataset").
	Type string `json:"type,omitempty"`
	// TypeURL is the deep link to the type taxonomy in OpenAlex.
	TypeURL string `json:"typeUrl,omitempty"`
	// CitedByCount is the total number of citations recorded for this work in OpenAlex.
	CitedByCount int `json:"citedByCount,omitempty"`
	// CitedByURL is the deep link to all citing works in OpenAlex.
	CitedByURL string `json:"citedByUrl,omitempty"`
	// SourceName is the name of the hosting journal, book series, or repository.
	SourceName string `json:"sourceName,omitempty"`
	// SourceURL is the deep link to the hosting venue profile in OpenAlex.
	SourceURL string `json:"sourceUrl,omitempty"`
	// PublisherName is the parent host organization or publisher name.
	PublisherName string `json:"publisherName,omitempty"`
	// PublisherURL is the deep link to the publisher in OpenAlex.
	PublisherURL string `json:"publisherUrl,omitempty"`
	// PrimaryTopic represents the primary scholarly topic classified by OpenAlex.
	PrimaryTopic *DataspaceTopicEntry `json:"primaryTopic,omitempty"`
	// SecondaryTopics contains other relevant subject classifications.
	SecondaryTopics []DataspaceTopicEntry `json:"secondaryTopics,omitempty"`
	// Authorships lists the contributing authors and their affiliated academic institutions.
	Authorships []OpenAlexAuthorshipItem `json:"authorships,omitempty"`
	// CitingWorks contains the top citing works discovered in OpenAlex.
	CitingWorks []OpenAlexWorkReport `json:"citingWorks,omitempty"`
	// InLibraryCollection is true if this work was matched in the physical Alma library catalog.
	InLibraryCollection bool `json:"inLibraryCollection,omitempty"`
	// AlmaMMSID is the library system identifier in Alma if available.
	AlmaMMSID string `json:"almaMmsId,omitempty"`
	// AlmaCallNumber is the shelf mark / call number in the library collection.
	AlmaCallNumber string `json:"almaCallNumber,omitempty"`
	// AlmaSwisscoveryURL is the deep link to the work in the Swisscovery library network.
	AlmaSwisscoveryURL string `json:"almaSwisscoveryUrl,omitempty"`
	// AlmaLibrary is the library name or holding location where the item is shelved.
	AlmaLibrary string `json:"almaLibrary,omitempty"`
}

// OpenAlexAuthorshipItem represents an authorship with affiliations for the report dossier.
type OpenAlexAuthorshipItem struct {
	// AuthorName is the display name of the author.
	AuthorName string `json:"authorName"`
	// AuthorID is the OpenAlex author identifier.
	AuthorID string `json:"authorId,omitempty"`
	// AuthorURL is the deep link to the author profile in OpenAlex.
	AuthorURL string `json:"authorUrl,omitempty"`
	// ORCID is the researcher's ORCID identifier.
	ORCID string `json:"orcid,omitempty"`
	// Institutions lists the university or research institutions the author is affiliated with.
	Institutions []DataspaceInstitution `json:"institutions,omitempty"`
}

// DataspaceInstitution represents an academic affiliation or research institution.
type DataspaceInstitution struct {
	// Name is the display name of the institution.
	Name string `json:"name"`
	// ID is the OpenAlex institution identifier (e.g. "https://openalex.org/I118019719").
	ID string `json:"id,omitempty"`
	// URL is the deep link to the institution in OpenAlex.
	URL string `json:"url,omitempty"`
	// ROR is the Research Organization Registry identifier (e.g. "https://ror.org/00hx57361").
	ROR string `json:"ror,omitempty"`
	// Type is the institution type (e.g. "education", "facility", "government").
	Type string `json:"type,omitempty"`
}

// DataspaceReportLink represents a discovered cross-connection between multiple scanned items.
type DataspaceReportLink struct {
	// SourceBarcode is the barcode of the originating medium.
	SourceBarcode string `json:"sourceBarcode"`
	// SourceTitle is the title of the originating medium.
	SourceTitle string `json:"sourceTitle"`
	// TargetBarcode is the barcode of the connected target medium.
	TargetBarcode string `json:"targetBarcode"`
	// TargetTitle is the title of the connected target medium.
	TargetTitle string `json:"targetTitle"`
	// RelationType describes the nature of the link (e.g. "Shared Author", "Shared Topic").
	RelationType string `json:"relationType"`
	// Description provides human-readable context on why the items are connected.
	Description string `json:"description"`
}

// ============================================================================
// Domain Entities for Mediathek InK & OpenAlex
// ============================================================================

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

// OpenAlexWork represents a scholarly work from openalex-works.
type OpenAlexWork struct {
	ID              string               `json:"id"`
	DOI             string               `json:"doi,omitempty"`
	Title           string               `json:"title,omitempty"`
	DisplayName     string               `json:"display_name,omitempty"`
	PublicationYear int                  `json:"publication_year,omitempty"`
	Type            string               `json:"type,omitempty"`
	CitedByCount    int                  `json:"cited_by_count,omitempty"`
	CitedByAPIURL   string               `json:"cited_by_api_url,omitempty"`
	Authorships     []OpenAlexAuthorship `json:"authorships,omitempty"`
	PrimaryTopic    *OpenAlexTopicRef    `json:"primary_topic,omitempty"`
	Topics          []OpenAlexTopicRef   `json:"topics,omitempty"`
	PrimaryLocation *OpenAlexLocation    `json:"primary_location,omitempty"`
	Locations       []OpenAlexLocation   `json:"locations,omitempty"`
	ReferencedWorks []string             `json:"referenced_works,omitempty"`
	RelatedWorks    []string             `json:"related_works,omitempty"`
	CitingWorks     []OpenAlexWork       `json:"citing_works,omitempty"`
	AlmaBook        *catalog.Book        `json:"alma_book,omitempty"`
}

// OpenAlexAuthorship represents an authorship structure in OpenAlex.
type OpenAlexAuthorship struct {
	Author       OpenAlexAuthorRef        `json:"author"`
	Institutions []OpenAlexInstitutionRef `json:"institutions,omitempty"`
}

// OpenAlexAuthorRef represents an author entity in OpenAlex.
type OpenAlexAuthorRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ORCID       string `json:"orcid,omitempty"`
}

// OpenAlexInstitutionRef represents an institution entity in OpenAlex.
type OpenAlexInstitutionRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ROR         string `json:"ror,omitempty"`
	Type        string `json:"type,omitempty"`
}

// OpenAlexTopicRef represents a research topic in OpenAlex.
type OpenAlexTopicRef struct {
	ID          string               `json:"id"`
	DisplayName string               `json:"display_name"`
	Subfield    *OpenAlexTaxonomyRef `json:"subfield,omitempty"`
	Field       *OpenAlexTaxonomyRef `json:"field,omitempty"`
	Domain      *OpenAlexTaxonomyRef `json:"domain,omitempty"`
}

// OpenAlexTaxonomyRef represents a subfield, field, or domain entity.
type OpenAlexTaxonomyRef struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// OpenAlexLocation represents a location / hosting venue for a work.
type OpenAlexLocation struct {
	Source *OpenAlexSourceRef `json:"source,omitempty"`
}

// OpenAlexSourceRef represents a journal, repository, or publisher host.
type OpenAlexSourceRef struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	Type             string `json:"type,omitempty"`
	HostOrganization string `json:"host_organization,omitempty"`
	HostOrgName      string `json:"host_organization_name,omitempty"`
}

// OpenAlexSearchFunc searches the openalex-works index for matching scholarly records.
type OpenAlexSearchFunc func(ctx context.Context, isbns []string, title string, authors []string, dois []string) ([]OpenAlexWork, error)

// OpenAlexCitingSearchFunc searches the openalex-works index for works citing a specific work ID.
type OpenAlexCitingSearchFunc func(ctx context.Context, workID string, limit int) ([]OpenAlexWork, error)

// CatalogWorkSearchFunc searches the Alma catalog index for records matching an OpenAlex work (by DOIs, ISBNs, title, authors).
type CatalogWorkSearchFunc func(ctx context.Context, dois []string, isbns []string, title string, authors []string) ([]catalog.Book, error)

// dataspaceItemContext encapsulates all raw queried data for a single barcode.
type dataspaceItemContext struct {
	barcode       string
	book          *catalog.Book
	inkItems      []InkItem
	openAlexWorks []OpenAlexWork
}

// ============================================================================
// Generic Utilities & Functional Helpers
// ============================================================================

// cleanAndDeduplicate returns a slice containing the elements of items with
// duplicate values removed, preserving original insertion order.
func cleanAndDeduplicate[T comparable](items []T) []T {
	if len(items) == 0 {
		return []T{}
	}
	result := make([]T, 0, len(items))
	seen := make(map[T]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	return result
}

// cleanStrings trims whitespace from each string in items, discards any empty
// strings, and removes duplicates while preserving original order.
func cleanStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, s := range items {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			if _, exists := seen[trimmed]; !exists {
				seen[trimmed] = struct{}{}
				result = append(result, trimmed)
			}
		}
	}
	return result
}

// mapSlice transforms a slice of type T into a slice of type R using the supplied mapper function.
func mapSlice[T any, R any](items []T, transform func(T) R) []R {
	if items == nil {
		return nil
	}
	result := make([]R, len(items))
	for i, v := range items {
		result[i] = transform(v)
	}
	return result
}

// filterSlice returns a new slice containing only elements that satisfy the predicate.
func filterSlice[T any](items []T, predicate func(T) bool) []T {
	if len(items) == 0 {
		return []T{}
	}
	result := make([]T, 0, len(items))
	for _, item := range items {
		if predicate(item) {
			result = append(result, item)
		}
	}
	return result
}

// topKeysByFrequency extracts the top limit keys from a frequency map, sorted by
// their values in descending order. If multiple keys have the same value, their relative order is stable.
func topKeysByFrequency[K comparable, V cmp.Ordered](freqMap map[K]V, limit int) []K {
	if len(freqMap) == 0 || limit <= 0 {
		return []K{}
	}
	type pair struct {
		key   K
		value V
	}
	pairs := make([]pair, 0, len(freqMap))
	for k, v := range freqMap {
		pairs = append(pairs, pair{key: k, value: v})
	}

	// Sort descending by value
	slices.SortStableFunc(pairs, func(a, b pair) int {
		return cmp.Compare(b.value, a.value)
	})

	if limit > len(pairs) {
		limit = len(pairs)
	}

	res := make([]K, limit)
	for i := 0; i < limit; i++ {
		res[i] = pairs[i].key
	}
	return res
}

// funcCacheLoader provides a generic Otter cache loader implementation from a loading function.
type funcCacheLoader[K comparable, V any] struct {
	loadFn func(ctx context.Context, key K) (V, error)
}

// newFuncCacheLoader constructs a generic funcCacheLoader.
func newFuncCacheLoader[K comparable, V any](loadFn func(ctx context.Context, key K) (V, error)) funcCacheLoader[K, V] {
	return funcCacheLoader[K, V]{loadFn: loadFn}
}

// Load executes the underlying load function for the cache key.
func (l funcCacheLoader[K, V]) Load(ctx context.Context, key K) (V, error) {
	return l.loadFn(ctx, key)
}

// Reload re-fetches the cached value for the given key.
func (l funcCacheLoader[K, V]) Reload(ctx context.Context, key K, _ V) (V, error) {
	return l.loadFn(ctx, key)
}

// ============================================================================
// Catalog Book Extraction Helpers
// ============================================================================

// CatalogAuthorInfo encapsulates extracted author details from an Alma catalog record.
type CatalogAuthorInfo struct {
	Name        string
	Role        string
	GNDID       string
	GNDURL      string
	OpenAlexURL string
	Tooltip     string
}

// CatalogTopicInfo encapsulates extracted topic details from an Alma catalog record.
type CatalogTopicInfo struct {
	Label      string
	Identifier string
	URL        string
	Tooltip    string
}

// extractBookTitle retrieves the main title and subtitle from a book record with a fallback.
func extractBookTitle(book *catalog.Book, fallback string) (title string, subtitle string) {
	if book != nil && len(book.Titles) > 0 {
		title = book.Titles[0].Title
		subtitle = book.Titles[0].SubTitle
	}
	if title == "" {
		title = fallback
	}
	return title, subtitle
}

// extractPrimaryISBN returns the primary ISBN (enriched or raw ID) from a book record.
func extractPrimaryISBN(book *catalog.Book) string {
	if book == nil || len(book.ISBNs) == 0 {
		return ""
	}
	if book.ISBNs[0].Enriched != "" {
		return book.ISBNs[0].Enriched
	}
	return book.ISBNs[0].ID
}

// extractAllISBNs returns all non-empty ISBN identifiers from a book record.
func extractAllISBNs(book *catalog.Book) []string {
	if book == nil {
		return nil
	}
	var isbns []string
	for _, isbn := range book.ISBNs {
		if isbn.Enriched != "" {
			isbns = append(isbns, isbn.Enriched)
		} else if isbn.ID != "" {
			isbns = append(isbns, isbn.ID)
		}
	}
	return cleanStrings(isbns)
}

// extractBookCallNumber retrieves the call number or shelf location from holdings.
func extractBookCallNumber(book *catalog.Book) string {
	if book == nil || len(book.Holdings) == 0 {
		return ""
	}
	if book.Holdings[0].CallNumber != "" {
		return book.Holdings[0].CallNumber
	}
	if len(book.Holdings[0].Items) > 0 && book.Holdings[0].Items[0].ItemCallNumber != "" {
		return book.Holdings[0].Items[0].ItemCallNumber
	}
	return ""
}

// extractBookPublisher retrieves the publication year and formatted publishers.
func extractBookPublisher(book *catalog.Book) (pubYear string, publisher string) {
	if book == nil || len(book.Publications) == 0 {
		return "", ""
	}
	pub := book.Publications[0]
	pubYear = pub.DateString
	if len(pub.Publisher) > 0 {
		publisher = strings.Join(pub.Publisher, ", ")
	}
	return pubYear, publisher
}

// extractBookDOIs retrieves all DOI identifiers present in a book record.
func extractBookDOIs(book *catalog.Book) []string {
	if book == nil {
		return nil
	}
	var dois []string
	for _, id := range book.Identifiers {
		trimmed := strings.TrimSpace(id)
		if strings.HasPrefix(strings.ToLower(trimmed), "doi:") || strings.Contains(trimmed, "10.") {
			dois = append(dois, trimmed)
		}
	}
	return cleanStrings(dois)
}

// extractCatalogAuthors extracts structured author entries from Alma PersonalNames.
func extractCatalogAuthors(book *catalog.Book) []CatalogAuthorInfo {
	if book == nil {
		return nil
	}
	var authors []CatalogAuthorInfo
	for _, nameMap := range book.PersonalNames {
		for role, nameEntry := range nameMap {
			if nameEntry.NamePart != "" {
				gndID := extractGNDID(nameEntry.Identifier)
				var gndURL string
				if gndID != "" {
					gndURL = fmt.Sprintf("https://lobid.org/gnd/%s", url.PathEscape(gndID))
				}
				oaURL, tooltip := buildAuthorOrGNDURL(nameEntry.Identifier, nameEntry.NamePart)
				authors = append(authors, CatalogAuthorInfo{
					Name:        nameEntry.NamePart,
					Role:        role,
					GNDID:       gndID,
					GNDURL:      gndURL,
					OpenAlexURL: oaURL,
					Tooltip:     tooltip,
				})
			}
		}
	}
	return authors
}

// extractCatalogTopics extracts structured subject terms from Alma Subjects.Topic.
func extractCatalogTopics(book *catalog.Book) []CatalogTopicInfo {
	if book == nil {
		return nil
	}
	var topics []CatalogTopicInfo
	for _, topicMap := range book.Subjects.Topic {
		for _, term := range topicMap {
			if term.Label != "" {
				topicURL, tooltip := buildTopicOrGNDURL(term.Identifier, term.Label)
				topics = append(topics, CatalogTopicInfo{
					Label:      term.Label,
					Identifier: term.Identifier,
					URL:        topicURL,
					Tooltip:    tooltip,
				})
			}
		}
	}
	return topics
}
