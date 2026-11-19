package web

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
)

func getSkinFromCookie(c *gin.Context) string {
	if c == nil {
		return "default"
	}
	skin, err := c.Cookie("rfid_skin")
	if err != nil || skin == "" {
		return "default"
	}
	return skin
}

func getThemeModeFromCookie(c *gin.Context) string {
	if c == nil {
		return "auto"
	}
	mode, err := c.Cookie("rfid_theme_mode")
	if err != nil || mode == "" {
		return "auto"
	}
	return mode
}

func (ctrl *Controller) getLoanData(channel ...string) desk.LoanData {
	ch := ""
	if len(channel) > 0 {
		ch = channel[0]
	}
	if ch == "" && len(ctrl.cfg.Channels) > 0 {
		ch = ctrl.cfg.Channels[0]
	}
	staticPath := "static"
	if len(channel) > 0 && channel[0] != "" {
		staticPath = "../static"
	}

	var tags []*desk.TagItem
	if ch != "" {
		tags = ctrl.hub.GetActiveTagsForChannel(ch, "*")
	} else {
		tags = ctrl.hub.GetActiveTags("*")
	}

	successCount := 0
	errorCount := 0
	var failedItems []string
	for _, item := range tags {
		if item.AFIWriteStatus == "success" {
			successCount++
		} else if item.AFIWriteStatus == "error" {
			errorCount++
			label := item.ItemID
			if label == "" {
				label = item.UID
			}
			if item.AFIWriteError != "" {
				failedItems = append(failedItems, fmt.Sprintf("%s (%s)", label, item.AFIWriteError))
			} else {
				failedItems = append(failedItems, label)
			}
		}
	}

	data := desk.LoanData{
		ClientName:        ctrl.cfg.ClientName,
		WSBroker:          ctrl.cfg.WSBroker,
		Channels:          ctrl.cfg.Channels,
		Channel:           ch,
		Addr:              ctrl.cfg.Addr,
		ExtAddr:           ctrl.cfg.ExtAddr,
		Tags:              tags,
		WriteSuccessCount: successCount,
		WriteErrorCount:   errorCount,
		FailedItems:       failedItems,
		Skin:              "default",
		StaticPath:        staticPath,
	}

	data.ThemeLight = ctrl.cfg.ThemeLight
	data.ThemeDark = ctrl.cfg.ThemeDark
	return data
}

// HandleLoan handles GET / and GET /loan to display active NFC objects on the reader.
func (ctrl *Controller) HandleLoan(c *gin.Context) {
	channel := c.Param("channel")
	data := ctrl.getLoanData(channel)
	data.Skin = getSkinFromCookie(c)
	data.ThemeMode = getThemeModeFromCookie(c)
	ctrl.renderHTML(c, http.StatusOK, "dataspace.gohtml", data)
}

// HandleChannelRoot handles GET /:channel/
func (ctrl *Controller) HandleChannelRoot(c *gin.Context) {
	ctrl.HandleLoan(c)
}

// HandleAFIAction executes AFI security modification on active tags on the reader.
func (ctrl *Controller) HandleAFIAction(c *gin.Context) {
	action := strings.ToLower(c.Param("action"))
	channel := c.Param("channel")
	if channel == "" && len(ctrl.cfg.Channels) > 0 {
		channel = ctrl.cfg.Channels[0]
	}

	var targetAFI string
	switch action {
	case "unsecure", "entsichern", "c2":
		targetAFI = libraryclient.AFIUnsecured
	case "secure", "sichern", "07":
		targetAFI = libraryclient.AFISecured
	default:
		c.String(http.StatusBadRequest, "Invalid AFI action: %s", action)
		return
	}

	activeTags := ctrl.hub.GetActiveTagsForChannel(channel, "*")
	if len(activeTags) == 0 {
		ctrl.renderLoanTableSnippet(c, channel, loanMessage{}, message("afi.noTagsFound"))
		return
	}

	if ctrl.rfidClient == nil {
		ctrl.renderLoanTableSnippet(c, channel, loanMessage{}, message("reader.communicationError"))
		return
	}

	var uids []string
	for _, tag := range activeTags {
		if tag.UID != "" {
			uids = append(uids, tag.UID)
		}
	}

	// Mark pending in hub
	ctrl.hub.SetTagAFIPending(uids...)

	req := libraryclient.NewWriteAFIBatchRequest(targetAFI, uids)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	resp, err := ctrl.rfidClient.WriteAFI(ctx, channel, req)
	if err != nil {
		for _, uid := range uids {
			ctrl.hub.SetTagAFIResult(uid, targetAFI, false, err.Error())
		}
		ctrl.renderLoanTableSnippet(c, channel, loanMessage{}, message("afi.writeErrorDetails", err.Error()))
		return
	}

	if resp != nil {
		ctrl.HandleWriteAFIResponse(resp)
	}

	ctrl.renderLoanTableSnippet(c, channel, message("afi.writeSuccess"), loanMessage{})
}

func (ctrl *Controller) renderLoanTableSnippet(c *gin.Context, channel string, success, failure loanMessage) {
	data := ctrl.getLoanData(channel)
	lang := requestLanguage(c.Request)
	data = localizeLoanMessages(data, lang, success, failure)

	tmpl := ctrl.languageTemplates[lang]
	var buf bytes.Buffer
	if tmpl != nil && tmpl.ExecuteTemplate(&buf, "loan_table.gohtml", data) == nil {
		c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
		return
	}
	ctrl.renderHTML(c, http.StatusOK, "loan_table.gohtml", data)
}
