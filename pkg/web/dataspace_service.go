package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/maypok86/otter/v2"
)

// dataspaceService orchestrates querying, caching, and synthesizing bibliographic
// metadata from Alma library catalog, digital repository items from Mediathek InK,
// and scholarly citation / taxonomy data from OpenAlex.
type dataspaceService struct {
	catalogSearch  func(context.Context, string) ([]catalog.Book, error)
	inkSearch      InkSearchFunc
	openAlexSearch OpenAlexSearchFunc
	graphCache     *otter.Cache[string, DataspaceGraphResponse]
	graphLoader    funcCacheLoader[string, DataspaceGraphResponse]
	reportCache    *otter.Cache[string, DataspaceReportResponse]
	reportLoader   funcCacheLoader[string, DataspaceReportResponse]
}

// newDataspaceService initializes and configures the dataspace service with caching.
func newDataspaceService(
	catalogSearch func(context.Context, string) ([]catalog.Book, error),
	inkSearch InkSearchFunc,
	openAlexSearch OpenAlexSearchFunc,
) *dataspaceService {
	svc := &dataspaceService{
		catalogSearch:  catalogSearch,
		inkSearch:      inkSearch,
		openAlexSearch: openAlexSearch,
		graphCache: otter.Must(&otter.Options[string, DataspaceGraphResponse]{
			MaximumSize:      2000,
			ExpiryCalculator: otter.ExpiryWriting[string, DataspaceGraphResponse](5 * time.Minute),
		}),
		reportCache: otter.Must(&otter.Options[string, DataspaceReportResponse]{
			MaximumSize:      2000,
			ExpiryCalculator: otter.ExpiryWriting[string, DataspaceReportResponse](5 * time.Minute),
		}),
	}

	svc.graphLoader = newFuncCacheLoader(func(ctx context.Context, key string) (DataspaceGraphResponse, error) {
		// Key format: "graph:DIRECTION:bc1,bc2,..."
		parts := strings.SplitN(strings.TrimPrefix(key, "graph:"), ":", 2)
		dir := parts[0]
		var barcodes []string
		if len(parts) > 1 && parts[1] != "" {
			barcodes = strings.Split(parts[1], ",")
		}
		return svc.buildGraph(ctx, barcodes, dir)
	})

	svc.reportLoader = newFuncCacheLoader(func(ctx context.Context, key string) (DataspaceReportResponse, error) {
		// Key format: "report:bc1,bc2,..."
		rawBarcodes := strings.TrimPrefix(key, "report:")
		var barcodes []string
		if rawBarcodes != "" {
			barcodes = strings.Split(rawBarcodes, ",")
		}
		return svc.buildReport(ctx, barcodes)
	})

	return svc
}

// resolveDataspaceData executes unified querying across Alma, InK, and OpenAlex for each barcode.
func (s *dataspaceService) resolveDataspaceData(ctx context.Context, cleanBarcodes []string) ([]dataspaceItemContext, error) {
	items := make([]dataspaceItemContext, 0, len(cleanBarcodes))

	for _, bc := range cleanBarcodes {
		itCtx := dataspaceItemContext{barcode: bc}

		// 1. Resolve Alma Catalog Record
		if s.catalogSearch != nil {
			books, err := s.catalogSearch(ctx, bc)
			if err == nil && len(books) > 0 {
				itCtx.book = &books[0]
			}
		}

		// 2. Extract search keys for InK and OpenAlex using catalog extraction helpers
		var authors []string
		var title string
		var isbns []string
		var dois []string

		if itCtx.book != nil {
			title, _ = extractBookTitle(itCtx.book, "")
			authors = mapSlice(extractCatalogAuthors(itCtx.book), func(a CatalogAuthorInfo) string { return a.Name })
			isbns = extractAllISBNs(itCtx.book)
			dois = extractBookDOIs(itCtx.book)
		}

		// 3. Search Mediathek InK digital repository
		if s.inkSearch != nil {
			inkItems, err := s.inkSearch(ctx, bc, authors, title, isbns)
			if err == nil && len(inkItems) > 0 {
				itCtx.inkItems = inkItems
			}
		}

		// 4. Search OpenAlex scholarly knowledge graph
		if s.openAlexSearch != nil && (len(isbns) > 0 || title != "" || len(authors) > 0 || len(dois) > 0) {
			oaWorks, err := s.openAlexSearch(ctx, isbns, title, authors, dois)
			if err == nil && len(oaWorks) > 0 {
				itCtx.openAlexWorks = oaWorks
			}
		}

		items = append(items, itCtx)
	}

	return items, nil
}

// handleDataspaceGraph handles HTTP GET requests to /api/dataspace/graph and /api/dataspace/graph/:channel.
func (ctrl *Controller) handleDataspaceGraph(c *gin.Context) {
	if ctrl.dataspaceService == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	barcodes := ctrl.extractBarcodesFromRequest(c)
	direction := c.DefaultQuery("direction", "TD")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := ctrl.dataspaceService.GenerateMermaid(ctx, barcodes, direction)
	if err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Warn().Err(err).Msg("dataspace graph generation failed")
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleDataspaceReport handles HTTP GET requests to /api/dataspace/report and /api/dataspace/report/:channel.
func (ctrl *Controller) handleDataspaceReport(c *gin.Context) {
	if ctrl.dataspaceService == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}

	barcodes := ctrl.extractBarcodesFromRequest(c)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := ctrl.dataspaceService.GenerateReport(ctx, barcodes)
	if err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Warn().Err(err).Msg("dataspace report generation failed")
		}
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// extractBarcodesFromRequest extracts, cleans, and deduplicates barcodes from either the
// "barcodes" query parameter (comma-separated) or active RFID tags on the antenna channel.
func (ctrl *Controller) extractBarcodesFromRequest(c *gin.Context) []string {
	barcodesParam := strings.TrimSpace(c.Query("barcodes"))
	if barcodesParam != "" {
		return cleanStrings(strings.Split(barcodesParam, ","))
	}

	// Retrieve active RFID tags on the specified desk channel
	channel := c.Param("channel")
	if channel == "" && len(ctrl.cfg.Channels) > 0 {
		channel = ctrl.cfg.Channels[0]
	}
	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, "*")
	rawBarcodes := mapSlice(activeTags, func(t *desk.TagItem) string {
		if t != nil {
			return t.ItemID
		}
		return ""
	})
	return cleanStrings(rawBarcodes)
}
