package native_messaging

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"example/remote/internal/logger"
	"io"
	"unsafe"
)

type Responder[I any] interface {
	HandleMessage(msg I)
}

type NativeMessaging[I any, O any] struct {
	logger *logger.Logger

	inputHandle io.Reader

	outputHandle io.Writer

	// bufferSize used to set size of IO buffer - adjust to accommodate message payloads
	bufferSize int

	// nativeEndian used to detect native byte order
	nativeEndian binary.ByteOrder
}

func NewNativeMessaging[I any, O any](logger *logger.Logger, inputHandle io.Reader, outputHandle io.Writer) NativeMessaging[I, O] {
	return NativeMessaging[I, O]{
		logger:       logger,
		inputHandle:  inputHandle,
		outputHandle: outputHandle,
		bufferSize:   8192,
		nativeEndian: determineByteOrder(),
	}
}

func determineByteOrder() binary.ByteOrder {
	// determine native byte order so that we can read message size correctly
	var one int16 = 1
	b := (*byte)(unsafe.Pointer(&one))
	if *b == 0 {
		return binary.BigEndian
	} else {
		return binary.LittleEndian
	}
}

// ReadMessagesFromBrowser creates a new buffered I/O reader and reads messages from inputFile.
func (nm *NativeMessaging[I, O]) ReadMessagesFromBrowser(responder Responder[I]) {
	nm.logger.Trace.Printf("Chrome native messaging host started. Native byte order: %v.", nm.nativeEndian)

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
		nm.handleMessageFromBrowser(content, responder)
	}

	nm.logger.Trace.Print("Stdin closed.")
	nm.logger.Trace.Print("Chrome native messaging host exited.")
}

// readMessageLength reads and returns the message length value in native byte order.
func (nm *NativeMessaging[I, O]) readMessageLength(msg []byte) int {
	var length uint32
	buf := bytes.NewBuffer(msg)
	err := binary.Read(buf, nm.nativeEndian, &length)
	if err != nil {
		nm.logger.Error.Printf("Unable to read bytes representing message length: %v", err)
	}
	return int(length)
}

// handleMessageFromBrowser parses incoming message from browser
func (nm *NativeMessaging[I, O]) handleMessageFromBrowser(msg []byte, responder Responder[I]) {
	incomingMsg := nm.decodeMessageFromBrowser(msg)
	nm.logger.Trace.Printf("Message received from browser: %s", msg)
	responder.HandleMessage(incomingMsg)
}

// decodeMessageFromBrowser unmarshals incoming json request and returns query value.
func (nm *NativeMessaging[I, O]) decodeMessageFromBrowser(msg []byte) I {
	var incomingMsg I
	err := json.Unmarshal(msg, &incomingMsg)
	if err != nil {
		nm.logger.Error.Printf("Unable to unmarshal json to struct: %v", err)
	}
	return incomingMsg
}

// sendToBrowser sends an outgoing message to outputFile.
func (nm *NativeMessaging[I, O]) SendToBrowser(msg O) {
	byteMsg := nm.dataToBytes(msg)
	nm.writeMessageLength(byteMsg)

	var msgBuf bytes.Buffer
	_, err := msgBuf.Write(byteMsg)
	if err != nil {
		nm.logger.Error.Printf("Unable to write message length to message buffer: %v", err)
	}

	_, err = msgBuf.WriteTo(nm.outputHandle)
	if err != nil {
		nm.logger.Error.Printf("Unable to write message buffer to Stdout: %v", err)
	}

	nm.logger.Trace.Printf("Message sent to browser: %s", byteMsg)
}

// dataToBytes marshals an outcoming message struct to slice of bytes
func (nm *NativeMessaging[I, O]) dataToBytes(msg O) []byte {
	byteMsg, err := json.Marshal(msg)
	if err != nil {
		nm.logger.Error.Printf("Unable to marshal outgoing message struct to slice of bytes: %v", err)
	}
	return byteMsg
}

// writeMessageLength determines length of message and writes it to outputFile.
func (nm *NativeMessaging[I, O]) writeMessageLength(msg []byte) {
	err := binary.Write(nm.outputHandle, nm.nativeEndian, uint32(len(msg)))
	if err != nil {
		nm.logger.Error.Printf("Unable to write message length to Stdout: %v", err)
	}
}
