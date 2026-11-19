package web

import (
	"context"
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"emperror.dev/errors"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/je4/Library-RFID-Dataspace/data"
	"github.com/je4/utils/v2/pkg/zLogger"
)

// Config defines settings for the rfid-dataspace web server.
type Config struct {
	Addr        string
	ExtAddr     string
	ClientName  string
	WSBroker    string
	Channels    []string
	DefaultISIL string
	DefaultAFI  string
	ThemeLight  string
	ThemeDark   string
	Hub         *desk.Hub
	RFIDClient  libraryclient.Client
	ISILList    map[string]*desk.ISILEntry
	Logger      zLogger.ZLogger
}

// Controller manages HTTP routes, SSE streaming, and embedded frontend rendering.
type Controller struct {
	cfg               Config
	router            *gin.Engine
	server            http.Server
	languageTemplates map[string]*template.Template
	logger            zLogger.ZLogger
	hub               *desk.Hub
	rfidClient        libraryclient.Client
}

// NewController creates and initializes the Web Controller instance.
func NewController(cfg Config) (*Controller, error) {
	if cfg.Hub == nil {
		return nil, errors.New("desk Hub cannot be nil")
	}

	if cfg.ISILList != nil && cfg.Hub.GetISILList() == nil {
		cfg.Hub.SetISILList(cfg.ISILList)
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	ctrl := &Controller{
		cfg:        cfg,
		router:     router,
		logger:     cfg.Logger,
		hub:        cfg.Hub,
		rfidClient: cfg.RFIDClient,
	}

	// Compile templates for all supported languages
	ctrl.languageTemplates = make(map[string]*template.Template)
	for _, lang := range []string{"de", "en", "fr", "it"} {
		tmpl, err := template.New("").Funcs(templateFunctions(lang)).ParseFS(data.WebFS, "web/templates/*.gohtml")
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse templates for language %s", lang)
		}
		ctrl.languageTemplates[lang] = tmpl
	}

	// CORS configuration
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Accept-Language", "Accept"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Static assets route
	staticFS, err := fs.Sub(data.WebFS, "web/static")
	if err != nil {
		return nil, errors.Wrap(err, "cannot create sub filesystem for static assets")
	}
	router.StaticFS("/static", http.FS(staticFS))

	// Root and core routes
	router.GET("/", ctrl.HandleLoan)
	router.GET("/loan", ctrl.HandleLoan)
	router.GET("/dataspace", ctrl.HandleLoan)
	router.GET("/events", ctrl.HandleEventsStream)
	router.POST("/loan/afi/:action", ctrl.HandleAFIAction)
	router.POST("/dataspace/afi/:action", ctrl.HandleAFIAction)

	// Channel-scoped routes
	channelGroup := router.Group("/:channel")
	{
		channelGroup.GET("", ctrl.HandleChannelRoot)
		channelGroup.GET("/", ctrl.HandleChannelRoot)
		channelGroup.GET("/loan", ctrl.HandleLoan)
		channelGroup.GET("/dataspace", ctrl.HandleLoan)
		channelGroup.GET("/events", ctrl.HandleEventsStream)
		channelGroup.POST("/loan/afi/:action", ctrl.HandleAFIAction)
		channelGroup.POST("/dataspace/afi/:action", ctrl.HandleAFIAction)
		channelGroup.StaticFS("/static", http.FS(staticFS))
	}

	ctrl.server = http.Server{
		Addr:    cfg.Addr,
		Handler: router,
	}

	return ctrl, nil
}

// SetRFIDClient assigns or updates the rfidclient instance.
func (ctrl *Controller) SetRFIDClient(client libraryclient.Client) {
	ctrl.rfidClient = client
}

// Start launches the HTTP server in a separate goroutine.
func (ctrl *Controller) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		if ctrl.logger != nil {
			ctrl.logger.Info().Str("addr", ctrl.cfg.Addr).Msg("starting HTTP server for rfid-dataspace")
		}
		if err := ctrl.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if ctrl.logger != nil {
				ctrl.logger.Error().Err(err).Msg("HTTP server terminated unexpectedly")
			}
		}
	}()
}

// GracefulStop cleanly terminates the HTTP server.
func (ctrl *Controller) GracefulStop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ctrl.server.Shutdown(ctx); err != nil {
		if ctrl.logger != nil {
			ctrl.logger.Error().Err(err).Msg("error during HTTP server shutdown")
		}
	}
}

// HandleTagRead processes incoming tag scans from wsbroker / rfidclient.
func (ctrl *Controller) HandleTagRead(tag *libraryclient.TagReadEvent) {
	if tag == nil {
		return
	}
	ctrl.hub.AddTag(tag)
	ctrl.BroadcastLoanTags(tag.Channel, loanMessage{}, loanMessage{})
}

// HandleTagsCleared processes antenna cleared events when media is removed.
func (ctrl *Controller) HandleTagsCleared() {
	ctrl.hub.ClearActive()
	ctrl.BroadcastLoanTags("", loanMessage{}, loanMessage{})
}

// HandleWSActivity triggers WebSocket live traffic indicator blink.
func (ctrl *Controller) HandleWSActivity() {
	ctrl.BroadcastWSActivity()
}

// HandleWriteAFIResponse processes responses from AFI security writes.
func (ctrl *Controller) HandleWriteAFIResponse(resp *libraryclient.WriteAFIResponse) {
	if resp == nil {
		return
	}

	channel := resp.Channel
	targetAFI := resp.Data.AFI
	errMsg := resp.Data.GetError()

	if len(resp.Data.SuccessfulUIDs) > 0 {
		for _, uid := range resp.Data.SuccessfulUIDs {
			ctrl.hub.SetTagAFIResult(uid, targetAFI, true, "")
		}
	}
	if len(resp.Data.FailedUIDs) > 0 {
		for uid, failErr := range resp.Data.FailedUIDs {
			ctrl.hub.SetTagAFIResult(uid, targetAFI, false, failErr)
		}
	}
	if len(resp.Data.SuccessfulUIDs) == 0 && len(resp.Data.FailedUIDs) == 0 && resp.Data.UID != "" {
		ctrl.hub.SetTagAFIResult(resp.Data.UID, targetAFI, resp.Data.IsSuccess(), errMsg)
	}

	successMsg, errorMsg := loanCountMessages(len(resp.Data.SuccessfulUIDs), len(resp.Data.FailedUIDs))
	if !resp.Data.IsSuccess() && len(resp.Data.FailedUIDs) == 0 && errMsg != "" {
		errorMsg = message("afi.writeErrorDetails", errMsg)
	} else if resp.Data.IsSuccess() && len(resp.Data.SuccessfulUIDs) == 0 {
		successMsg = message("afi.writeSuccess")
	}

	ctrl.BroadcastLoanTags(channel, successMsg, errorMsg)
}
