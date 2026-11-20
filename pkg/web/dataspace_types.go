package web

import (
	"context"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/catalog"
)

// DataspaceGraphResponse represents the JSON payload returned to the frontend for the Mermaid graph.
type DataspaceGraphResponse struct {
	Mermaid   string   `json:"mermaid"`
	ItemCount int      `json:"itemCount"`
	NodeCount int      `json:"nodeCount"`
	EdgeCount int      `json:"edgeCount"`
	Barcodes  []string `json:"barcodes"`
	Direction string   `json:"direction"`
}

// DataspaceReportResponse represents the structured report / dossier returned for the active items.
type DataspaceReportResponse struct {
	GeneratedAt time.Time             `json:"generatedAt"`
	ItemCount   int                   `json:"itemCount"`
	WorkCount   int                   `json:"workCount"`
	AuthorCount int                   `json:"authorCount"`
	TopicCount  int                   `json:"topicCount"`
	InkCount    int                   `json:"inkCount"`
	Barcodes    []string              `json:"barcodes"`
	Items       []DataspaceReportItem `json:"items"`
	Summary     DataspaceSummaryStats `json:"summary"`
	SharedLinks []DataspaceReportLink `json:"sharedLinks"`
}

// DataspaceSummaryStats holds aggregate numbers for the header overview.
type DataspaceSummaryStats struct {
	TotalBarcodes    int      `json:"totalBarcodes"`
	CatalogHits      int      `json:"catalogHits"`
	InkHits          int      `json:"inkHits"`
	OpenAlexHits     int      `json:"openAlexHits"`
	PrimaryDomains   []string `json:"primaryDomains"`
	PrimaryFields    []string `json:"primaryFields"`
	ProminentAuthors []string `json:"prominentAuthors"`
}

// DataspaceReportItem contains all resolved bibliographic, repository, and scholarly data for one item.
type DataspaceReportItem struct {
	Barcode         string                 `json:"barcode"`
	HasCatalog      bool                   `json:"hasCatalog"`
	Title           string                 `json:"title"`
	Subtitle        string                 `json:"subtitle,omitempty"`
	Responsibility  string                 `json:"responsibility,omitempty"`
	PublicationYear string                 `json:"publicationYear,omitempty"`
	Publisher       string                 `json:"publisher,omitempty"`
	ISBN            string                 `json:"isbn,omitempty"`
	CallNumber      string                 `json:"callNumber,omitempty"`
	MMSID           string                 `json:"mmsId,omitempty"`
	SwisscoveryURL  string                 `json:"swisscoveryUrl,omitempty"`
	Authors         []DataspaceAuthorEntry `json:"authors,omitempty"`
	Subjects        []DataspaceTopicEntry  `json:"subjects,omitempty"`
	InkItems        []InkItemReport        `json:"inkItems,omitempty"`
	OpenAlexWorks   []OpenAlexWorkReport   `json:"openAlexWorks,omitempty"`
}

// DataspaceAuthorEntry represents an author with GND authority and OpenAlex links.
type DataspaceAuthorEntry struct {
	Name        string `json:"name"`
	Role        string `json:"role,omitempty"`
	GNDID       string `json:"gndId,omitempty"`
	GNDURL      string `json:"gndUrl,omitempty"`
	OpenAlexID  string `json:"openAlexId,omitempty"`
	OpenAlexURL string `json:"openAlexUrl,omitempty"`
	ORCID       string `json:"orcid,omitempty"`
}

// DataspaceTopicEntry represents a topic, subject term, or classification.
type DataspaceTopicEntry struct {
	Label       string `json:"label"`
	Identifier  string `json:"identifier,omitempty"`
	URL         string `json:"url,omitempty"`
	Subfield    string `json:"subfield,omitempty"`
	SubfieldURL string `json:"subfieldUrl,omitempty"`
	Field       string `json:"field,omitempty"`
	FieldURL    string `json:"fieldUrl,omitempty"`
	Domain      string `json:"domain,omitempty"`
	DomainURL   string `json:"domainUrl,omitempty"`
}

// InkItemReport represents a formatted InK repository item for the report.
type InkItemReport struct {
	Signature       string      `json:"signature"`
	Title           string      `json:"title"`
	Type            string      `json:"type,omitempty"`
	Date            string      `json:"date,omitempty"`
	CollectionTitle string      `json:"collectionTitle,omitempty"`
	Publisher       string      `json:"publisher,omitempty"`
	Abstract        string      `json:"abstract,omitempty"`
	DetailURL       string      `json:"detailUrl"`
	Persons         []InkPerson `json:"persons,omitempty"`
	Tags            []string    `json:"tags,omitempty"`
}

// OpenAlexWorkReport represents a formatted OpenAlex work entry for the report.
type OpenAlexWorkReport struct {
	ID              string                   `json:"id"`
	WorkURL         string                   `json:"workUrl"`
	DOI             string                   `json:"doi,omitempty"`
	DOIURL          string                   `json:"doiUrl,omitempty"`
	DisplayName     string                   `json:"displayName"`
	PublicationYear int                      `json:"publicationYear,omitempty"`
	Type            string                   `json:"type,omitempty"`
	TypeURL         string                   `json:"typeUrl,omitempty"`
	SourceName      string                   `json:"sourceName,omitempty"`
	SourceURL       string                   `json:"sourceUrl,omitempty"`
	PublisherName   string                   `json:"publisherName,omitempty"`
	PublisherURL    string                   `json:"publisherUrl,omitempty"`
	PrimaryTopic    *DataspaceTopicEntry     `json:"primaryTopic,omitempty"`
	SecondaryTopics []DataspaceTopicEntry    `json:"secondaryTopics,omitempty"`
	Authorships     []OpenAlexAuthorshipItem `json:"authorships,omitempty"`
}

// OpenAlexAuthorshipItem represents an authorship with affiliations for the report.
type OpenAlexAuthorshipItem struct {
	AuthorName   string                 `json:"authorName"`
	AuthorID     string                 `json:"authorId,omitempty"`
	AuthorURL    string                 `json:"authorUrl,omitempty"`
	ORCID        string                 `json:"orcid,omitempty"`
	Institutions []DataspaceInstitution `json:"institutions,omitempty"`
}

// DataspaceInstitution represents an academic affiliation.
type DataspaceInstitution struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
	ROR  string `json:"ror,omitempty"`
	Type string `json:"type,omitempty"`
}

// DataspaceReportLink represents a discovered cross-connection between multiple scanned items.
type DataspaceReportLink struct {
	SourceBarcode string `json:"sourceBarcode"`
	SourceTitle   string `json:"sourceTitle"`
	TargetBarcode string `json:"targetBarcode"`
	TargetTitle   string `json:"targetTitle"`
	RelationType  string `json:"relationType"`
	Description   string `json:"description"`
}

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
	Authorships     []OpenAlexAuthorship `json:"authorships,omitempty"`
	PrimaryTopic    *OpenAlexTopicRef    `json:"primary_topic,omitempty"`
	Topics          []OpenAlexTopicRef   `json:"topics,omitempty"`
	PrimaryLocation *OpenAlexLocation    `json:"primary_location,omitempty"`
	Locations       []OpenAlexLocation   `json:"locations,omitempty"`
	ReferencedWorks []string             `json:"referenced_works,omitempty"`
	RelatedWorks    []string             `json:"related_works,omitempty"`
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

// dataspaceItemContext encapsulates all raw queried data for a single barcode.
type dataspaceItemContext struct {
	barcode       string
	book          *catalog.Book
	inkItems      []InkItem
	openAlexWorks []OpenAlexWork
}
