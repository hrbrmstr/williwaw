package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/alexflint/go-arg"
	_ "github.com/joho/godotenv/autoload"
	"github.com/livefir/fir"
	"github.com/livefir/fir/pubsub"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/exp/slog"
)

type WillowParams struct {
	SeekritToken string `arg:"-s,--seekrit,env:SEEKRIT_TOKEN" help:"token enabling remote disabling" placeholder:"SEEKRIT_TOKEN"`
	DbPath       string `arg:"-p,--path,env:DB_PATH" help:"Full path to datalogger file db" placeholder:"DB_PATH"`
	ListenPort   string `arg:"-l,--listen-port,env:PORT" help:"port to listen on" placeholder:"PORT" default:"9867"`
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

var willowParams WillowParams
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

// load is called when the page is loaded.
func (i *index) load(ctx fir.RouteContext) error {
	reading := formatReading(lastReading)
	hubStatus := formatHubStatus(lastHubStatus)
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
	return ctx.Data(formatReading(lastReading))
}

// updateHub is called when the "hub" event is received.
func (i *index) updateHub(ctx fir.RouteContext) error {
	return ctx.Data(formatHubStatus(lastHubStatus))
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

	arg.MustParse(&willowParams)

	initUDPListener()
	defer conn.Close()

	if willowParams.DbPath != "" {
		db = initDB(os.Getenv("DB_PATH"))
		defer db.Close()
	}

	pubsubAdapter := pubsub.NewInmem()

	controller := fir.NewController(
		"wx-app",
		fir.DevelopmentMode(true),
		fir.WithPubsubAdapter(pubsubAdapter),
	)

	http.Handle("/", SetUserId(controller.Route(NewWxIndex(pubsubAdapter))))

	http.Handle("/prefs", controller.RouteFunc(prefs))

	if willowParams.DbPath != "" {
		http.Handle("/charts", controller.RouteFunc(charts))
		http.HandleFunc("/since", since)
	}

	if willowParams.SeekritToken == "" {
		http.HandleFunc("/quit", func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			secretToken := willowParams.SeekritToken

			if token == secretToken {
				slog.Info("Shutting down...")
				conn.Close()
				if willowParams.DbPath != "" {
					db.Close()
				}
				os.Exit(0)
			} else {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
		})
	}

	http.ListenAndServe("0.0.0.0:"+willowParams.ListenPort, nil)
}
