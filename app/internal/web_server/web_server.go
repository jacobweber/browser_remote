package web_server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jacobweber/browser_remote/internal/logger"
	"github.com/jacobweber/browser_remote/internal/mutex_map"
	"github.com/jacobweber/browser_remote/internal/shared"

	"github.com/google/uuid"
)

type TimerKey struct{}

const browserTimeoutSecs = 5

type WebServer struct {
	logger          *logger.Logger
	senderToBrowser func(shared.MessageToBrowser)
	// Map UUIDs of HTTP requests to a channel where we send their browser response.
	messageFromBrowserHandlers *mutex_map.MutexMap[string, chan shared.MessageFromBrowser]
	server                     *http.ServeMux
}

func New(logger *logger.Logger) *WebServer {
	server := http.NewServeMux()
	ws := WebServer{
		logger:                     logger,
		senderToBrowser:            nil,
		messageFromBrowserHandlers: mutex_map.New[string, chan shared.MessageFromBrowser](),
		server:                     server,
	}
	ws.server.Handle("/", http.HandlerFunc(ws.HandlePost))
	return &ws
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

func (ws *WebServer) OnMessageReadyForBrowser(handler func(shared.MessageToBrowser)) {
	ws.senderToBrowser = handler
}

func (ws *WebServer) HandleMessageFromBrowser(incomingMsg shared.MessageFromBrowser) {
	if incomingMsg.Id != "" {
		responder := ws.messageFromBrowserHandlers.Get(incomingMsg.Id)
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
		respondJson(w, http.StatusNotFound, shared.MessageFromWebServer{Status: "not found", Results: []any{}})
		return
	}
	if req.Method != "POST" {
		ws.logger.Error.Printf("Invalid method %v", req.Method)
		respondJson(w, http.StatusMethodNotAllowed, shared.MessageFromWebServer{Status: "invalid method", Results: []any{}})
		return
	}

	ws.logger.Trace.Printf("Got POST request")
	var msg shared.MessageToWebServer
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&msg)
	if err != nil {
		ws.logger.Error.Printf("Error parsing POST request: %v", err)
		respondJson(w, http.StatusBadRequest, shared.MessageFromWebServer{Status: "invalid JSON", Results: []any{}})
		return
	}

	// send message to browser with a random ID, and listen for messages from browser with that ID
	uuid := uuid.NewString()
	messageFromBrowserHandler := make(chan shared.MessageFromBrowser)
	ws.messageFromBrowserHandlers.Set(uuid, messageFromBrowserHandler)
	defer ws.messageFromBrowserHandlers.Delete(uuid)
	if ws.senderToBrowser != nil {
		ws.senderToBrowser(shared.MessageToBrowser{Id: uuid, Query: msg.Query, Tabs: msg.Tabs})
	}

	var timer shared.Timer
	timer, ok := req.Context().Value(TimerKey{}).(shared.Timer)
	if !ok {
		timer = &shared.RealTimer{}
	}

	// wait for a browser message or a timeout
	select {
	case messageFromBrowser := <-messageFromBrowserHandler:
		respondJson(w, http.StatusOK, shared.MessageFromWebServer{Status: messageFromBrowser.Status, Results: messageFromBrowser.Results})
	case <-timer.StartTimer(browserTimeoutSecs * time.Second):
		ws.logger.Error.Printf("Timeout responding to request ID %v", uuid)
		respondJson(w, http.StatusInternalServerError, shared.MessageFromWebServer{Status: "timeout", Results: []any{}})
	}
}

func respondJson(w http.ResponseWriter, statusCode int, msg shared.MessageFromWebServer) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}
