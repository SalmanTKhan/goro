package mobileui

import (
	"strings"

	"github.com/kivutar/goro/input"
)

type DialogAction uint8

const (
	DialogClose DialogAction = iota
	DialogOpenShop
	DialogNext
	DialogMenuChoice
	DialogOpenStorage
	DialogNPCClose
)

type DialogOption struct {
	ID      string
	Label   string
	Action  DialogAction
	Enabled bool
	Value   int
}

// DialogMessage is one speaker turn in an NPC dialog. RO servers commonly
// prefix dialog text with a bracketed script name, for example [Tine]. Keep
// that identity separate from the body so the mobile renderer can place it in
// a header or a per-turn label without showing the prefix as body text.
type DialogMessage struct {
	Speaker string
	Text    string
}

type MobileDialogModel struct {
	Open     bool
	NPCID    uint32
	Title    string
	Message  string
	Messages []DialogMessage
	Notice   string
	Options  []DialogOption
}

func ProjectDialog(npcID uint32, title, message string, shopAvailable bool) MobileDialogModel {
	model := MobileDialogModel{Open: true, NPCID: npcID, Title: title, Message: message}
	if title == "" {
		model.Title = "DIALOG"
	}
	model.Options = append(model.Options, DialogOption{ID: "close", Label: "Close", Action: DialogClose, Enabled: true})
	if shopAvailable {
		model.Options = append(model.Options, DialogOption{ID: "shop", Label: "Open Shop", Action: DialogOpenShop, Enabled: true})
	}
	return model
}

type DialogLayout struct {
	Safe, Panel, Header, Message, Notice, Actions, Close Rect
	Options                                              []Rect
	MessageBlocks                                        []DialogMessageLayout
	MessageContentHeight                                 float32
	Portrait                                             bool
}

// DialogMessageLayout maps a speaker turn to the exact rectangles used by the
// renderer. SpeakerRect can be empty when a consecutive turn has the same
// speaker as the previous one.
type DialogMessageLayout struct {
	Speaker     string
	Text        string
	SpeakerRect Rect
	TextRect    Rect
}

// ParseDialogSpeaker extracts a leading RO-style [Speaker] prefix. Only a
// prefix at the beginning of the message is considered an identity marker;
// bracketed text in the body remains ordinary dialog content.
func ParseDialogSpeaker(value string) (speaker, body string, ok bool) {
	trimmed := strings.TrimLeft(value, " \t")
	if !strings.HasPrefix(trimmed, "[") {
		return "", value, false
	}
	end := strings.IndexByte(trimmed, ']')
	if end <= 1 {
		return "", value, false
	}
	speaker = strings.TrimSpace(trimmed[1:end])
	if speaker == "" {
		return "", value, false
	}
	body = strings.TrimLeft(trimmed[end+1:], " \t")
	return speaker, body, true
}

func parseDialogPacket(raw string) []DialogMessage {
	segments := strings.Split(NormalizeROText(raw), "\n")
	var messages []DialogMessage
	var current DialogMessage
	haveCurrent := false
	flush := func() {
		if !haveCurrent {
			return
		}
		current.Text = strings.TrimRight(current.Text, "\n")
		if current.Text != "" || current.Speaker != "" {
			messages = append(messages, current)
		}
		current = DialogMessage{}
		haveCurrent = false
	}
	for _, segment := range segments {
		if speaker, body, ok := ParseDialogSpeaker(segment); ok {
			flush()
			current = DialogMessage{Speaker: speaker, Text: body}
			haveCurrent = true
			continue
		}
		if !haveCurrent {
			if strings.TrimSpace(segment) == "" {
				continue
			}
			current = DialogMessage{Text: segment}
			haveCurrent = true
			continue
		}
		if current.Text != "" {
			current.Text += "\n"
		}
		current.Text += segment
	}
	flush()
	return messages
}

// ParseDialogMessages preserves packet order while splitting speaker markers
// that occur at the beginning of a line or an RO <br> paragraph. This covers
// both the usual one-speaker-per-packet form and scripts that emit several
// bracketed names in one packet. Some servers send the marker as its own
// packet, so a marker-only message is paired with the next prose message.
func ParseDialogMessages(lines []string) []DialogMessage {
	var messages []DialogMessage
	pendingSpeaker := ""
	for _, raw := range lines {
		for _, message := range parseDialogPacket(raw) {
			if message.Speaker != "" && strings.TrimSpace(message.Text) == "" {
				pendingSpeaker = message.Speaker
				continue
			}
			if pendingSpeaker != "" {
				if message.Speaker == "" {
					message.Speaker = pendingSpeaker
				}
				pendingSpeaker = ""
			}
			messages = append(messages, message)
		}
	}
	if pendingSpeaker != "" {
		messages = append(messages, DialogMessage{Speaker: pendingSpeaker})
	}
	return messages
}

func dialogMessagesForLayout(model MobileDialogModel) []DialogMessage {
	if len(model.Messages) > 0 {
		return model.Messages
	}
	if model.Message == "" {
		return []DialogMessage{{Text: ""}}
	}
	if speaker, body, ok := ParseDialogSpeaker(model.Message); ok {
		return []DialogMessage{{Speaker: speaker, Text: body}}
	}
	return []DialogMessage{{Text: model.Message}}
}

func dialogMessageContentHeight(messages []DialogMessage, maxChars int, lineAdvance float32, initialSpeaker string) float32 {
	height := float32(0)
	previousSpeaker := strings.TrimSpace(initialSpeaker)
	for i, message := range messages {
		speaker := strings.TrimSpace(message.Speaker)
		if speaker != "" && speaker != previousSpeaker {
			height += 24
			if height > 0 {
				height += 3
			}
		}
		if i > 0 {
			height += 3
		}
		height += float32(dialogLineCount(message.Text, maxChars)) * lineAdvance
		if speaker != "" {
			previousSpeaker = speaker
		}
	}
	return maxf(64, height)
}

func LayoutDialog(viewport Viewport, model MobileDialogModel) DialogLayout {
	safe := viewport.SafeRect()
	layout := DialogLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 || !model.Open {
		return layout
	}
	layout.Portrait = viewport.IsPortrait()
	panelW := minf(680, maxf(0, safe.W*0.62))
	if layout.Portrait {
		panelW = maxf(0, safe.W-20)
	}
	pad := float32(20)
	if layout.Portrait {
		pad = 16
	}
	messageW := maxf(0, panelW-2*pad)
	actionCount := len(model.Options)
	buttonH := float32(52)
	if actionCount > 1 {
		// Script menu labels are content, not authored chrome. Give them enough
		// vertical room to wrap to two lines instead of ellipsizing the choice.
		buttonH = 64
	}
	actionGap := float32(10)
	actionH := float32(0)
	actionColumns := 1
	if actionCount > 0 {
		if layout.Portrait {
			actionH = float32(actionCount)*buttonH + float32(actionCount-1)*actionGap
		} else {
			actionColumns = dialogActionColumns(messageW, actionCount, actionGap)
			rows := (actionCount + actionColumns - 1) / actionColumns
			actionH = float32(rows)*buttonH + float32(rows-1)*actionGap
		}
	}
	noticeH := float32(0)
	if strings.TrimSpace(model.Notice) != "" {
		noticeH = 28
	}
	messageScale := dialogTextScale(viewport)
	// The retained mobile theme renders body text at 28 logical pixels with
	// ~1.3 line spacing. Use a deliberately conservative estimate here so the
	// scroll range never ends before the real measured text does.
	lineAdvance := maxf(36, float32(38)*messageScale)
	messageMaxChars := maxInt(18, int(messageW/(15*messageScale)))
	messages := dialogMessagesForLayout(model)
	messageH := dialogMessageContentHeight(messages, messageMaxChars, lineAdvance, strings.TrimSpace(model.Title))
	layout.MessageContentHeight = messageH
	contentH := float32(16+56+12) + messageH
	if noticeH > 0 {
		contentH += 8 + noticeH
	}
	if actionH > 0 {
		contentH += 12 + actionH
	}
	contentH += 16
	minimumH := float32(156)
	if layout.Portrait {
		minimumH = 190
	}
	maxH := maxf(0, safe.H-24)
	if layout.Portrait {
		// Portrait also needs room for the NPC cut-in. Keep the conversation as
		// a bottom sheet and scroll its body instead of allowing it to grow over
		// the character illustration.
		maxH = minf(maxH, safe.H*0.52)
	} else {
		maxH = minf(maxH, safe.H*0.72)
	}
	panelH := minf(maxH, maxf(minimumH, contentH))
	panelY := safe.Bottom() - panelH - 16
	if layout.Portrait {
		panelY = safe.Bottom() - panelH - 12
	}
	layout.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: panelY, W: panelW, H: panelH}
	layout.Header = Rect{X: layout.Panel.X + pad, Y: layout.Panel.Y + 16, W: layout.Panel.W - 2*pad, H: 56}
	actionBottom := layout.Panel.Bottom() - 16
	if actionH > 0 {
		layout.Actions = Rect{X: layout.Panel.X + pad, Y: actionBottom - actionH, W: layout.Panel.W - 2*pad, H: actionH}
	}
	contentBottom := actionBottom
	if actionH > 0 {
		contentBottom = layout.Actions.Y - 12
	}
	if noticeH > 0 {
		layout.Notice = Rect{X: layout.Panel.X + pad, Y: contentBottom - noticeH, W: layout.Panel.W - 2*pad, H: noticeH}
		contentBottom = layout.Notice.Y - 8
	}
	layout.Message = Rect{X: layout.Panel.X + pad, Y: layout.Header.Bottom() + 12, W: layout.Panel.W - 2*pad, H: maxf(0, contentBottom-(layout.Header.Bottom()+12))}
	if len(model.Messages) > 0 {
		cursorY := layout.Message.Y
		previousSpeaker := strings.TrimSpace(model.Title)
		for i, message := range messages {
			speaker := strings.TrimSpace(message.Speaker)
			if speaker != "" && speaker != previousSpeaker {
				speakerRect := Rect{X: layout.Message.X, Y: cursorY, W: layout.Message.W, H: 24}
				layout.MessageBlocks = append(layout.MessageBlocks, DialogMessageLayout{Speaker: speaker, Text: message.Text, SpeakerRect: speakerRect})
				cursorY = speakerRect.Bottom() + 4
			} else {
				if i > 0 {
					cursorY += 3
				}
				layout.MessageBlocks = append(layout.MessageBlocks, DialogMessageLayout{Speaker: speaker, Text: message.Text})
			}
			block := &layout.MessageBlocks[len(layout.MessageBlocks)-1]
			block.TextRect = Rect{X: layout.Message.X, Y: cursorY, W: layout.Message.W, H: maxf(24, float32(dialogLineCount(message.Text, messageMaxChars))*lineAdvance)}
			cursorY = block.TextRect.Bottom()
			previousSpeaker = speaker
		}
	}
	if actionH > 0 {
		if layout.Portrait {
			buttonW := maxf(0, layout.Actions.W)
			for i, option := range model.Options {
				y := layout.Actions.Y + float32(i)*(buttonH+actionGap)
				layout.Options = append(layout.Options, Rect{X: layout.Actions.X, Y: y, W: buttonW, H: buttonH})
				if option.Action == DialogClose || option.Action == DialogNPCClose {
					layout.Close = layout.Options[len(layout.Options)-1]
				}
			}
		} else {
			columns := actionColumns
			buttonW := minf(300, maxf(136, (layout.Actions.W-float32(columns-1)*actionGap)/float32(columns)))
			if actionCount == 1 {
				// A single Next/Close action is the primary conversation
				// affordance on touch screens; make it deliberately generous.
				buttonW = minf(320, layout.Actions.W)
			}
			for i, option := range model.Options {
				row, column := i/columns, i%columns
				rowStart := layout.Actions.X + (layout.Actions.W-(float32(columns)*buttonW+float32(columns-1)*actionGap))/2
				x := rowStart + float32(column)*(buttonW+actionGap)
				y := layout.Actions.Y + float32(row)*(buttonH+actionGap)
				layout.Options = append(layout.Options, Rect{X: x, Y: y, W: buttonW, H: buttonH})
				if option.Action == DialogClose || option.Action == DialogNPCClose {
					layout.Close = layout.Options[len(layout.Options)-1]
				}
			}
		}
	}
	return layout
}

func dialogActionColumns(width float32, actionCount int, gap float32) int {
	if actionCount <= 1 {
		return 1
	}
	columns := maxInt(1, int((width+gap)/(220+gap)))
	columns = minInt(columns, actionCount)
	// More than two menu choices in a single row makes script labels unreadable
	// on phones even when the framebuffer itself is wide.
	if actionCount >= 3 {
		columns = minInt(columns, 2)
	}
	return columns
}

func dialogLineCount(message string, maxChars int) int {
	if maxChars < 1 {
		maxChars = 1
	}
	message = StripROText(message)
	lines := 0
	for _, paragraph := range strings.Split(message, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines++
			continue
		}
		lineLength := 0
		for _, word := range words {
			wordLength := len([]rune(word))
			if lineLength == 0 {
				lineLength = wordLength
				lines++
				continue
			}
			if lineLength+1+wordLength > maxChars {
				lineLength = wordLength
				lines++
				continue
			}
			lineLength += 1 + wordLength
		}
	}
	if lines < 1 {
		return 1
	}
	return lines
}

func dialogTextScale(viewport Viewport) float32 {
	safe := viewport.SafeRect()
	shortEdge := safe.H
	if safe.W > 0 && (shortEdge <= 0 || safe.W < shortEdge) {
		shortEdge = safe.W
	}
	if shortEdge <= 0 {
		return 1
	}
	// Text widgets use the mobile theme's real font size already. This value is
	// only a layout estimate; the previous 2.5x minimum produced enormous blank
	// vertical gaps and full-screen dialog sheets.
	scale := shortEdge / 720
	return minf(1.35, maxf(0.95, scale))
}

func normalizeDialogText(value string) string {
	return NormalizeROText(value)
}

type DialogController struct {
	Model        MobileDialogModel
	Layout       DialogLayout
	Viewport     Viewport
	Sink         input.CommandSink
	ScrollOffset float32
}

func NewDialogController(model MobileDialogModel, viewport Viewport, sink input.CommandSink) *DialogController {
	c := &DialogController{Model: model, Viewport: viewport, Sink: sink}
	c.relayout()
	return c
}

func (c *DialogController) SetModel(model MobileDialogModel) {
	if c == nil {
		return
	}
	if !sameDialogContent(c.Model, model) {
		c.ScrollOffset = 0
	}
	c.Model = model
	c.relayout()
}

func (c *DialogController) Open(model MobileDialogModel) {
	if c == nil {
		return
	}
	model.Open = true
	c.SetModel(model)
}

func (c *DialogController) Close() bool {
	if c == nil || !c.Model.Open {
		return false
	}
	c.Model.Open = false
	c.relayout()
	return true
}

func (c *DialogController) Resize(viewport Viewport) {
	if c == nil {
		return
	}
	c.Viewport = viewport
	c.relayout()
}

func (c *DialogController) ConsumeTouch(point input.TouchPoint) bool {
	_ = point
	return c != nil && c.Model.Open
}

func (c *DialogController) Tap(x, y float32) bool {
	if c == nil || !c.Model.Open {
		return false
	}
	for i, option := range c.Model.Options {
		if i >= len(c.Layout.Options) || !c.Layout.Options[i].Contains(x, y) {
			continue
		}
		c.activate(option)
		return true
	}
	// Next/Close dialogs have one obvious action. Treat the complete action
	// strip as its touch target so small raster/layout differences cannot make
	// the visibly large mobile button inert.
	if len(c.Model.Options) == 1 && c.Layout.Actions.Contains(x, y) {
		c.activate(c.Model.Options[0])
		return true
	}
	return c.Layout.Safe.Contains(x, y)
}

func (c *DialogController) activate(option DialogOption) {
	if c == nil || !option.Enabled {
		return
	}
	switch option.Action {
	case DialogOpenShop:
		if c.Sink != nil {
			c.Sink.Emit(input.PlayerCommand{Kind: input.CommandOpenShop, NPCID: c.Model.NPCID})
		}
		c.Close()
	case DialogNext:
		if c.Sink != nil {
			c.Sink.Emit(input.PlayerCommand{Kind: input.CommandNPCNext, NPCID: c.Model.NPCID})
		}
	case DialogMenuChoice:
		if c.Sink != nil {
			c.Sink.Emit(input.PlayerCommand{Kind: input.CommandNPCMenuChoice, NPCID: c.Model.NPCID, Choice: uint8(option.Value)})
		}
	case DialogOpenStorage:
		if c.Sink != nil {
			c.Sink.Emit(input.PlayerCommand{Kind: input.CommandOpenStorage, NPCID: c.Model.NPCID})
		}
		c.Close()
	case DialogNPCClose:
		if c.Sink != nil {
			c.Sink.Emit(input.PlayerCommand{Kind: input.CommandNPCClose, NPCID: c.Model.NPCID})
		}
		c.Close()
	default:
		c.Close()
	}
}

func (c *DialogController) Back() bool {
	return c.Close()
}

func (c *DialogController) ScrollBy(delta float32) bool {
	if c == nil || !c.Model.Open || c.Layout.Message.H <= 0 {
		return false
	}
	maxOffset := maxf(0, c.Layout.MessageContentHeight-c.Layout.Message.H)
	next := c.ScrollOffset + delta
	if next < 0 {
		next = 0
	}
	if next > maxOffset {
		next = maxOffset
	}
	if next == c.ScrollOffset {
		return false
	}
	c.ScrollOffset = next
	return true
}

func sameDialogContent(a, b MobileDialogModel) bool {
	if a.Open != b.Open || a.NPCID != b.NPCID || a.Title != b.Title || a.Message != b.Message || a.Notice != b.Notice ||
		len(a.Messages) != len(b.Messages) || len(a.Options) != len(b.Options) {
		return false
	}
	for i := range a.Messages {
		if a.Messages[i] != b.Messages[i] {
			return false
		}
	}
	for i := range a.Options {
		if a.Options[i] != b.Options[i] {
			return false
		}
	}
	return true
}

func (c *DialogController) relayout() {
	if c == nil {
		return
	}
	c.Layout = LayoutDialog(c.Viewport, c.Model)
	maxOffset := maxf(0, c.Layout.MessageContentHeight-c.Layout.Message.H)
	if c.ScrollOffset > maxOffset {
		c.ScrollOffset = maxOffset
	}
}
