package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/je4/Library-RFID-Apps/pkg/desk"
	"github.com/je4/Library-RFID-Apps/pkg/libraryclient"
)

const sseVariantsMarker = "<!--rfid-dataspace-i18n:"

type loanMessage struct {
	id   string
	args []any
	raw  string
}

func message(id string, args ...any) loanMessage { return loanMessage{id: id, args: args} }

func afiStatusMessage(afi string) loanMessage {
	switch libraryclient.AFISecurityStatus(afi) {
	case "Gesichert":
		return message("afi.secured")
	case "Entsichert":
		return message("afi.unsecured")
	default:
		return loanMessage{}
	}
}

func (m loanMessage) text(lang string) string {
	if m.raw != "" {
		return m.raw
	}
	if m.id == "" {
		return ""
	}
	args := make([]any, len(m.args))
	for i, arg := range m.args {
		if nested, ok := arg.(loanMessage); ok {
			args[i] = nested.text(lang)
		} else {
			args[i] = arg
		}
	}
	text := translate(lang, m.id)
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

func loanCountMessages(successCount, errorCount int) (loanMessage, loanMessage) {
	if errorCount > 0 && successCount > 0 {
		return message("afi.itemsUpdatedCount", successCount, successCount+errorCount), message("afi.itemsFailedCount", errorCount, successCount+errorCount)
	}
	if errorCount > 0 {
		return loanMessage{}, message("afi.writeErrors", errorCount)
	}
	return loanMessage{}, loanMessage{}
}

func localizeLoanMessages(data desk.LoanData, lang string, success, failure loanMessage) desk.LoanData {
	data.SuccessMessage = success.text(lang)
	data.ErrorMessage = failure.text(lang)
	return data
}

// Pack variants alongside the German payload; the HTTP stream removes this metadata.
func packSSEVariants(data string, variants map[string]string, jsonPayload bool) string {
	encoded, err := json.Marshal(variants)
	if err != nil || len(variants) == 0 {
		return data
	}
	value := base64.StdEncoding.EncodeToString(encoded)
	if jsonPayload && strings.HasSuffix(data, "}") {
		return data[:len(data)-1] + `,"_rfidDataspaceI18n":"` + value + `"}`
	}
	return data + sseVariantsMarker + value + "-->"
}

func localizedSSEData(event, data, lang string) string {
	if (event == "tag_read" || event == "loan_tags" || event == "dataspace_tags") && strings.Contains(data, sseVariantsMarker) && strings.HasSuffix(data, "-->") {
		i := strings.LastIndex(data, sseVariantsMarker)
		encoded := strings.TrimSuffix(data[i+len(sseVariantsMarker):], "-->")
		data = data[:i]
		if decoded, err := base64.StdEncoding.DecodeString(encoded); err == nil {
			var variants map[string]string
			if json.Unmarshal(decoded, &variants) == nil && variants[lang] != "" {
				return variants[lang]
			}
		}
	}
	return data
}

// HandleEventsStream handles SSE subscription for live events.
func (ctrl *Controller) HandleEventsStream(c *gin.Context) {
	channel := c.Param("channel")
	lang := requestLanguage(c.Request)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	msgChan := ctrl.hub.Subscribe(channel)
	defer ctrl.hub.Unsubscribe(msgChan)

	// Send initial connection event
	c.SSEvent("connected", map[string]string{
		"time":    time.Now().Format(time.RFC3339),
		"channel": channel,
	})
	c.Writer.Flush()

	notify := c.Request.Context().Done()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-notify:
			return
		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			eventData := localizedSSEData(msg.Event, msg.Data, lang)
			c.SSEvent(msg.Event, eventData)
			c.Writer.Flush()
		case <-ticker.C:
			c.SSEvent("ping", time.Now().Unix())
			c.Writer.Flush()
		}
	}
}

// BroadcastLoanTags formats and sends updated loan/dataspace table HTML to all SSE subscribers.
func (ctrl *Controller) BroadcastLoanTags(channel string, success, failure loanMessage) {
	variants := make(map[string]string)
	for _, lang := range []string{"de", "en", "fr", "it"} {
		loanData := ctrl.getLoanData(channel)
		loanData = localizeLoanMessages(loanData, lang, success, failure)
		var buf bytes.Buffer
		tmpl := ctrl.languageTemplates[lang]
		if tmpl == nil {
			continue
		}
		if err := tmpl.ExecuteTemplate(&buf, "loan_table.gohtml", loanData); err == nil {
			variants[lang] = buf.String()
		}
	}
	if len(variants) > 0 {
		ctrl.hub.Broadcast(&desk.SSEMessage{
			Event: "loan_tags",
			Data:  packSSEVariants(variants["de"], variants, false),
		}, channel)
	}
}

func (ctrl *Controller) BroadcastWSActivity() {
	ctrl.hub.Broadcast(&desk.SSEMessage{
		Event: "ws_activity",
		Data:  "1",
	})
}
