package main

import (
	"example/remote/internal/testing/browser_remote_tester"
	"strconv"
	"sync"
	"testing"
)

func TestApp(t *testing.T) {
	t.Run("responds to test", func(t *testing.T) {
		br := browser_remote_tester.NewBrowserRemoteTester()
		br.Start()
		listener := br.ListenForBrowserToReceiveQuery("name")
		postDone, recorder, _ := br.SendWebRequest("{\"query\":\"name\"}")
		msg := <-listener
		br.SendBrowserResponse(msg.Id, "ok", "john")
		br.AssertWebResponse(postDone, recorder, "{\"status\":\"ok\",\"result\":\"john\"}\n", t)
		br.Cleanup()
	})

	t.Run("ignores browser responses with invalid IDs", func(t *testing.T) {
		br := browser_remote_tester.NewBrowserRemoteTester()
		br.Start()
		listener := br.ListenForBrowserToReceiveQuery("name")
		postDone, recorder, _ := br.SendWebRequest("{\"query\":\"name\"}")
		msg := <-listener
		br.SendBrowserResponse("xxx", "ok", "jim")
		br.SendBrowserResponse(msg.Id, "ok", "john")
		br.AssertWebResponse(postDone, recorder, "{\"status\":\"ok\",\"result\":\"john\"}\n", t)
		br.Cleanup()
	})

	t.Run("responds with browser error", func(t *testing.T) {
		br := browser_remote_tester.NewBrowserRemoteTester()
		br.Start()
		listener := br.ListenForBrowserToReceiveQuery("name")
		postDone, recorder, _ := br.SendWebRequest("{\"query\":\"name\"}")
		msg := <-listener
		br.SendBrowserResponse(msg.Id, "error", "")
		br.AssertWebResponse(postDone, recorder, "{\"status\":\"error\",\"result\":\"\"}\n", t)
		br.Cleanup()
	})

	t.Run("responds with timeout error", func(t *testing.T) {
		br := browser_remote_tester.NewBrowserRemoteTester()
		br.Start()
		listener := br.ListenForBrowserToReceiveQuery("name")
		postDone, recorder, timeout := br.SendWebRequest("{\"query\":\"name\"}")
		<-listener
		timeout.FireTimer()
		br.AssertWebResponse(postDone, recorder, "{\"status\":\"timeout\",\"result\":null}\n", t)
		br.Cleanup()
	})

	t.Run("handles overlapping calls", func(t *testing.T) {
		br := browser_remote_tester.NewBrowserRemoteTester()
		br.Start()
		listener2 := br.ListenForBrowserToReceiveQuery("age")
		listener1 := br.ListenForBrowserToReceiveQuery("name")
		postDone1, recorder1, _ := br.SendWebRequest("{\"query\":\"name\"}")
		postDone2, recorder2, _ := br.SendWebRequest("{\"query\":\"age\"}")
		msg2 := <-listener2
		msg1 := <-listener1
		br.SendBrowserResponse(msg2.Id, "ok", "31")
		br.SendBrowserResponse(msg1.Id, "ok", "john")
		br.AssertWebResponse(postDone1, recorder1, "{\"status\":\"ok\",\"result\":\"john\"}\n", t)
		br.AssertWebResponse(postDone2, recorder2, "{\"status\":\"ok\",\"result\":\"31\"}\n", t)
		br.Cleanup()
	})

	t.Run("handles parallel calls", func(t *testing.T) {
		br := browser_remote_tester.NewBrowserRemoteTester()
		br.Start()
		var wg sync.WaitGroup
		for idInt := range 10 {
			wg.Add(1)
			go func() {
				id := strconv.Itoa(idInt)
				defer wg.Done()
				listener := br.ListenForBrowserToReceiveQuery("name" + id)
				postDone, recorder, _ := br.SendWebRequest("{\"query\":\"name" + id + "\"}")
				msg := <-listener
				br.SendBrowserResponse(msg.Id, "ok", "john"+id)
				br.AssertWebResponse(postDone, recorder, "{\"status\":\"ok\",\"result\":\"john"+id+"\"}\n", t)
			}()
		}
		wg.Wait()
		br.Cleanup()
	})
}
