package native_messaging

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"example/remote/internal/logger"
	"example/remote/internal/shared"
	"io"
)

type NativeMessagingWriter[O any] struct {
	logger *logger.Logger

	outputHandle io.Writer

	sends chan O

	// nativeEndian used to detect native byte order
	nativeEndian binary.ByteOrder
}

func NewNativeMessagingWriter[O any](logger *logger.Logger, outputHandle io.Writer) NativeMessagingWriter[O] {
	return NativeMessagingWriter[O]{
		logger:       logger,
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

// SendMessage queues an outgoing message to be sent to outputFile.
func (nm *NativeMessagingWriter[O]) SendMessage(msg O) {
	nm.sends <- msg
}

// sendMessageNow sends an outgoing message to outputFile.
func (nm *NativeMessagingWriter[O]) sendMessageNow(msg O) {
	byteMsg := nm.dataToBytes(msg)
	nm.writeMessageLength(byteMsg)

	var msgBuf bytes.Buffer
	_, err := msgBuf.Write(byteMsg)
	if err != nil {
		nm.logger.Error.Printf("Unable to write message length to message buffer: %v", err)
	}

	_, err = msgBuf.WriteTo(nm.outputHandle)
	if err != nil {
		nm.logger.Error.Printf("Unable to write message buffer: %v", err)
	}

	nm.logger.Trace.Printf("Message sent: %s", byteMsg)
}

// dataToBytes marshals an outcoming message struct to slice of bytes
func (nm *NativeMessagingWriter[O]) dataToBytes(msg O) []byte {
	byteMsg, err := json.Marshal(msg)
	if err != nil {
		nm.logger.Error.Printf("Unable to marshal outgoing message struct to slice of bytes: %v", err)
	}
	return byteMsg
}

// writeMessageLength determines length of message and writes it to outputFile.
func (nm *NativeMessagingWriter[O]) writeMessageLength(msg []byte) {
	err := binary.Write(nm.outputHandle, nm.nativeEndian, uint32(len(msg)))
	if err != nil {
		nm.logger.Error.Printf("Unable to write message length: %v", err)
	}
}
