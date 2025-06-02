package native_messaging

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"example/remote/internal/logger"
	"example/remote/internal/shared"
	"io"
)

type Responder[I any] interface {
	HandleMessage(msg I)
}

type NativeMessagingReader[I any] struct {
	logger *logger.Logger

	inputHandle io.Reader

	// bufferSize used to set size of IO buffer - adjust to accommodate message payloads
	bufferSize int

	// nativeEndian used to detect native byte order
	nativeEndian binary.ByteOrder
}

func NewNativeMessagingReader[I any](logger *logger.Logger, inputHandle io.Reader) NativeMessagingReader[I] {
	return NativeMessagingReader[I]{
		logger:       logger,
		inputHandle:  inputHandle,
		bufferSize:   8192,
		nativeEndian: shared.DetermineByteOrder(),
	}
}

// ReadMessages creates a new buffered I/O reader and reads messages from inputFile.
func (nm *NativeMessagingReader[I]) Start(responder Responder[I]) {
	nm.logger.Trace.Printf("Native messaging host started. Native byte order: %v.", nm.nativeEndian)

	v := bufio.NewReader(nm.inputHandle)
	// adjust buffer size to accommodate your json payload size limits; default is 4096
	s := bufio.NewReaderSize(v, nm.bufferSize)
	nm.logger.Trace.Printf("IO buffer reader created with buffer size of %v.", s.Size())

	lengthBytes := make([]byte, 4)
	lengthNum := int(0)

	// we're going to indefinitely read the first 4 bytes in buffer, which gives us the message length.
	// if stdIn is closed we'll exit the loop and shut down host
	for b, err := s.Read(lengthBytes); b > 0 && err == nil; b, err = s.Read(lengthBytes) {
		// convert message length bytes to integer value
		lengthNum = nm.readMessageLength(lengthBytes)
		nm.logger.Trace.Printf("Message size in bytes: %v", lengthNum)

		// If message length exceeds size of buffer, the message will be truncated.
		// This will likely cause an error when we attempt to unmarshal message to JSON.
		if lengthNum > nm.bufferSize {
			nm.logger.Error.Printf("Message size of %d exceeds buffer size of %d. Message will be truncated and is unlikely to unmarshal to JSON.", lengthNum, nm.bufferSize)
		}

		// read the content of the message from buffer
		content := make([]byte, lengthNum)
		_, err := s.Read(content)
		if err != nil && err != io.EOF {
			nm.logger.Error.Fatal(err)
		}

		// message has been read, now parse and process
		nm.handleMessage(content, responder)
	}

	nm.logger.Trace.Print("Native messaging host exited.")
}

// readMessageLength reads and returns the message length value in native byte order.
func (nm *NativeMessagingReader[I]) readMessageLength(msg []byte) int {
	var length uint32
	buf := bytes.NewBuffer(msg)
	err := binary.Read(buf, nm.nativeEndian, &length)
	if err != nil {
		nm.logger.Error.Printf("Unable to read bytes representing message length: %v", err)
	}
	return int(length)
}

// handleMessage parses incoming message from input
func (nm *NativeMessagingReader[I]) handleMessage(msg []byte, responder Responder[I]) {
	incomingMsg := nm.decodeMessage(msg)
	nm.logger.Trace.Printf("Message received: %s", msg)
	responder.HandleMessage(incomingMsg)
}

// decodeMessage unmarshals incoming json request and returns query value.
func (nm *NativeMessagingReader[I]) decodeMessage(msg []byte) I {
	var incomingMsg I
	err := json.Unmarshal(msg, &incomingMsg)
	if err != nil {
		nm.logger.Error.Printf("Unable to unmarshal json to struct: %v", err)
	}
	return incomingMsg
}
