package web_server

import (
	"encoding/json"
	"example/remote/internal/logger"
	"example/remote/internal/mutex_map"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type TimerKey struct{}

const browserTimeoutSecs = 5

// MessageFromBrowser represents a message from the browser to the native host.
type MessageFromBrowser struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Result any    `json:"result"`
}

// MessageToBrowser respresents a response from the native host to the browser.
type MessageToBrowser struct {
	Id    string `json:"id"`
	Query string `json:"query"`
}

// MessageToWebServer represents a message to the web server.
type MessageToWebServer struct {
	Query string `json:"query"`
}

// MessageFromWebServer respresents a response from the web server.
type MessageFromWebServer struct {
	Status string `json:"status"`
	Result any    `json:"result"`
}

type SenderToBrowser interface {
	SendMessage(msg MessageToBrowser)
}

type Timer interface {
	StartTimer(time.Duration) <-chan time.Time
}

type RealTimer struct {
}

func (timer *RealTimer) StartTimer(dur time.Duration) <-chan time.Time {
	return time.After(dur)
}

type WebServer struct {
	logger *logger.Logger
	// Map UUIDs of HTTP requests to a channel where we send their browser response.
	browserResponders *mutex_map.MutexMap[string, chan MessageFromBrowser]
	sender            SenderToBrowser
	server            *http.ServeMux
}

func NewWebServer(logger *logger.Logger, sender SenderToBrowser) WebServer {
	server := http.NewServeMux()
	ws := WebServer{
		logger:            logger,
		browserResponders: mutex_map.NewMap[string, chan MessageFromBrowser](),
		sender:            sender,
		server:            server,
	}
	ws.server.Handle("/", http.HandlerFunc(ws.HandlePost))
	return ws
}

func (ws *WebServer) Start(host string, port int) {
	go func() {
		err := http.ListenAndServe(fmt.Sprintf("%v:%v", host, port), ws.server)
		if err != nil {
			ws.logger.Error.Printf("Unable to open HTTP server: %v", err)
		}
	}()
	ws.logger.Trace.Printf("Opened HTTP server on http://%v:%v", host, port)
}

func (ws *WebServer) HandleMessage(incomingMsg MessageFromBrowser) {
	if incomingMsg.Id != "" {
		responder := ws.browserResponders.Get(incomingMsg.Id)
		if responder != nil {
			ws.logger.Trace.Printf("Message received from browser for ID: %v", incomingMsg.Id)
			responder <- incomingMsg
		}
	}
}

func (ws *WebServer) ServeHttp(w http.ResponseWriter, req *http.Request) {
	ws.server.ServeHTTP(w, req)
}

func (ws *WebServer) HandlePost(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		ws.logger.Error.Printf("Invalid path %v", req.URL.Path)
		respondJson(w, http.StatusNotFound, MessageFromWebServer{Status: "not found"})
		return
	}
	if req.Method != "POST" {
		ws.logger.Error.Printf("Invalid method %v", req.Method)
		respondJson(w, http.StatusMethodNotAllowed, MessageFromWebServer{Status: "invalid method"})
		return
	}

	ws.logger.Trace.Printf("Got POST request")
	var msg MessageToWebServer
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&msg)
	if err != nil {
		ws.logger.Error.Printf("Error parsing POST request: %v", err)
		respondJson(w, http.StatusBadRequest, MessageFromWebServer{Status: "invalid JSON"})
		return
	}

	// send message to browser with a random ID, and listen for messages from browser with that ID
	uuid := uuid.NewString()
	browserResponder := make(chan MessageFromBrowser)
	ws.browserResponders.Set(uuid, browserResponder)
	defer ws.browserResponders.Delete(uuid)
	ws.sender.SendMessage(MessageToBrowser{Id: uuid, Query: msg.Query})

	var timer Timer
	timer, ok := req.Context().Value(TimerKey{}).(Timer)
	if !ok {
		timer = &RealTimer{}
	}

	// wait for a browser message or a timeout
	select {
	case browserResponse := <-browserResponder:
		respondJson(w, http.StatusOK, MessageFromWebServer{Status: browserResponse.Status, Result: browserResponse.Result})
	case <-timer.StartTimer(browserTimeoutSecs * time.Second):
		ws.logger.Error.Printf("Timeout responding to request ID %v", uuid)
		respondJson(w, http.StatusInternalServerError, MessageFromWebServer{Status: "timeout"})
	}
}

func respondJson(w http.ResponseWriter, statusCode int, msg MessageFromWebServer) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}
