package native_messaging

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/jacobweber/browser_remote/internal/logger"
	"github.com/jacobweber/browser_remote/internal/shared"
)

type NativeMessagingReader[I any] struct {
	logger *logger.Logger

	name string

	inputHandle io.Reader

	// size of IO buffer - adjust to accommodate message payloads
	bufferSize int

	nativeEndian binary.ByteOrder

	messageHandler func(I)
}

func NewReader[I any](logger *logger.Logger, inputHandle io.Reader, name string) *NativeMessagingReader[I] {
	return &NativeMessagingReader[I]{
		logger:         logger,
		name:           name,
		inputHandle:    inputHandle,
		bufferSize:     8192,
		nativeEndian:   shared.DetermineByteOrder(),
		messageHandler: nil,
	}
}

func (nm *NativeMessagingReader[I]) OnMessageRead(handler func(I)) {
	nm.messageHandler = handler
}

// Creates a new buffered I/O reader and reads messages from inputFile.
func (nm *NativeMessagingReader[I]) Start() {
	nm.logger.Trace.Printf("%v: reader started with native byte order: %v", nm.name, nm.nativeEndian)

	v := bufio.NewReader(nm.inputHandle)
	// adjust buffer size to accommodate your json payload size limits; default is 4096
	s := bufio.NewReaderSize(v, nm.bufferSize)
	nm.logger.Trace.Printf("%v: buffer reader created with buffer size of %v", nm.name, s.Size())

	lengthBytes := make([]byte, 4)
	lengthNum := int(0)

	// we're going to indefinitely read the first 4 bytes in buffer, which gives us the message length.
	// if stdIn is closed we'll exit the loop and shut down host
	for b, err := s.Read(lengthBytes); b > 0 && err == nil; b, err = s.Read(lengthBytes) {
		// convert message length bytes to integer value
		lengthNum = nm.readMessageLength(lengthBytes)
		nm.logger.Trace.Printf("%v: read message size in bytes: %v", nm.name, lengthNum)

		// If message length exceeds size of buffer, the message will be truncated.
		// This will likely cause an error when we attempt to unmarshal message to JSON.
		if lengthNum > nm.bufferSize {
			nm.logger.Error.Printf("%v: message size of %d exceeds buffer size of %d; message will be truncated and is unlikely to unmarshal to JSON", nm.name, lengthNum, nm.bufferSize)
		}

		// read the content of the message from buffer
		content := make([]byte, lengthNum)
		_, err := s.Read(content)
		if err != nil && err != io.EOF {
			nm.logger.Error.Fatalf("%v: %v", nm.name, err)
		}

		// message has been read, now parse and process
		nm.handleMessage(content)
	}

	nm.logger.Trace.Printf("%v: reader exited", nm.name)
}

// Reads and returns the message length value in native byte order.
func (nm *NativeMessagingReader[I]) readMessageLength(msg []byte) int {
	var length uint32
	buf := bytes.NewBuffer(msg)
	err := binary.Read(buf, nm.nativeEndian, &length)
	if err != nil {
		nm.logger.Error.Printf("%v: unable to read bytes representing message length: %v", nm.name, err)
	}
	return int(length)
}

// Parses incoming message from input.
func (nm *NativeMessagingReader[I]) handleMessage(msg []byte) {
	incomingMsg := nm.decodeMessage(msg)
	nm.logger.Trace.Printf("%v: message received: %s", nm.name, msg)
	if nm.messageHandler != nil {
		nm.messageHandler(incomingMsg)
	}
}

// Unmarshals incoming json request and returns query value.
func (nm *NativeMessagingReader[I]) decodeMessage(msg []byte) I {
	var incomingMsg I
	err := json.Unmarshal(msg, &incomingMsg)
	if err != nil {
		nm.logger.Error.Printf("%v: unable to unmarshal json to struct: %v", nm.name, err)
	}
	return incomingMsg
}
