package web_server

import (
	"example/remote/internal/logger"
	"example/remote/internal/shared"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type TestSenderToBrowser struct {
	messages chan shared.MessageToBrowser
}

func NewTestSenderToBrowser() TestSenderToBrowser {
	return TestSenderToBrowser{
		messages: make(chan shared.MessageToBrowser),
	}
}

func (resp *TestSenderToBrowser) SendMessage(msg shared.MessageToBrowser) {
	resp.messages <- msg
}

func TestWebServer(t *testing.T) {
	logger := logger.NewStdout()
	sender := NewTestSenderToBrowser()
	ws := New(&logger, &sender)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{ \"query\": \"name\" }"))
	recorder := httptest.NewRecorder()

	postDone := make(chan bool)
	go func() {
		ws.ServeHttp(recorder, req)
		postDone <- true
	}()

	msg := <-sender.messages
	if msg.Query != "name" {
		t.Errorf("invalid message sent to web server: %v", msg.Query)
	}
	ws.HandleMessage(shared.MessageFromBrowser{Id: msg.Id, Status: "ok", Results: []any{"john"}})

	<-postDone
	resp := recorder.Result()
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}
	if string(body) != "{\"status\":\"ok\",\"results\":[\"john\"]}\n" {
		t.Errorf("invalid response received from web server: %v", string(body))
	}
}
