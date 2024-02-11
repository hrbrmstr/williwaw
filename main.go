package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/livefir/fir"
	"github.com/livefir/fir/pubsub"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sj14/astral/pkg/astral"
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
var latitude float64
var longitude float64
var elevation float64
var observer astral.Observer

func NewWxIndex(pubsub pubsub.Adapter) *index {
	c := &index{
		model:       &App{},
		pubsub:      pubsub,
		eventSender: make(chan fir.Event),
		id:          "wx-app",
	}

	go func() {
		buf := make([]byte, 1024)

		for {
			n, _, _ := conn.ReadFromUDP(buf)

			data := buf[:n]
			msg := make(map[string]interface{})
			json.Unmarshal(data, &msg)

			if ttype, ok := msg["type"]; ok {
				if ttype == "hub_status" {

					json.Unmarshal(data, &lastHubStatus)
					c.eventSender <- fir.NewEvent("hub", nil)

				} else if ttype == "obs_st" {
					if os.Getenv("DB_PATH") != "" {
						logReading("obs_st", data)
					}

					json.Unmarshal(data, &lastReading)
					c.eventSender <- fir.NewEvent("reading", nil)
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
		fir.OnLoad(i.load),
		fir.OnEvent("reading", i.updateReading),
		fir.OnEvent("hub", i.updateHub),
		fir.EventSender(i.eventSender),
	}
}

// /prefs route handler
func prefs() fir.RouteOptions {
	return fir.RouteOptions{
		fir.ID("prefs"),
		fir.Content("prefs.html"),
		fir.OnLoad(func(ctx fir.RouteContext) error {
			return ctx.Data(map[string]any{})
		}),
	}
}

// /charts route handler
func charts() fir.RouteOptions {
	return fir.RouteOptions{
		fir.ID("charts"),
		fir.Content("charts.html"),
		fir.OnLoad(func(ctx fir.RouteContext) error {
			return ctx.Data(map[string]any{})
		}),
	}
}

// /maps route handler
func maps() fir.RouteOptions {
	return fir.RouteOptions{
		fir.ID("maps"),
		fir.Content("maps.html"),
		fir.OnLoad(func(ctx fir.RouteContext) error {
			return ctx.Data(map[string]any{})
		}),
	}
}

// load is called when the page is loaded.
func (i *index) load(ctx fir.RouteContext) error {
	reading := formatReading(lastReading)

	hubStatus := formatHubStatus(lastHubStatus)

	reading["station"] = os.Getenv("STATION")

	if os.Getenv("LATITUDE") != "" {
		sunrise, _ := astral.Sunrise(observer, time.Now())
		sunset, _ := astral.Sunset(observer, time.Now())

		reading["sunrise"] = sunrise.Format("15:04")
		reading["sunset"] = sunset.Format("15:04")
	} else {
		reading["sunrise"] = ""
		reading["sunset"] = ""
	}

	for key, value := range hubStatus {
		reading[key] = value
	}

	if os.Getenv("DB_PATH") != "" {
		reading["chartIcon"] = "show"
	} else {
		reading["chartIcon"] = "hide"
	}

	return ctx.Data(reading)
}

// updateReading is called when the "reading" event is received.
func (i *index) updateReading(ctx fir.RouteContext) error {
	reading := formatReading(lastReading)
	return ctx.Data(reading)
}

// updateHub is called when the "hub" event is received.
func (i *index) updateHub(ctx fir.RouteContext) error {

	hub := formatHubStatus(lastHubStatus)

	hub["station"] = os.Getenv("STATION")

	if os.Getenv("LATITUDE") != "" {
		sunrise, _ := astral.Sunrise(observer, time.Now())
		sunset, _ := astral.Sunset(observer, time.Now())

		hub["sunrise"] = sunrise.Format("15:04")
		hub["sunset"] = sunset.Format("15:04")
	} else {
		hub["sunrise"] = ""
		hub["sunset"] = ""
	}

	return ctx.Data(hub)
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

// there's only one user and setting this in the context prevents
// fir from spewing out needless messages about the user not being set.
func SetUserId(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), fir.UserKey, "wx")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func now(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	res, _ := json.Marshal(lastReading)
	w.Write(res)
}

func since(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	queryParameters := r.URL.Query()

	value := queryParameters.Get("ts")
	parsedDate, err := parseDateOrDateTime(value)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("[]"))
	} else {
		res, err := sinceQuery(parsedDate)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("[]"))
		}
		w.Write([]byte(res))
	}
}

func main() {

	initUDPListener()
	defer conn.Close()

	if os.Getenv("DB_PATH") != "" {
		db = initDB(os.Getenv("DB_PATH"))
		defer db.Close()
	}

	if os.Getenv("LATITUDE") != "" {
		latitude, _ = strconv.ParseFloat(os.Getenv("LATITUDE"), 64)
		longitude, _ = strconv.ParseFloat(os.Getenv("LONGITUDE"), 64)
		elevation, _ = strconv.ParseFloat(os.Getenv("ELEVATION"), 64)
		observer = astral.Observer{
			Latitude:  latitude,
			Longitude: longitude,
			Elevation: elevation,
		}
	}

	pubsubAdapter := pubsub.NewInmem()

	controller := fir.NewController(
		"wx-app",
		fir.DevelopmentMode(true),
		fir.WithPubsubAdapter(pubsubAdapter),
	)

	http.Handle("/", SetUserId(controller.Route(NewWxIndex(pubsubAdapter))))

	http.Handle("/prefs", controller.RouteFunc(prefs))

	http.Handle("/maps", controller.RouteFunc(maps))

	http.HandleFunc("/now", now)

	if os.Getenv("DB_PATH") != "" {
		http.Handle("/charts", controller.RouteFunc(charts))
		http.HandleFunc("/since", since)
	}

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

	http.ListenAndServe("0.0.0.0:"+os.Getenv("PORT"), nil)
}
