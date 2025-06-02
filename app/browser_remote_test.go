package main

import (
	"example/remote/internal/testing/browser_remote_tester"
	"strconv"
	"sync"
	"testing"
)

func TestApp(t *testing.T) {
	br := browser_remote_tester.NewBrowserRemoteTester()
	br.Start()

	t.Run("responds to query", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		msg := <-listener
		br.SendResponseFromBrowser(msg.Id, "ok", "john")
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"ok\",\"result\":\"john\"}\n", t)
	})

	t.Run("ignores browser responses with invalid IDs", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		msg := <-listener
		br.SendResponseFromBrowser("xxx", "ok", "jim")
		br.SendResponseFromBrowser(msg.Id, "ok", "john")
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"ok\",\"result\":\"john\"}\n", t)
	})

	t.Run("responds with browser error", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		msg := <-listener
		br.SendResponseFromBrowser(msg.Id, "error", "")
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"error\",\"result\":\"\"}\n", t)
	})

	t.Run("responds with timeout error", func(t *testing.T) {
		listener := br.ListenForQueryToBrowser("name")
		postDone, recorder, timeout := br.SendRequestToWeb("{\"query\":\"name\"}")
		<-listener
		timeout.FireTimer()
		br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"timeout\",\"result\":null}\n", t)
	})

	t.Run("handles overlapping calls", func(t *testing.T) {
		listener2 := br.ListenForQueryToBrowser("age")
		listener1 := br.ListenForQueryToBrowser("name")
		postDone1, recorder1, _ := br.SendRequestToWeb("{\"query\":\"name\"}")
		postDone2, recorder2, _ := br.SendRequestToWeb("{\"query\":\"age\"}")
		msg2 := <-listener2
		msg1 := <-listener1
		br.SendResponseFromBrowser(msg2.Id, "ok", "31")
		br.SendResponseFromBrowser(msg1.Id, "ok", "john")
		br.AssertResponseFromWeb(postDone1, recorder1, "{\"status\":\"ok\",\"result\":\"john\"}\n", t)
		br.AssertResponseFromWeb(postDone2, recorder2, "{\"status\":\"ok\",\"result\":\"31\"}\n", t)
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
				br.SendResponseFromBrowser(msg.Id, "ok", "john"+id)
				br.AssertResponseFromWeb(postDone, recorder, "{\"status\":\"ok\",\"result\":\"john"+id+"\"}\n", t)
			}()
		}
		wg.Wait()
	})

	br.Cleanup()
}
