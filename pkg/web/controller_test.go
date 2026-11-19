package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
	"github.com/rs/zerolog"
)

func setupTestController(t *testing.T) (*Controller, *desk.Hub) {
	t.Helper()
	logger := zerolog.Nop()
	hub := desk.NewHub(100)
	hubCtx, hubCancel := context.WithCancel(context.Background())
	t.Cleanup(hubCancel)
	hub.Start(hubCtx)

	cfg := Config{
		Addr:        "localhost:5000",
		ExtAddr:     "http://localhost:5000",
		ClientName:  "rfid-dataspace-test",
		WSBroker:    "websocket-broker.k8s-tls.ub.unibas.ch",
		Channels:    []string{"mediathek01"},
		DefaultISIL: "CH-001596-0",
		DefaultAFI:  "07",
		Hub:         hub,
		Logger:      &logger,
	}

	ctrl, err := NewController(cfg)
	if err != nil {
		t.Fatalf("failed to create web controller: %v", err)
	}
	return ctrl, hub
}

func TestController_RootPageRendersNFCList(t *testing.T) {
	ctrl, hub := setupTestController(t)

	// Add mock tag
	now := time.Now()
	parts := 1
	partNo := 1
	usageType := 1
	mockTag := &libraryclient.TagReadEvent{
		ID:        "evt-01",
		Channel:   "mediathek01",
		Source:    "reader-01",
		Timestamp: now,
		Data: libraryclient.TagReadData{
			UID:       "E0040100DEADBEEF",
			ItemID:    "M987654321",
			ISIL:      "CH-001596-0",
			AFI:       "07",
			ReaderID:  "reader-01",
			UsageType: &usageType,
			Parts:     &parts,
			PartNo:    &partNo,
		},
	}
	hub.AddTag(mockTag)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	ctrl.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "M987654321") {
		t.Errorf("expected tag ItemID 'M987654321' in rendered page, got: %s", body)
	}
	if !strings.Contains(body, "E0040100DEADBEEF") {
		t.Errorf("expected tag UID 'E0040100DEADBEEF' in rendered page, got: %s", body)
	}
	if !strings.Contains(body, "dataspace.css") {
		t.Errorf("expected reference to dataspace.css stylesheet")
	}
	if !strings.Contains(body, "htmx-4.0.js") {
		t.Errorf("expected reference to htmx-4.0.js")
	}
}

func TestController_ChannelPageRendersNFCList(t *testing.T) {
	ctrl, hub := setupTestController(t)

	now := time.Now()
	mockTag := &libraryclient.TagReadEvent{
		ID:        "evt-02",
		Channel:   "mediathek01",
		Timestamp: now,
		Data: libraryclient.TagReadData{
			UID:      "E0040100CAFE1234",
			ItemID:   "BOOK-100",
			ISIL:     "CH-001596-0",
			AFI:      "C2",
			ReaderID: "reader-01",
		},
	}
	hub.AddTag(mockTag)

	req := httptest.NewRequest(http.MethodGet, "/mediathek01/", nil)
	w := httptest.NewRecorder()
	ctrl.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "BOOK-100") {
		t.Errorf("expected tag ItemID 'BOOK-100' in rendered channel page")
	}
}

func TestController_StaticAssets(t *testing.T) {
	ctrl, _ := setupTestController(t)

	// Test CSS asset
	req := httptest.NewRequest(http.MethodGet, "/static/scss/dataspace.css", nil)
	w := httptest.NewRecorder()
	ctrl.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for static/scss/dataspace.css, got %d", w.Code)
	}

	// Test HTMX JS
	req = httptest.NewRequest(http.MethodGet, "/static/js/htmx-4.0.js", nil)
	w = httptest.NewRecorder()
	ctrl.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for static/js/htmx-4.0.js, got %d", w.Code)
	}
}

func TestController_AFIAction_Empty(t *testing.T) {
	ctrl, _ := setupTestController(t)

	req := httptest.NewRequest(http.MethodPost, "/loan/afi/unsecure", nil)
	w := httptest.NewRecorder()
	ctrl.router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for afi action response snippet, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "loan-table-row-empty") && !strings.Contains(body, "Keine Medien") && !strings.Contains(body, "table") {
		t.Errorf("expected empty table row snippet, got %s", body)
	}
}
