package color

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"
	"github.com/nats-io/nats.go"
)

const (
	DefaultNATSUrl = "nats://nonlocal.info:4222"
	DefaultSubject = "color.index"
)

// ColorIndex is the message format from bci-hypergraph
type ColorIndex struct {
	Timestamp int64      `json:"ts"`
	Epoch     int        `json:"epoch"`
	Seed      uint64     `json:"seed"`
	Index     int        `json:"index"`
	Hue       float64    `json:"hue"`
	RGB       [3]uint8   `json:"rgb"`
	Hex       string     `json:"hex"`
	Trit      int        `json:"trit"`
	TritSum   int        `json:"trit_sum"`
	GF3OK     bool       `json:"gf3"`
	D4        string     `json:"d4"`
	Channels  [8]float64 `json:"alpha"`
	Band      string     `json:"band"`
	Edges     int        `json:"edges"`
}

// Subscribe connects to NATS and renders color.index messages in the terminal
func Subscribe(natsUrl, subject string) error {
	if natsUrl == "" {
		natsUrl = DefaultNATSUrl
	}
	if subject == "" {
		subject = DefaultSubject
	}

	p := termenv.ColorProfile()

	nc, err := nats.Connect(natsUrl,
		nats.Name("boxxy-color"),
		nats.ReconnectWait(2*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return fmt.Errorf("nats connect %s: %w", natsUrl, err)
	}
	defer nc.Drain()

	fmt.Printf("connected to %s\n", natsUrl)
	fmt.Printf("subscribing to %s\n\n", subject)

	// Trit symbols
	tritGlyph := map[int]string{
		-1: "−",
		0:  "○",
		1:  "+",
	}

	count := 0

	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		var ci ColorIndex
		if err := json.Unmarshal(msg.Data, &ci); err != nil {
			return
		}

		count++

		// Render the color block
		c, _ := colorful.Hex(ci.Hex)
		h, s, l := c.Hsl()

		// Background block with the color
		bg := termenv.RGBColor(ci.Hex)
		// Foreground: white or black depending on luminance
		fg := termenv.RGBColor("#000000")
		if l < 0.5 {
			fg = termenv.RGBColor("#FFFFFF")
		}

		// GF(3) conservation indicator
		gf3 := "✓"
		if !ci.GF3OK {
			gf3 = "✗"
		}

		trit := tritGlyph[ci.Trit]

		// Alpha band sparkline (8 channels)
		spark := ""
		sparkChars := []rune(" ▁▂▃▄▅▆▇")
		for _, a := range ci.Channels {
			norm := math.Min(a/3.0, 1.0)
			level := int(norm * 8)
			if level > 8 {
				level = 8
			}
			spark += string(sparkChars[level])
		}

		// Color swatch (4 chars wide)
		swatch := termenv.String("    ").Background(bg).Foreground(fg)

		// Build output line
		line := fmt.Sprintf(
			"%s %s %s  epoch:%-4d  trit:%s  gf3:%s  hue:%-6.1f  %s  %s  α:[%s]",
			swatch,
			termenv.String(ci.Hex).Foreground(p.Color(ci.Hex)),
			termenv.String(ci.D4).Foreground(p.Color("#A855F7")),
			ci.Epoch,
			trit,
			gf3,
			h,
			termenv.String(fmt.Sprintf("s:%.0f%% l:%.0f%%", s*100, l*100)).Foreground(p.Color("#888888")),
			termenv.String(fmt.Sprintf("e:%d", ci.Edges)).Foreground(p.Color("#888888")),
			termenv.String(spark).Foreground(p.Color(ci.Hex)),
		)

		fmt.Println(line)
	})
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", subject, err)
	}

	// Wait for Ctrl-C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Printf("\n%d colors received\n", count)
	return nil
}

// Publish sends a single color to NATS (for testing)
func Publish(natsUrl, subject, hexColor string, epoch int, trit int) error {
	if natsUrl == "" {
		natsUrl = DefaultNATSUrl
	}
	if subject == "" {
		subject = DefaultSubject
	}

	nc, err := nats.Connect(natsUrl, nats.Name("boxxy-color-pub"))
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer nc.Drain()

	c, _ := colorful.Hex(hexColor)
	h, _, _ := c.Hsl()
	r, g, b := c.RGB255()

	ci := ColorIndex{
		Timestamp: time.Now().UnixMilli(),
		Epoch:     epoch,
		Seed:      1069,
		Index:     epoch,
		Hue:       h,
		RGB:       [3]uint8{r, g, b},
		Hex:       hexColor,
		Trit:      trit,
		TritSum:   0,
		GF3OK:     true,
		D4:        "boxxy",
		Edges:     0,
	}

	data, _ := json.Marshal(ci)
	return nc.Publish(subject, data)
}
