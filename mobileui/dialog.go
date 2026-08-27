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

func dialogMessageSpeakerCount(messages []DialogMessage) int {
	count := 0
	previous := ""
	for _, message := range messages {
		speaker := strings.TrimSpace(message.Speaker)
		if speaker != "" && speaker != previous {
			count++
		}
		previous = speaker
	}
	return count
}

func dialogMessageContentHeight(messages []DialogMessage, maxChars int, lineAdvance float32) float32 {
	height := float32(0)
	previousSpeaker := ""
	for i, message := range messages {
		if speaker := strings.TrimSpace(message.Speaker); speaker != "" && speaker != previousSpeaker {
			height += 28
			if height > 0 {
				height += 4
			}
		}
		if i > 0 {
			height += 8
		}
		height += float32(dialogLineCount(message.Text, maxChars)) * lineAdvance
		previousSpeaker = strings.TrimSpace(message.Speaker)
	}
	return maxf(72, height)
}

func LayoutDialog(viewport Viewport, model MobileDialogModel) DialogLayout {
	safe := viewport.SafeRect()
	layout := DialogLayout{Safe: safe}
	if safe.W <= 0 || safe.H <= 0 || !model.Open {
		return layout
	}
	layout.Portrait = viewport.IsPortrait()
	panelW := minf(1100, maxf(0, safe.W-48))
	if layout.Portrait {
		panelW = maxf(0, safe.W-32)
	}
	pad := float32(20)
	if layout.Portrait {
		pad = 16
	}
	messageW := maxf(0, panelW-2*pad)
	actionCount := len(model.Options)
	buttonH := float32(56)
	actionGap := float32(12)
	actionH := float32(0)
	if actionCount > 0 {
		if layout.Portrait {
			actionH = float32(actionCount)*buttonH + float32(actionCount-1)*actionGap
		} else {
			columns := maxInt(1, int((messageW+actionGap)/(136+actionGap)))
			columns = minInt(columns, actionCount)
			rows := (actionCount + columns - 1) / columns
			actionH = float32(rows)*buttonH + float32(rows-1)*actionGap
		}
	}
	noticeH := float32(0)
	if strings.TrimSpace(model.Notice) != "" {
		noticeH = 28
	}
	messageScale := dialogTextScale(viewport) * 1.08
	lineAdvance := maxf(24, float32(27)*dialogTextScale(viewport))
	messageMaxChars := maxInt(28, int(messageW/(11*messageScale)))
	messages := dialogMessagesForLayout(model)
	messageH := dialogMessageContentHeight(messages, messageMaxChars, lineAdvance)
	contentH := float32(16+56+12) + messageH
	if noticeH > 0 {
		contentH += 8 + noticeH
	}
	if actionH > 0 {
		contentH += 12 + actionH
	}
	contentH += 16
	minimumH := float32(240)
	if layout.Portrait {
		minimumH = 300
	}
	panelH := minf(maxf(0, safe.H-32), maxf(minimumH, contentH))
	layout.Panel = Rect{X: safe.X + (safe.W-panelW)/2, Y: safe.Y + (safe.H-panelH)/2, W: panelW, H: panelH}
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
		previousSpeaker := ""
		for i, message := range messages {
			speaker := strings.TrimSpace(message.Speaker)
			if speaker != "" && speaker != previousSpeaker {
				speakerRect := Rect{X: layout.Message.X, Y: cursorY, W: layout.Message.W, H: 28}
				layout.MessageBlocks = append(layout.MessageBlocks, DialogMessageLayout{Speaker: speaker, Text: message.Text, SpeakerRect: speakerRect})
				cursorY = speakerRect.Bottom() + 4
			} else {
				if i > 0 {
					cursorY += 8
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
			columns := maxInt(1, int((layout.Actions.W+actionGap)/(136+actionGap)))
			columns = minInt(columns, actionCount)
			buttonW := minf(190, maxf(136, (layout.Actions.W-float32(columns-1)*actionGap)/float32(columns)))
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
		return 2.50
	}
	scale := shortEdge / 320
	return minf(2.75, maxf(2.50, scale))
}

func normalizeDialogText(value string) string {
	return NormalizeROText(value)
}

type DialogController struct {
	Model    MobileDialogModel
	Layout   DialogLayout
	Viewport Viewport
	Sink     input.CommandSink
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
		if !option.Enabled {
			return true
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
		return true
	}
	return c.Layout.Safe.Contains(x, y)
}

func (c *DialogController) Back() bool {
	return c.Close()
}

func (c *DialogController) relayout() {
	if c == nil {
		return
	}
	c.Layout = LayoutDialog(c.Viewport, c.Model)
}
