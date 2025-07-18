package main

import (
	"strconv"
	"sync"
	"testing"

	"github.com/jacobweber/browser_remote/internal/shared"
	"github.com/jacobweber/browser_remote/internal/testing/browser_remote_tester"
)

func TestApp(t *testing.T) {
	br := browser_remote_tester.New()
	br.Start()

	t.Run("responds to query", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		msg := <-listener
		br.SendResponseFromBrowser(msg.Id, "ok", []any{"john"})
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"ok\",\"results\":[\"john\"]}\n", t)
	})

	t.Run("ignores browser responses with invalid IDs", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		msg := <-listener
		br.SendResponseFromBrowser("xxx", "ok", []any{"jim"})
		br.SendResponseFromBrowser(msg.Id, "ok", []any{"john"})
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"ok\",\"results\":[\"john\"]}\n", t)
	})

	t.Run("responds with browser error", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		msg := <-listener
		br.SendResponseFromBrowser(msg.Id, "error", []any{})
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"error\",\"results\":[]}\n", t)
	})

	t.Run("responds with timeout error", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, timeout := br.SendRequestToWeb("{\"query\":\"name\"}")
		<-listener
		timeout.FireTimer()
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"timeout\",\"results\":[]}\n", t)
	})

	t.Run("handles overlapping calls", func(t *testing.T) {
		listener2 := br.ListenForQueryToBrowser("age")
		listener1 := br.ListenForQueryToBrowser("name")
		postDone1, recorder1, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		postDone2, recorder2, _ := br.SendRequestToWeb("{\"query\":\"age\"}")

		var msg1, msg2 shared.MessageToBrowser
		done := 0
		for done < 2 {
			select {
			case v := <-listener1:
				msg1 = v
				done++
			case v := <-listener2:
				msg2 = v
				done++
			}
		}

		br.SendResponseFromBrowser(msg2.Id, "ok", []any{31})
		br.SendResponseFromBrowser(msg1.Id, "ok", []any{"john"})
		br.AssertResponseFromWeb(postDone1, recorder1, "{\"status\":\"ok\",\"results\":[\"john\"]}\n", t)
		br.AssertResponseFromWeb(postDone2, recorder2, "{\"status\":\"ok\",\"results\":[31]}\n", t)
	})

	t.Run("handles parallel calls", func(t *testing.T) {
		var wg sync.WaitGroup
		for idInt := range 10 {
			wg.Add(1)
			go func() {
				id := strconv.Itoa(idInt)
				defer wg.Done()
				listener := br.ListenForQueryToBrowser("name" + id)
				postDone, recorder, _ := br.SendRequestToWeb("{\"query\":\"name" + id + "\"}")
				msg := <-listener
				br.SendResponseFromBrowser(msg.Id, "ok", []any{"john" + id})
				br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"ok\",\"results\":[\"john"+id+"\"]}\n", t)
			}()
		}
		wg.Wait()
	})

	br.Cleanup()
}
