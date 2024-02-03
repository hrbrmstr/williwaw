package main

import (
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/livefir/fir"
	"github.com/livefir/fir/pubsub"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/exp/slog"
)

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
var lastHubStatus HubStatus
var db *sql.DB

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
		var hubStatusInstance HubStatus

		for {
			n, _, _ := conn.ReadFromUDP(buf)

			data := buf[:n]
			msg := make(map[string]interface{})
			json.Unmarshal(data, &msg)

			if ttype, ok := msg["type"]; ok {
				if ttype == "hub_status" {

					json.Unmarshal(data, &hubStatusInstance)
					lastHubStatus = hubStatusInstance
					c.eventSender <- fir.NewEvent("hub", hubStatusInstance)

				} else if ttype == "obs_st" {
					if os.Getenv("DB_PATH") != "" {
						logReading("obs_st", data)
					}

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
		fir.OnEvent("hub", i.hub),
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

// load is called when the page is loaded.
func (i *index) load(ctx fir.RouteContext) error {
	reading := formatReading(lastReading)
	hubStatus := formatHubStatus(lastHubStatus)
	for key, value := range hubStatus {
		reading[key] = value
	}
	return ctx.Data(reading)
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

// hub is called when the "hub" event is received.
func (i *index) hub(ctx fir.RouteContext) error {
	hubStatus := &HubStatus{}

	err := ctx.Bind(hubStatus)
	if err != nil {
		return err
	}

	return ctx.Data(formatHubStatus(*hubStatus))
}

// initUDPListener creates a UDP listener on port 50222.
func initUDPListener() {
	addr, _ := net.ResolveUDPAddr("udp", ":50222")
	conn, _ = net.ListenUDP("udp", addr)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		slog.Info("Shutting down...")
		conn.Close()
		if os.Getenv("DB_PATH") != "" {
			db.Close()
		}
		os.Exit(0)
	}()
}

func main() {

	initUDPListener()
	defer conn.Close()

	if os.Getenv("DB_PATH") != "" {
		db = initDB(os.Getenv("DB_PATH"))
		defer db.Close()
	}

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
				slog.Info("Shutting down...")
				conn.Close()
				if os.Getenv("DB_PATH") != "" {
					db.Close()
				}
				os.Exit(0)
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		})
	}

	http.ListenAndServe("0.0.0.0:9867", nil)
}
