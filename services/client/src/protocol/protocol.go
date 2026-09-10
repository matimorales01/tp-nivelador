package protocol

import (
	"encoding/binary"
	"io"
	"strings"

	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/safe_socket"
)

const (
	MsgBet     byte = 1
	MsgFinish  byte = 2
	MsgWinners byte = 3
)

const (
	msgTypeSize   = 1
	msgLengthSize = 4
	headerSize    = msgTypeSize + msgLengthSize
)

func SendMessage(conn io.Writer, msgType byte, payload []byte) error {
	message := make([]byte, headerSize+len(payload))
	message[0] = msgType
	binary.BigEndian.PutUint32(message[msgTypeSize:headerSize], uint32(len(payload)))
	copy(message[headerSize:], payload)

	return safe_socket.SendAll(conn, message)
}

func ReadMessage(conn io.Reader) (byte, []byte, error) {
	header, err := safe_socket.RecvAll(conn, headerSize)
	if err != nil {
		return 0, nil, err
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[msgTypeSize:])

	payload, err := safe_socket.RecvAll(conn, int(length))
	if err != nil {
		return 0, nil, err
	}

	return msgType, payload, nil
}

func SerializeBet(agencyId, line string) []byte {
	return []byte(agencyId + "," + line)
}

func ParseWinners(payload string) []string {
	if payload == "" {
		return []string{}
	}
	return strings.Split(payload, ",")
}

const documentFieldIndex = 2 // nombre,apellido,documento,nacimiento,numero

func DocumentFromLine(line string) string {
	fields := strings.Split(line, ",")
	return fields[documentFieldIndex]
}

