package client

import (
	"bufio"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/logger"
	"github.com/7574-sistemas-distribuidos/tp-nivelador/src/protocol"
)

const CONNECTION_ATTEMPTS_MAX = 15
const CONNECTION_ATTEMPS_DELAY_MS = 500

const OUTPUT_FILE_ATTEMPTS_MAX = 60
const OUTPUT_FILE_ATTEMPS_DELAY_MS = 1000

type ClientConfig struct {
	ServerHost string
	ServerPort string
	AgencyId   string
	InputFile  string
	OutputFile string
	BatchSize  int
}

type Client struct {
	conn         net.Conn
	config       ClientConfig
	shuttingDown bool
	mu           sync.Mutex
}

func NewClient(config ClientConfig) (*Client, error) {
	client := &Client{config: config}
	client.watchShutdown()

	conn, err := client.connectToServer(config.ServerHost, config.ServerPort)
	if err != nil {
		logger.Warn("connect-to-server", logger.Fail)
		return client, err
	}

	client.mu.Lock()
	client.conn = conn
	client.mu.Unlock()
	return client, nil
}

func (client *Client) watchShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM)
	go func() {
		<-sigChan
		client.mu.Lock()
		client.shuttingDown = true
		conn := client.conn
		client.mu.Unlock()
		logger.Info("shutdown", logger.InProgress, "signal", "SIGTERM")
		if conn != nil {
			conn.Close()
		}
	}()
}

func (client *Client) IsShuttingDown() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return client.shuttingDown
}

func (client *Client) connectToServer(host, port string) (net.Conn, error) {
	const action = "connect-to-server"
	var err error
	var conn net.Conn

	logger.Info(action, logger.InProgress)
	for i := range CONNECTION_ATTEMPTS_MAX {
		if client.IsShuttingDown() {
			return nil, errors.New("shutting down")
		}

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

func createOutputFile(path string) (*os.File, error) {
	const action = "open-output-file"
	var err error
	var file *os.File

	for i := range OUTPUT_FILE_ATTEMPTS_MAX {
		file, err = os.Create(path)
		if err == nil {
			return file, nil
		}
		logger.Warn(action, logger.Fail, "attempt", i)
		time.Sleep(OUTPUT_FILE_ATTEMPS_DELAY_MS * time.Millisecond)
	}

	return nil, err
}

func (client *Client) Run() error {
	defer client.conn.Close()

	inputFile, err := os.Open(client.config.InputFile)
	if err != nil {
		logger.Error("open-input-file", logger.Fail, "err", err)
		return err
	}
	defer inputFile.Close()

	const action = "process-bet"
	messageId := 0
	scanner := bufio.NewScanner(inputFile)
	batch := make([]string, 0, client.config.BatchSize)
	for scanner.Scan() {
		line := scanner.Text()

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

			if messageId%25 == 0 {
				debug.FreeOSMemory()
			}
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

	outputFile, err := createOutputFile(client.config.OutputFile)
	if err != nil {
		logger.Error("open-output-file", logger.Fail, "err", err)
		return err
	}
	defer outputFile.Close()

	if _, err := inputFile.Seek(0, io.SeekStart); err != nil {
		logger.Error("seek-input-file", logger.Fail, "err", err)
		return err
	}
	winnersScanner := bufio.NewScanner(inputFile)
	for winnersScanner.Scan() {
		line := winnersScanner.Text()
		doc := protocol.DocumentFromLine(line)
		if _, isWinner := winnerSet[doc]; !isWinner {
			continue
		}
		if _, err := outputFile.WriteString(line + "\n"); err != nil {
			logger.Error("write-output", logger.Fail, "agency-id", client.config.AgencyId)
			return err
		}
	}
	if err := winnersScanner.Err(); err != nil {
		logger.Error(action, logger.Fail, "err", err)
		return err
	}

	logger.Info(action, logger.Success, "agency-id", client.config.AgencyId, "messages-amount", messageId, "winners-amount", len(winnerDocs))
	return nil
}
