package web_server

import (
	"encoding/json"
	"example/remote/internal/logger"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const browserTimeoutSecs = 5

// IncomingBrowserMessage represents a message from the browser to the native host.
type IncomingBrowserMessage struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Result any    `json:"result"`
}

// OutgoingBrowserMessage respresents a response from the native host to the browser.
type OutgoingBrowserMessage struct {
	Id    string `json:"id"`
	Query string `json:"query"`
}

// IncomingHttpMessage represents a message to the web server.
type IncomingHttpMessage struct {
	Query string `json:"query"`
}

// OutgoingHttpMessage respresents a response from the web server.
type OutgoingHttpMessage struct {
	Status string `json:"status"`
	Result any    `json:"result"`
}

type BrowserSender interface {
	SendToBrowser(msg OutgoingBrowserMessage)
}

type WebServer struct {
	logger *logger.Logger
	host   string
	port   int
	// Map UUIDs of HTTP requests to a channel where we send their browser response.
	browserResponders map[string]chan IncomingBrowserMessage
	sender            BrowserSender
}

func NewWebServer(logger *logger.Logger, host string, port int) WebServer {
	return WebServer{
		logger:            logger,
		host:              host,
		port:              port,
		browserResponders: make(map[string]chan IncomingBrowserMessage),
	}
}

func (ws *WebServer) Start(sender BrowserSender) {
	ws.sender = sender // TODO: improve
	http.Handle("/", http.HandlerFunc(ws.handlePost))
	go func() {
		err := http.ListenAndServe(fmt.Sprintf("%v:%v", ws.host, ws.port), nil)
		if err != nil {
			ws.logger.Error.Printf("Unable to open HTTP server: %v", err)
		}
	}()
	ws.logger.Trace.Printf("Opened HTTP server on http://%v:%v", ws.host, ws.port)
}

func (ws *WebServer) HandleMessage(incomingMsg IncomingBrowserMessage) {
	if incomingMsg.Id != "" {
		responder := ws.browserResponders[incomingMsg.Id]
		if responder != nil {
			ws.logger.Trace.Printf("Message received from browser for ID: %v", incomingMsg.Id)
			responder <- incomingMsg
		}
	}
}

func (ws *WebServer) handlePost(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		ws.logger.Error.Printf("Invalid path %v", req.URL.Path)
		respondJson(w, http.StatusNotFound, OutgoingHttpMessage{Status: "not found"})
		return
	}
	if req.Method != "POST" {
		ws.logger.Error.Printf("Invalid method %v", req.Method)
		respondJson(w, http.StatusMethodNotAllowed, OutgoingHttpMessage{Status: "invalid method"})
		return
	}

	ws.logger.Trace.Printf("Got POST request")
	var msg IncomingHttpMessage
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&msg)
	if err != nil {
		ws.logger.Error.Printf("Error parsing POST request: %v", err)
		respondJson(w, http.StatusBadRequest, OutgoingHttpMessage{Status: "invalid JSON"})
		return
	}

	// send message to browser with a random ID, and listen for messages from browser with that ID
	uuid := uuid.NewString()
	browserResponder := make(chan IncomingBrowserMessage)
	ws.browserResponders[uuid] = browserResponder
	defer delete(ws.browserResponders, uuid)
	ws.sender.SendToBrowser(OutgoingBrowserMessage{Id: uuid, Query: msg.Query})

	// wait for a browser message or a timeout
	select {
	case browserResponse := <-browserResponder:
		respondJson(w, http.StatusOK, OutgoingHttpMessage{Status: browserResponse.Status, Result: browserResponse.Result})
	case <-time.After(browserTimeoutSecs * time.Second):
		ws.logger.Error.Printf("Timeout responding to request ID %v", uuid)
		respondJson(w, http.StatusInternalServerError, OutgoingHttpMessage{Status: "timeout"})
	}
}

func respondJson(w http.ResponseWriter, statusCode int, msg OutgoingHttpMessage) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
}
