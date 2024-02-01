package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/livefir/fir"
	"github.com/livefir/fir/pubsub"
)

type ObsSt struct {
	FirmwareRevision int64       `json:"firmware_revision"`
	HubSn            string      `json:"hub_sn"`
	Obs              [][]float64 `json:"obs"`
	SerialNumber     string      `json:"serial_number"`
	Type             string      `json:"type"`
}

type App struct {
	sync.RWMutex
}

type index struct {
	model       *App
	pubsub      pubsub.Adapter
	eventSender chan fir.Event
	id          string
}

var conn *net.UDPConn
var lastReading ObsSt

func Format(n int64) string {
	in := strconv.FormatInt(n, 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
}

func DegToCompass(deg float64) string {
	var directions = []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW", "N"}
	ix := int((deg + 22.5) / 45)
	if ix < 0 {
		ix = 0
	} else if ix >= len(directions) {
		ix = len(directions) - 1
	}
	return directions[ix]
}

func NewWxIndex(pubsub pubsub.Adapter) *index {
	c := &index{
		model:       &App{},
		pubsub:      pubsub,
		eventSender: make(chan fir.Event),
		id:          "wx-app",
	}

	go func() {
		buf := make([]byte, 1024)
		var obsInstance ObsSt

		for {
			n, _, _ := conn.ReadFromUDP(buf)

			data := buf[:n]
			msg := make(map[string]interface{})
			json.Unmarshal(data, &msg)

			if ttype, ok := msg["type"]; ok {
				if ttype == "obs_st" {

					json.Unmarshal(data, &obsInstance)
					lastReading = obsInstance

					c.eventSender <- fir.NewEvent("updated", obsInstance)

				}
			}
		}
	}()

	return c
}

func (i *index) Options() fir.RouteOptions {
	return fir.RouteOptions{
		fir.ID(i.id),
		fir.Content("readings.html"),
		fir.Layout("layout.html"),
		fir.OnLoad(i.load),
		fir.OnEvent("updated", i.updated),
		fir.EventSender(i.eventSender),
	}
}

func prefs() fir.RouteOptions {
	return fir.RouteOptions{
		fir.ID("prefs"),
		fir.Content("prefs.html"),
		fir.OnLoad(func(ctx fir.RouteContext) error {
			return ctx.Data(map[string]any{})
		}),
	}
}

func formatReading(reading ObsSt) map[string]any {
	if reading.FirmwareRevision == 0 {
		return map[string]any{
			"hub":   "⌛️",
			"batt":  "⌛️",
			"temp":  "⌛️",
			"humid": "⌛️",
			"lumos": "⌛️",
			"press": "⌛️",
			"insol": "⌛️",
			"ultra": "⌛️",
			"wind":  "⌛️",
			"wdir":  "⌛️",
			"when":  "⌛️",
		}
	} else {
		return map[string]any{
			"hub":   reading.HubSn,
			"batt":  fmt.Sprintf("%.1f volts", reading.Obs[0][16]),
			"temp":  fmt.Sprintf("%.1f", reading.Obs[0][7]),
			"humid": fmt.Sprintf("%.1f%%", lastReading.Obs[0][8]),
			"lumos": Format(int64(reading.Obs[0][9])),
			"press": strconv.FormatInt(int64(reading.Obs[0][6]), 10),
			"insol": Format(int64(reading.Obs[0][11])),
			"ultra": Format(int64(reading.Obs[0][10])),
			"wind":  fmt.Sprintf("%.1f", reading.Obs[0][2]),
			"wdir":  DegToCompass(reading.Obs[0][4]),
			"when":  time.Now().Format("2006-01-02 15:04:05"),
		}
	}
}

// load is called when the page is loaded.
func (i *index) load(ctx fir.RouteContext) error {
	return ctx.Data(formatReading(lastReading))
}

// updated is called when the "updated" event is received.
func (i *index) updated(ctx fir.RouteContext) error {
	reading := &ObsSt{}

	err := ctx.Bind(reading)
	if err != nil {
		return err
	}

	return ctx.Data(formatReading(*reading))
}

// initUDPListener creates a UDP listener on port 50222.
func initUDPListener() {
	addr, _ := net.ResolveUDPAddr("udp", ":50222")
	conn, _ = net.ListenUDP("udp", addr)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		conn.Close()
		os.Exit(0)
	}()
}

func main() {

	initUDPListener()
	defer conn.Close()

	pubsubAdapter := pubsub.NewInmem()

	controller := fir.NewController(
		"wx-app",
		fir.DevelopmentMode(true),
		fir.WithPubsubAdapter(pubsubAdapter),
	)

	http.Handle("/", controller.Route(NewWxIndex(pubsubAdapter)))

	http.Handle("/prefs", controller.RouteFunc(prefs))

	if os.Getenv("SEEKRIT_TOKEN") == "" {
		http.HandleFunc("/quit", func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			secretToken := os.Getenv("SEEKRIT_TOKEN")

			if token == secretToken {
				conn.Close()
				os.Exit(0)
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		})
	}

	http.ListenAndServe("0.0.0.0:9867", nil)
}
