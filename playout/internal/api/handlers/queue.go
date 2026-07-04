package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/Waelson/radio-playout-engine/internal/commands"
	"github.com/Waelson/radio-playout-engine/internal/queue"
)

// cmdTimeout is the maximum time an API handler waits for the Dispatcher to
// accept or reject a queue command.
const cmdTimeout = 2 * time.Second

// queueBus is the subset of commands.Bus used by queue handlers.
type queueBus interface {
	Send(ctx context.Context, cmd commands.Command) error
}

// queueReader is the subset of queue.Manager used for read operations.
type queueReader interface {
	Current() *queue.QueueItem
	ListPending() []*queue.QueueItem
	Size() int
}

// --- Request / response types ------------------------------------------------

type enqueueRequest struct {
	Items []queueItemInput `json:"items"`
}

type insertNextRequest struct {
	Item queueItemInput `json:"item"`
}

type insertAfterRequest struct {
	AfterQueueItemID string         `json:"after_queue_item_id"`
	Item             queueItemInput `json:"item"`
}

type clearQueueRequest struct {
	PreserveCurrent *bool `json:"preserve_current"`
}

// queueItemInput mirrors commands.QueueItemInput for JSON decoding.
type queueItemInput struct {
	AssetID    string            `json:"asset_id"`
	Path       string            `json:"path"`
	Type       string            `json:"type"`
	Title      string            `json:"title"`
	Artist     string            `json:"artist"`
	DurationMS int64             `json:"duration_ms"`
	CueInMS    int64             `json:"cue_in_ms"`
	CueOutMS   int64             `json:"cue_out_ms"`
	GainDB     float64           `json:"gain_db"`
	Transition *transitionInput  `json:"transition,omitempty"`
	Mandatory  bool              `json:"mandatory"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type transitionInput struct {
	Type       string `json:"type"`
	DurationMS int64  `json:"duration_ms"`
}

type cmdResponse struct {
	OK        bool   `json:"ok"`
	CommandID string `json:"command_id,omitempty"`
	Accepted  bool   `json:"accepted"`
	Reason    string `json:"reason,omitempty"`
	QueueSize *int   `json:"queue_size,omitempty"`
}

// queueListResponse is the shape of GET /v1/queue.
// pending is a polymorphic list: each entry is either a plain item
// (kind:"item") or a grouped commercial break (kind:"break").
type queueListResponse struct {
	Current    *queueItemView `json:"current"`
	Pending    []pendingEntry `json:"pending"`
	Total      int            `json:"total"`
	BreakCount int            `json:"break_count"`
}

type pendingEntry struct {
	Kind  string         `json:"kind"` // "item" | "break"
	Item  *queueItemView `json:"item,omitempty"`
	Break *breakView     `json:"break,omitempty"`
}

type breakView struct {
	BreakID         string         `json:"break_id"`
	Title           string         `json:"title"`
	Status          string         `json:"status"` // "QUEUED" | "PLAYING"
	TotalDurationMS int64          `json:"total_duration_ms"`
	Items           []breakSubItem `json:"items"`
}

type breakSubItem struct {
	BreakRole string        `json:"break_role"`
	Item      queueItemView `json:"item"`
}

type transitionView struct {
	Type       string `json:"type"`
	DurationMS int64  `json:"duration_ms"`
}

type queueItemView struct {
	QueueItemID string            `json:"queue_item_id"`
	AssetID     string            `json:"asset_id"`
	Path        string            `json:"path"`
	Title       string            `json:"title"`
	Artist      string            `json:"artist"`
	Type        string            `json:"type"`
	DurationMS  int64             `json:"duration_ms"`
	CueInMS     int64             `json:"cue_in_ms"`
	CueOutMS    int64             `json:"cue_out_ms"`
	Mandatory   bool              `json:"mandatory"`
	Status      string            `json:"status"`
	Transition  *transitionView   `json:"transition"`
	Metadata    map[string]string `json:"metadata"`
}

// --- Helpers -----------------------------------------------------------------

// toCommandItem converts a handler-level queueItemInput to commands.QueueItemInput.
func toCommandItem(inp queueItemInput) commands.QueueItemInput {
	out := commands.QueueItemInput{
		AssetID:    inp.AssetID,
		Path:       inp.Path,
		Type:       inp.Type,
		Title:      inp.Title,
		Artist:     inp.Artist,
		DurationMS: inp.DurationMS,
		CueInMS:    inp.CueInMS,
		CueOutMS:   inp.CueOutMS,
		GainDB:     inp.GainDB,
		Mandatory:  inp.Mandatory,
		Metadata:   inp.Metadata,
	}
	if inp.Transition != nil {
		out.Transition = &commands.TransitionInput{
			Type:       inp.Transition.Type,
			DurationMS: inp.Transition.DurationMS,
		}
	}
	return out
}

// validateItem returns a non-empty reason string when the item is invalid.
func validateItem(inp queueItemInput) string {
	// HORA_CERTA items have no fixed path — the engine resolves it at play time.
	if inp.Type != "HORA_CERTA" && inp.Path == "" {
		return "field path is required"
	}
	return ""
}

// sendAndWait sends a sync command to the bus and waits up to cmdTimeout for
// the Dispatcher's acceptance decision. Returns the Result or a timeout error.
func sendAndWait(w http.ResponseWriter, bus queueBus, cmd commands.Command, replyCh <-chan commands.Result) (commands.Result, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	if err := bus.Send(ctx, cmd); err != nil {
		writeError(w, http.StatusServiceUnavailable, "bus_unavailable", "command bus is full or unavailable")
		return commands.Result{}, false
	}

	select {
	case result := <-replyCh:
		return result, true
	case <-ctx.Done():
		writeError(w, http.StatusGatewayTimeout, "command_timeout", "engine did not respond in time")
		return commands.Result{}, false
	}
}

// toItemView converts a *queue.QueueItem to a queueItemView for JSON output.
func toItemView(it *queue.QueueItem) queueItemView {
	v := queueItemView{
		QueueItemID: it.QueueItemID,
		AssetID:     it.AssetID,
		Path:        it.Path,
		Title:       it.Title,
		Artist:      it.Artist,
		Type:        string(it.Type),
		DurationMS:  it.DurationMS,
		CueInMS:     it.CueInMS,
		CueOutMS:    it.CueOutMS,
		Mandatory:   it.Mandatory,
		Status:      string(it.Status),
		Metadata:    it.Metadata,
	}
	if it.Transition.Type != "" {
		v.Transition = &transitionView{
			Type:       string(it.Transition.Type),
			DurationMS: it.Transition.DurationMS,
		}
	}
	return v
}

// --- Handlers ----------------------------------------------------------------

// QueueList returns a handler for GET /v1/queue.
// The response separates the currently-playing item (current) from the pending
// queue. Items that belong to a commercial break are grouped into a break block
// (kind:"break"); standalone items appear as kind:"item".
func QueueList(qMgr queueReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cur := qMgr.Current()
		pendingItems := qMgr.ListPending()

		// Build the current view.
		var currentView *queueItemView
		if cur != nil {
			v := toItemView(cur)
			currentView = &v
		}

		// Build the polymorphic pending list, grouping by BreakID.
		pending := make([]pendingEntry, 0, len(pendingItems))
		breakCount := 0
		total := 0
		if cur != nil {
			total++
		}
		total += len(pendingItems)

		i := 0
		for i < len(pendingItems) {
			it := pendingItems[i]
			if it.BreakID == "" {
				v := toItemView(it)
				pending = append(pending, pendingEntry{Kind: "item", Item: &v})
				i++
				continue
			}
			// Collect all consecutive pending items belonging to the same break.
			breakID := it.BreakID
			var subItems []breakSubItem
			var totalDurMS int64
			for i < len(pendingItems) && pendingItems[i].BreakID == breakID {
				sub := pendingItems[i]
				sv := toItemView(sub)
				subItems = append(subItems, breakSubItem{BreakRole: sub.BreakRole, Item: sv})
				totalDurMS += sub.DurationMS
				i++
			}
			// Determine break status: PLAYING if the current item is in this break.
			status := "QUEUED"
			if cur != nil && cur.BreakID == breakID {
				status = "PLAYING"
			}
			bv := breakView{
				BreakID:         breakID,
				Title:           it.BreakTitle,
				Status:          status,
				TotalDurationMS: totalDurMS,
				Items:           subItems,
			}
			pending = append(pending, pendingEntry{Kind: "break", Break: &bv})
			breakCount++
		}

		writeJSON(w, http.StatusOK, queueListResponse{
			Current:    currentView,
			Pending:    pending,
			Total:      total,
			BreakCount: breakCount,
		})
	}
}

// Enqueue returns a handler for POST /v1/queue/enqueue.
func Enqueue(bus queueBus, qMgr queueReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req enqueueRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
			return
		}
		if len(req.Items) == 0 {
			writeError(w, http.StatusBadRequest, "invalid_payload", "field items must not be empty")
			return
		}
		for _, it := range req.Items {
			if reason := validateItem(it); reason != "" {
				writeError(w, http.StatusBadRequest, "invalid_payload", reason)
				return
			}
		}

		inputs := make([]commands.QueueItemInput, len(req.Items))
		for i, it := range req.Items {
			inputs[i] = toCommandItem(it)
		}

		cmd, replyCh := commands.NewSync(commands.CmdEnqueue, commands.EnqueuePayload{Items: inputs})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}

		size := qMgr.Size()
		resp := cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
			QueueSize: &size,
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// enqueueBreakRequest is the JSON body for POST /v1/queue/enqueue-break.
type enqueueBreakRequest struct {
	Title string           `json:"title"`
	Open  *queueItemInput  `json:"open,omitempty"`
	Spots []queueItemInput `json:"spots"`
	Close *queueItemInput  `json:"close,omitempty"`
}

// EnqueueBreak returns a handler for POST /v1/queue/enqueue-break.
// It expands the break into flat QueueItems via CmdEnqueueBreak and returns
// the pre-computed break_id so the caller can track the block.
func EnqueueBreak(bus queueBus, qMgr queueReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req enqueueBreakRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
			return
		}
		if len(req.Spots) == 0 {
			writeError(w, http.StatusBadRequest, "missing_spots", "spots list must have at least one item")
			return
		}
		if req.Title == "" {
			writeError(w, http.StatusBadRequest, "missing_title", "title is required")
			return
		}
		for i, s := range req.Spots {
			if s.Path == "" {
				writeError(w, http.StatusBadRequest, "missing_path",
					"field path is required for spot at index "+itoa(i))
				return
			}
		}

		// Generate the BreakID here so we can include it in the response.
		breakID := "brk_" + ulid.Make().String()

		// Compute total_items and total_duration_ms for the response.
		totalItems := len(req.Spots)
		var totalDurationMS int64
		for _, s := range req.Spots {
			totalDurationMS += s.DurationMS
		}
		var openInput *commands.QueueItemInput
		if req.Open != nil {
			v := toCommandItem(*req.Open)
			openInput = &v
			totalItems++
			totalDurationMS += req.Open.DurationMS
		}
		var closeInput *commands.QueueItemInput
		if req.Close != nil {
			v := toCommandItem(*req.Close)
			closeInput = &v
			totalItems++
			totalDurationMS += req.Close.DurationMS
		}
		spots := make([]commands.QueueItemInput, len(req.Spots))
		for i, s := range req.Spots {
			spots[i] = toCommandItem(s)
		}

		payload := commands.EnqueueBreakPayload{
			BreakID: breakID,
			Break: commands.BreakItemInput{
				Title: req.Title,
				Open:  openInput,
				Spots: spots,
				Close: closeInput,
			},
		}

		cmd, replyCh := commands.NewSync(commands.CmdEnqueueBreak, payload)
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if result.Accepted {
			w.WriteHeader(http.StatusAccepted)
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"ok":                result.Accepted,
			"accepted":          result.Accepted,
			"break_id":          breakID,
			"title":             req.Title,
			"total_items":       totalItems,
			"total_duration_ms": totalDurationMS,
			"command_id":        result.CommandID,
			"reason":            result.Reason,
			"queue_size":        qMgr.Size(),
		})
	}
}

// itoa converts an int to string without importing strconv at the call site.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// InsertNext returns a handler for POST /v1/queue/insert-next.
func InsertNext(bus queueBus, qMgr queueReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req insertNextRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
			return
		}
		if reason := validateItem(req.Item); reason != "" {
			writeError(w, http.StatusBadRequest, "invalid_payload", reason)
			return
		}

		cmd, replyCh := commands.NewSync(commands.CmdInsertNext, commands.InsertNextPayload{
			Item: toCommandItem(req.Item),
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}

		size := qMgr.Size()
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
			QueueSize: &size,
		})
	}
}

// InsertAfter returns a handler for POST /v1/queue/insert-after.
func InsertAfter(bus queueBus, qMgr queueReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req insertAfterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_payload", "request body must be valid JSON")
			return
		}
		if req.AfterQueueItemID == "" {
			writeError(w, http.StatusBadRequest, "invalid_payload", "field after_queue_item_id is required")
			return
		}
		if reason := validateItem(req.Item); reason != "" {
			writeError(w, http.StatusBadRequest, "invalid_payload", reason)
			return
		}

		cmd, replyCh := commands.NewSync(commands.CmdInsertAfter, commands.InsertAfterPayload{
			AfterQueueItemID: req.AfterQueueItemID,
			Item:             toCommandItem(req.Item),
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}

		size := qMgr.Size()
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
			QueueSize: &size,
		})
	}
}

// ClearQueue returns a handler for POST /v1/queue/clear.
func RemoveItem(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			QueueItemID string `json:"queue_item_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.QueueItemID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "missing queue_item_id")
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdRemoveItem, commands.RemoveItemPayload{
			QueueItemID: req.QueueItemID,
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}

func MoveItem(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			QueueItemID string `json:"queue_item_id"`
			Direction   string `json:"direction"` // "up" | "down"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.QueueItemID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "missing queue_item_id")
			return
		}
		if req.Direction != "up" && req.Direction != "down" {
			writeError(w, http.StatusBadRequest, "bad_request", `direction must be "up" or "down"`)
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdMoveItem, commands.MoveItemPayload{
			QueueItemID: req.QueueItemID,
			Direction:   req.Direction,
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}

func ReorderItem(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			QueueItemID string `json:"queue_item_id"`
			BreakID     string `json:"break_id"`
			AfterID     string `json:"after_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
			return
		}
		if req.QueueItemID == "" && req.BreakID == "" {
			writeError(w, http.StatusBadRequest, "bad_request", "queue_item_id or break_id is required")
			return
		}
		cmd, replyCh := commands.NewSync(commands.CmdReorderItem, commands.ReorderItemPayload{
			QueueItemID: req.QueueItemID,
			BreakID:     req.BreakID,
			AfterID:     req.AfterID,
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}
		writeJSON(w, http.StatusOK, cmdResponse{
			OK: true, CommandID: cmd.ID, Accepted: result.Accepted, Reason: result.Reason,
		})
	}
}

func ClearQueue(bus queueBus) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req clearQueueRequest
		if r.ContentLength != 0 {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		preserveCurrent := true // default: keep the currently playing item
		if req.PreserveCurrent != nil {
			preserveCurrent = *req.PreserveCurrent
		}

		cmd, replyCh := commands.NewSync(commands.CmdClearQueue, commands.ClearQueuePayload{
			PreserveCurrent: preserveCurrent,
		})
		result, ok := sendAndWait(w, bus, cmd, replyCh)
		if !ok {
			return
		}

		writeJSON(w, http.StatusOK, cmdResponse{
			OK:        true,
			CommandID: cmd.ID,
			Accepted:  result.Accepted,
			Reason:    result.Reason,
		})
	}
}
