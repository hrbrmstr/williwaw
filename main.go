package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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

func NewWxIndex(pubsub pubsub.Adapter) *index {
	c := &index{
		model:       &App{},
		pubsub:      pubsub,
		eventSender: make(chan fir.Event),
		id:          "wx-app",
	}

	go func() {

		addr, _ := net.ResolveUDPAddr("udp", ":50222")
		conn, _ := net.ListenUDP("udp", addr)
		defer conn.Close()

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

					c.eventSender <- fir.NewEvent("updated", obsInstance)

				}
			}
		}
	}()

	return c
}

type index struct {
	model       *App
	pubsub      pubsub.Adapter
	eventSender chan fir.Event
	id          string
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

func (i *index) load(ctx fir.RouteContext) error {
	fmt.Println(ctx)
	return ctx.Data(map[string]any{
		"hub":   "⌛️",
		"batt":  "⌛️",
		"temp":  "⌛️",
		"humid": "⌛️",
		"lumos": "⌛️",
		"press": "⌛️",
		"when":  "⌛️",
	})
}

func (i *index) updated(ctx fir.RouteContext) error {
	reading := &ObsSt{}

	err := ctx.Bind(reading)
	if err != nil {
		return err
	}

	return ctx.Data(map[string]any{
		"hub":   reading.HubSn,
		"batt":  fmt.Sprintf("%.1f volts", reading.Obs[0][16]),
		"temp":  fmt.Sprintf("%.1f°F", reading.Obs[0][7]*1.8+32),
		"humid": fmt.Sprintf("%.1f%%", reading.Obs[0][8]),
		"lumos": Format(int64(reading.Obs[0][9])),
		"press": Format(int64(reading.Obs[0][6])) + "mb",
		"when":  time.Now().Format("2006-01-02 15:04:05"),
	})
}

func main() {
	pubsubAdapter := pubsub.NewInmem()
	controller := fir.NewController("wx-app", fir.DevelopmentMode(false), fir.WithPubsubAdapter(pubsubAdapter))
	http.Handle("/", controller.Route(NewWxIndex(pubsubAdapter)))
	http.ListenAndServe("0.0.0.0:9867", nil)
}
