package native_messaging

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"

	"github.com/jacobweber/browser_remote/internal/logger"
	"github.com/jacobweber/browser_remote/internal/shared"
)

type NativeMessagingWriter[O any] struct {
	logger *logger.Logger

	name string

	outputHandle io.Writer

	sends chan O

	nativeEndian binary.ByteOrder
}

func NewWriter[O any](logger *logger.Logger, outputHandle io.Writer, name string) *NativeMessagingWriter[O] {
	return &NativeMessagingWriter[O]{
		logger:       logger,
		name:         name,
		outputHandle: outputHandle,
		sends:        make(chan O),
		nativeEndian: shared.DetermineByteOrder(),
	}
}

func (nm *NativeMessagingWriter[O]) Start() {
	for msg := range nm.sends {
		nm.sendMessageNow(msg)
	}
}

func (nm *NativeMessagingWriter[O]) Done() {
	close(nm.sends)
}

// Queues an outgoing message to be sent to outputFile.
func (nm *NativeMessagingWriter[O]) SendMessage(msg O) {
	nm.sends <- msg
}

// Sends an outgoing message to outputFile.
func (nm *NativeMessagingWriter[O]) sendMessageNow(msg O) {
	byteMsg := nm.dataToBytes(msg)
	nm.writeMessageLength(byteMsg)

	var msgBuf bytes.Buffer
	_, err := msgBuf.Write(byteMsg)
	if err != nil {
		nm.logger.Error.Printf("%v: unable to write message length to message buffer: %v", nm.name, err)
	}

	_, err = msgBuf.WriteTo(nm.outputHandle)
	if err != nil {
		nm.logger.Error.Printf("%v: unable to write message buffer: %v", nm.name, err)
	}

	nm.logger.Trace.Printf("%v: message sent: %s", nm.name, byteMsg)
}

// Marshals an outcoming message struct to a slice of bytes.
func (nm *NativeMessagingWriter[O]) dataToBytes(msg O) []byte {
	byteMsg, err := json.Marshal(msg)
	if err != nil {
		nm.logger.Error.Printf("%v: unable to marshal outgoing message struct to slice of bytes: %v", nm.name, err)
	}
	return byteMsg
}

// Determines length of message and writes it to outputFile.
func (nm *NativeMessagingWriter[O]) writeMessageLength(msg []byte) {
	err := binary.Write(nm.outputHandle, nm.nativeEndian, uint32(len(msg)))
	if err != nil {
		nm.logger.Error.Printf("%v: unable to write message length: %v", nm.name, err)
	}
}
