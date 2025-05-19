/*
Request format:
POST /
{
	// an expression to evaluate and return:
	"query": "location.href"
	// or a function call:
	"query": "window.open(\"https://www.apple.com\")"
}

Response format:
{
	"status": "ok" | "error"
	// JSON-encoded response:
	"result": "\"https://www.google.com\""
}
*/

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"
	"unsafe"

	"github.com/google/uuid"
)

// command-line flags
var host = flag.String("host", "localhost", "listen hostname")
var port = flag.String("port", "5555", "listen port")

// constants for Logger
var (
	// Trace logs general information messages.
	Trace *log.Logger
	// Error logs error messages.
	Error *log.Logger
)

// nativeEndian used to detect native byte order
var nativeEndian binary.ByteOrder

// bufferSize used to set size of IO buffer - adjust to accommodate message payloads
var bufferSize = 8192

// IncomingBrowserMessage represents a message from the browser to the native host.
type IncomingBrowserMessage struct {
	Id     string `json:"id"`
	Result string `json:"result"`
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
	Result string `json:"result"`
}

var browserResponders map[string]chan IncomingBrowserMessage

// Init initializes logger and determines native byte order.
func Init(traceHandle io.Writer, errorHandle io.Writer) {
	Trace = log.New(traceHandle, "TRACE: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(errorHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	// determine native byte order so that we can read message size correctly
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		nativeEndian = binary.BigEndian
	} else {
		nativeEndian = binary.LittleEndian
	}
}

func main() {
	file, err := os.OpenFile("browser_remote.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		Init(os.Stdout, os.Stderr)
		Error.Printf("Unable to create and/or open log file. Will log to Stdout and Stderr. Error: %v", err)
	} else {
		Init(file, file)
		// ensure we close the log file when we're done
		defer file.Close()
	}
	browserResponders = make(map[string]chan IncomingBrowserMessage)

	flag.Parse()
	if !waitForPortFree(*host, *port, 10) {
		Error.Printf("Unable to open port: %v:%v", *host, *port)
		return
	}

	http.Handle("/", http.HandlerFunc(handlePost))
	go func() {
		err = http.ListenAndServe(*host+":"+*port, nil)
		if err != nil {
			Error.Printf("Unable to open HTTP server: %v", err)
		}
	}()
	Trace.Printf("Opened HTTP server on http://%v:%v", *host, *port)

	Trace.Printf("Chrome native messaging host started. Native byte order: %v.", nativeEndian)
	read()
	Trace.Print("Chrome native messaging host exited.")
}

func handlePost(w http.ResponseWriter, req *http.Request) {
	Trace.Printf("Got POST request")
	var msg IncomingHttpMessage
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&msg)
	if err != nil {
		Error.Printf("Error parsing POST request: %v", err)
		return
	}

	// send message to browser with a random ID, and listen for messages from browser with that ID
	uuid := uuid.NewString()
	browserResponder := make(chan IncomingBrowserMessage)
	browserResponders[uuid] = browserResponder
	defer delete(browserResponders, uuid)
	sendToBrowser(OutgoingBrowserMessage{Id: uuid, Query: msg.Query})

	// wait for a browser message or a timeout
	w.Header().Set("Content-Type", "application/json")
	select {
	case browserResponse := <-browserResponder:
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(OutgoingHttpMessage{Status: "ok", Result: browserResponse.Result})
	case <-time.After(30 * time.Second):
		Error.Printf("Timeout responding to request ID %v", uuid)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(OutgoingHttpMessage{Status: "timeout"})
	}
}

func isPortInUse(host string, port string, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil || conn == nil {
		return false
	}
	conn.Close()
	return true
}

func waitForPortFree(host string, port string, maxTries int) bool {
	timeout := time.Second / 2
	for i := 0; i < maxTries; i++ {
		if !isPortInUse(host, port, timeout) {
			return true
		}
		time.Sleep(timeout)
	}
	return false
}

// read creates a new buffered I/O reader and reads messages from Stdin.
func read() {
	v := bufio.NewReader(os.Stdin)
	// adjust buffer size to accommodate your json payload size limits; default is 4096
	s := bufio.NewReaderSize(v, bufferSize)
	Trace.Printf("IO buffer reader created with buffer size of %v.", s.Size())

	lengthBytes := make([]byte, 4)
	lengthNum := int(0)

	// we're going to indefinitely read the first 4 bytes in buffer, which gives us the message length.
	// if stdIn is closed we'll exit the loop and shut down host
	for b, err := s.Read(lengthBytes); b > 0 && err == nil; b, err = s.Read(lengthBytes) {
		// convert message length bytes to integer value
		lengthNum = readMessageLength(lengthBytes)
		Trace.Printf("Message size in bytes: %v", lengthNum)

		// If message length exceeds size of buffer, the message will be truncated.
		// This will likely cause an error when we attempt to unmarshal message to JSON.
		if lengthNum > bufferSize {
			Error.Printf("Message size of %d exceeds buffer size of %d. Message will be truncated and is unlikely to unmarshal to JSON.", lengthNum, bufferSize)
		}

		// read the content of the message from buffer
		content := make([]byte, lengthNum)
		_, err := s.Read(content)
		if err != nil && err != io.EOF {
			Error.Fatal(err)
		}

		// message has been read, now parse and process
		handleMessageFromBrowser(content)
	}

	Trace.Print("Stdin closed.")
}

// readMessageLength reads and returns the message length value in native byte order.
func readMessageLength(msg []byte) int {
	var length uint32
	buf := bytes.NewBuffer(msg)
	err := binary.Read(buf, nativeEndian, &length)
	if err != nil {
		Error.Printf("Unable to read bytes representing message length: %v", err)
	}
	return int(length)
}

// handleMessageFromBrowser parses incoming message from browser
func handleMessageFromBrowser(msg []byte) {
	incomingMsg := decodeMessageFromBrowser(msg)
	Trace.Printf("Message received from browser: %s", msg)
	if incomingMsg.Id != "" {
		responder := browserResponders[incomingMsg.Id]
		if responder != nil {
			Trace.Printf("Message received from browser for ID: %v", incomingMsg.Id)
			responder <- incomingMsg
		}
	}
}

// decodeMessageFromBrowser unmarshals incoming json request and returns query value.
func decodeMessageFromBrowser(msg []byte) IncomingBrowserMessage {
	var iMsg IncomingBrowserMessage
	err := json.Unmarshal(msg, &iMsg)
	if err != nil {
		Error.Printf("Unable to unmarshal json to struct: %v", err)
	}
	return iMsg
}

// sendToBrowser sends an OutgoingBrowserMessage to os.Stdout.
func sendToBrowser(msg OutgoingBrowserMessage) {
	byteMsg := dataToBytes(msg)
	writeMessageLength(byteMsg)

	var msgBuf bytes.Buffer
	_, err := msgBuf.Write(byteMsg)
	if err != nil {
		Error.Printf("Unable to write message length to message buffer: %v", err)
	}

	_, err = msgBuf.WriteTo(os.Stdout)
	if err != nil {
		Error.Printf("Unable to write message buffer to Stdout: %v", err)
	}
}

// dataToBytes marshals OutgoingBrowserMessage struct to slice of bytes
func dataToBytes(msg OutgoingBrowserMessage) []byte {
	byteMsg, err := json.Marshal(msg)
	if err != nil {
		Error.Printf("Unable to marshal OutgoingBrowserMessage struct to slice of bytes: %v", err)
	}
	return byteMsg
}

// writeMessageLength determines length of message and writes it to os.Stdout.
func writeMessageLength(msg []byte) {
	err := binary.Write(os.Stdout, nativeEndian, uint32(len(msg)))
	if err != nil {
		Error.Printf("Unable to write message length to Stdout: %v", err)
	}
}
