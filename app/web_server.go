package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

const browserTimeoutSecs = 5

// IncomingHttpMessage represents a message to the web server.
type IncomingHttpMessage struct {
	Query string `json:"query"`
}

// OutgoingHttpMessage respresents a response from the web server.
type OutgoingHttpMessage struct {
	Status string `json:"status"`
	Result any    `json:"result"`
}

type WebServer struct {
	host string
	port int
	// Map UUIDs of HTTP requests to a channel where we send their browser response.
	browserResponders map[string]chan IncomingBrowserMessage
}

func NewWebServer(host string, port int) WebServer {
	return WebServer{
		host:              host,
		port:              port,
		browserResponders: make(map[string]chan IncomingBrowserMessage),
	}
}

func (ws *WebServer) Start() {
	http.Handle("/", http.HandlerFunc(ws.handlePost))
	go func() {
		err := http.ListenAndServe(fmt.Sprintf("%v:%v", ws.host, ws.port), nil)
		if err != nil {
			Error.Printf("Unable to open HTTP server: %v", err)
		}
	}()
	Trace.Printf("Opened HTTP server on http://%v:%v", ws.host, ws.port)
}

func (ws *WebServer) GetBrowserResponder(id string) chan IncomingBrowserMessage {
	return ws.browserResponders[id]
}

func (ws *WebServer) handlePost(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		Error.Printf("Invalid path %v", req.URL.Path)
		respondJson(w, http.StatusNotFound, OutgoingHttpMessage{Status: "not found"})
		return
	}
	if req.Method != "POST" {
		Error.Printf("Invalid method %v", req.Method)
		respondJson(w, http.StatusMethodNotAllowed, OutgoingHttpMessage{Status: "invalid method"})
		return
	}

	Trace.Printf("Got POST request")
	var msg IncomingHttpMessage
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&msg)
	if err != nil {
		Error.Printf("Error parsing POST request: %v", err)
		respondJson(w, http.StatusBadRequest, OutgoingHttpMessage{Status: "invalid JSON"})
		return
	}

	// send message to browser with a random ID, and listen for messages from browser with that ID
	uuid := uuid.NewString()
	browserResponder := make(chan IncomingBrowserMessage)
	ws.browserResponders[uuid] = browserResponder
	defer delete(ws.browserResponders, uuid)
	sendToBrowser(OutgoingBrowserMessage{Id: uuid, Query: msg.Query})

	// wait for a browser message or a timeout
	select {
	case browserResponse := <-browserResponder:
		respondJson(w, http.StatusOK, OutgoingHttpMessage{Status: browserResponse.Status, Result: browserResponse.Result})
	case <-time.After(browserTimeoutSecs * time.Second):
		Error.Printf("Timeout responding to request ID %v", uuid)
		respondJson(w, http.StatusInternalServerError, OutgoingHttpMessage{Status: "timeout"})
	}
}
