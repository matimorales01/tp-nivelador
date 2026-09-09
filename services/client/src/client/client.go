package client

import (
	"bufio"
	"errors"
	"net"
	"os"
	"strings"
	"time"

	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/logger"
	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/protocol"
)

const CONNECTION_ATTEMPTS_MAX = 15
const CONNECTION_ATTEMPS_DELAY_MS = 500

type ClientConfig struct {
	ServerHost string
	ServerPort string
	AgencyId   string
	InputFile  string
	OutputFile string
	BatchSize  int
}

type Client struct {
	conn   net.Conn
	config ClientConfig
}

func NewClient(config ClientConfig) (*Client, error) {
	conn, err := connectToServer(config.ServerHost, config.ServerPort)
	if err != nil {
		logger.Warn("connect-to-server", logger.Fail)
		return nil, err
	}

	client := &Client{conn: conn, config: config}
	return client, nil
}

func connectToServer(host, port string) (net.Conn, error) {
	const action = "connect-to-server"
	var err error
	var conn net.Conn

	logger.Info(action, logger.InProgress)
	for i := range CONNECTION_ATTEMPTS_MAX {
		conn, err = net.Dial("tcp", host+":"+port)
		if err != nil {
			logger.Warn(action, logger.Fail, "attempt", i)
			time.Sleep(CONNECTION_ATTEMPS_DELAY_MS * time.Millisecond)
			continue
		}

		logger.Info(action, logger.Success)
		break
	}

	return conn, err
}

func (client *Client) Run() error {
	defer client.conn.Close()

	inputFile, err := os.Open(client.config.InputFile)
	if err != nil {
		logger.Error("open-input-file", logger.Fail, "err", err)
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(client.config.OutputFile)
	if err != nil {
		logger.Error("open-output-file", logger.Fail, "err", err)
		return err
	}
	defer outputFile.Close()

	const action = "process-bet"
	messageId := 0
	lines := make([]string, 0)
	scanner := bufio.NewScanner(inputFile)
	batch := make([]string, 0, client.config.BatchSize)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)

		payload := protocol.SerializeBet(client.config.AgencyId, line)
		batch = append(batch, string(payload))

		if len(batch) == client.config.BatchSize {
			messageArgs := []any{"agency-id", client.config.AgencyId, "message-id", messageId}
			logger.Info(action, logger.InProgress, messageArgs...)

			batchPayload := []byte(strings.Join(batch, "\n"))
			if err := protocol.SendMessage(client.conn, protocol.MsgBet, batchPayload); err != nil {
				logger.Error("send-message", logger.Fail, messageArgs...)
				return err
			}

			messageId++
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		batchPayload := []byte(strings.Join(batch, "\n"))
		if err := protocol.SendMessage(client.conn, protocol.MsgBet, batchPayload); err != nil {
			logger.Error("send-message", logger.Fail, "agency-id", client.config.AgencyId)
			return err
		}
		messageId++
	}
	if err := scanner.Err(); err != nil {
		logger.Error(action, logger.Fail, "err", err)
		return err
	}

	if err := protocol.SendMessage(client.conn, protocol.MsgFinish, []byte{}); err != nil {
		logger.Error("send-finish", logger.Fail, "agency-id", client.config.AgencyId)
		return err
	}

	msgType, responsePayload, err := protocol.ReadMessage(client.conn)
	if err != nil {
		logger.Error("recv-winners", logger.Fail, "agency-id", client.config.AgencyId)
		return err
	}
	if msgType != protocol.MsgWinners {
		logger.Error("recv-winners", logger.Fail, "agency-id", client.config.AgencyId, "msg-type", msgType)
		return errors.New("unexpected message type from server")
	}

	winnerDocs := protocol.ParseWinners(string(responsePayload))
	winnerSet := make(map[string]struct{})
	for _, doc := range winnerDocs {
		winnerSet[doc] = struct{}{}
	}

	for _, line := range lines {
		doc := protocol.DocumentFromLine(line)
		if _, isWinner := winnerSet[doc]; !isWinner {
			continue
		}
		if _, err := outputFile.WriteString(line + "\n"); err != nil {
			logger.Error("write-output", logger.Fail, "agency-id", client.config.AgencyId)
			return err
		}
	}

	logger.Info(action, logger.Success, "agency-id", client.config.AgencyId, "messages-amount", messageId, "winners-amount", len(winnerDocs))
	return nil
}
