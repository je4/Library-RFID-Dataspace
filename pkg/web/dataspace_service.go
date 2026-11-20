package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/catalog"
	"github.com/maypok86/otter/v2"
)

type dataspaceGraphLoader struct {
	svc *dataspaceService
}

func (l dataspaceGraphLoader) Load(ctx context.Context, key string) (DataspaceGraphResponse, error) {
	// key format: "graph:DIRECTION:bc1,bc2,..."
	parts := strings.SplitN(strings.TrimPrefix(key, "graph:"), ":", 2)
	dir := parts[0]
	var barcodes []string
	if len(parts) > 1 && parts[1] != "" {
		barcodes = strings.Split(parts[1], ",")
	}
	return l.svc.buildGraph(ctx, barcodes, dir)
}

func (l dataspaceGraphLoader) Reload(ctx context.Context, key string, _ DataspaceGraphResponse) (DataspaceGraphResponse, error) {
	return l.Load(ctx, key)
}

type dataspaceReportLoader struct {
	svc *dataspaceService
}

func (l dataspaceReportLoader) Load(ctx context.Context, key string) (DataspaceReportResponse, error) {
	// key format: "report:bc1,bc2,..."
	rawBarcodes := strings.TrimPrefix(key, "report:")
	var barcodes []string
	if rawBarcodes != "" {
		barcodes = strings.Split(rawBarcodes, ",")
	}
	return l.svc.buildReport(ctx, barcodes)
}

func (l dataspaceReportLoader) Reload(ctx context.Context, key string, _ DataspaceReportResponse) (DataspaceReportResponse, error) {
	return l.Load(ctx, key)
}

type dataspaceService struct {
	catalogSearch  func(context.Context, string) ([]catalog.Book, error)
	inkSearch      InkSearchFunc
	openAlexSearch OpenAlexSearchFunc
	graphCache     *otter.Cache[string, DataspaceGraphResponse]
	graphLoader    dataspaceGraphLoader
	reportCache    *otter.Cache[string, DataspaceReportResponse]
	reportLoader   dataspaceReportLoader
}

func newDataspaceService(catalogSearch func(context.Context, string) ([]catalog.Book, error), inkSearch InkSearchFunc, openAlexSearch OpenAlexSearchFunc) *dataspaceService {
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
	svc.graphLoader = dataspaceGraphLoader{svc: svc}
	svc.reportLoader = dataspaceReportLoader{svc: svc}
	return svc
}

// resolveDataspaceData executes unified querying across Alma, InK, and OpenAlex.
func (s *dataspaceService) resolveDataspaceData(ctx context.Context, cleanBarcodes []string) ([]dataspaceItemContext, error) {
	items := make([]dataspaceItemContext, 0, len(cleanBarcodes))

	for _, bc := range cleanBarcodes {
		itCtx := dataspaceItemContext{barcode: bc}

		if s.catalogSearch != nil {
			books, err := s.catalogSearch(ctx, bc)
			if err == nil && len(books) > 0 {
				itCtx.book = &books[0]
			}
		}

		// Gather search keys for InK and OpenAlex
		var authors []string
		var title string
		var isbns []string
		var dois []string

		if itCtx.book != nil {
			book := itCtx.book
			if len(book.Titles) > 0 {
				title = book.Titles[0].Title
			}
			for _, nameMap := range book.PersonalNames {
				for _, nameEntry := range nameMap {
					if nameEntry.NamePart != "" {
						authors = append(authors, nameEntry.NamePart)
					}
				}
			}
			for _, isbn := range book.ISBNs {
				if isbn.Enriched != "" {
					isbns = append(isbns, isbn.Enriched)
				} else if isbn.ID != "" {
					isbns = append(isbns, isbn.ID)
				}
			}
			for _, otherId := range book.Identifiers {
				otherId = strings.TrimSpace(otherId)
				if strings.HasPrefix(strings.ToLower(otherId), "doi:") || strings.Contains(otherId, "10.") {
					dois = append(dois, otherId)
				}
			}
		}

		// Search InK
		if s.inkSearch != nil {
			inkItems, err := s.inkSearch(ctx, bc, authors, title, isbns)
			if err == nil && len(inkItems) > 0 {
				itCtx.inkItems = inkItems
			}
		}

		// Search OpenAlex
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

func (ctrl *Controller) extractBarcodesFromRequest(c *gin.Context) []string {
	barcodesParam := strings.TrimSpace(c.Query("barcodes"))
	var barcodes []string
	if barcodesParam != "" {
		for _, b := range strings.Split(barcodesParam, ",") {
			b = strings.TrimSpace(b)
			if b != "" {
				barcodes = append(barcodes, b)
			}
		}
	} else {
		// Use active tags on reader channel
		channel := c.Param("channel")
		if channel == "" && len(ctrl.cfg.Channels) > 0 {
			channel = ctrl.cfg.Channels[0]
		}
		activeTags := ctrl.hub.GetActiveTagsForChannel(channel, "*")
		for _, t := range activeTags {
			if t.ItemID != "" {
				barcodes = append(barcodes, t.ItemID)
			}
		}
	}
	return barcodes
}
