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

func SendMessage(conn io.Writer, msgType byte, payload []byte) error {
	message := make([]byte, 5+len(payload))
	message[0] = msgType
	binary.BigEndian.PutUint32(message[1:5], uint32(len(payload)))
	copy(message[5:], payload)

	return safe_socket.SendAll(conn, message)
}

func ReadMessage(conn io.Reader) (byte, []byte, error) {
	header, err := safe_socket.RecvAll(conn, 5)
	if err != nil {
		return 0, nil, err
	}

	msgType := header[0]
	length := binary.BigEndian.Uint32(header[1:])

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

func DocumentFromLine(line string) string {
	fields := strings.Split(line, ",")
	return fields[2]
}

