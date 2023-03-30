package common

import (
	"fmt"
	"net"
	"time"
	"encoding/binary"
	"bytes"
	"math"
	"os"
    "os/signal"
    "syscall"

	log "github.com/sirupsen/logrus"
)

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopLapse     time.Duration
	LoopPeriod    time.Duration
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
	clientInfo ClientInfo
	shutdown chan os.Signal 
}

// ClientInfo Entity
type ClientInfo struct {
	Name		string
	Lastname   	string
	Document 	uint
	Birthday 	string
	Number		uint
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig, clientInfo ClientInfo) *Client {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	client := &Client{
		config: config,
		shutdown: sigs,
		clientInfo: clientInfo,
	}
	return client
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Fatalf(
	        "action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
	}
	c.conn = conn
	return nil
}

func (c *Client) encode() []byte {
	content := fmt.Sprintf(
		"%s;%s;%s;%d;%s;%d", 
		c.config.ID,
		c.clientInfo.Name, 
		c.clientInfo.Lastname,
		c.clientInfo.Document,
		c.clientInfo.Birthday,
		c.clientInfo.Number,
	)
	content_bytes := []byte(content)
	size := len(content_bytes)

	size_bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(size_bytes[0:], uint16(size))
	
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, content_bytes)
	if err != nil {
		log.Fatalf("action: encode | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
	}

	msg_to_send := bytes.Join(
		[][]byte{size_bytes, buf.Bytes()}, 
		[]byte(""),
	)

	log.Infof("action: encoding | result: success | client_id: %v | msg: %v -> %v",
		c.config.ID,
		content,
		msg_to_send,
	)

	return msg_to_send
}

func (c *Client) decode(msg []byte) uint16 {
	response := binary.BigEndian.Uint16(msg[:])

	log.Infof("action: decoding | result: success | client_id: %v | msg: %v -> %v",
		c.config.ID,
		msg,
		response,
	)

	return response
}

func min(a, b int) int {
	r := math.Min(float64(a), float64(b))
	return int(r)
}

func (c *Client) send_bytes(msg []byte) error {
	MAX_LEN := 8192 // put in config

	for n:=0; n < len(msg);  {
		bytes_sent, err := c.conn.Write(
			msg[n:min(MAX_LEN, len(msg) - n)],
		)
		if err != nil {
			c.conn.Close()
			log.Fatalf("action: send_bytes | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
		}
		
		n += bytes_sent
	}

	log.Infof("action: send_bytes | result: success | client_id: %v | msg: %v sent",
		c.config.ID,
		msg,
	)
	return nil
}

func (c *Client) recv_bytes() []byte {
	RESPONSE_SIZE := 2
	// msg := make([]byte, RESPONSE_SIZE)
	response := make([]byte, RESPONSE_SIZE)
	// bytes_recv := 0

	c.conn.Read(response)

	// for aux := 0; bytes_recv < RESPONSE_SIZE; bytes_recv += aux {
	// 	aux, err := c.conn.Read(msg)
	// 	if err != nil {
	// 		c.conn.Close()
	// 		log.Fatalf("action: recv_bytes | result: fail | client_id: %v | error: %v",
	// 			c.config.ID,
	// 			err,
	// 		)
	// 	}

	// 	response = append(response, msg[:aux]...)
	// }

	log.Infof("action: recv_bytes | result: success | client_id: %v | msg: %v received",
		c.config.ID,
		response,
	)

	return response
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop() {
	c.createClientSocket()

	// Catchig sigterm to shutdown gracefully
	// select {
	// case <-c.shutdown:
	// 	c.conn.Close()
		
	// 	log.Infof("action: shutdown | result: success | client_id: %v",
	// 		c.config.ID,
	// 	)
	// 	return
	// default:
	// }

	bytes_to_send := c.encode()
	c.send_bytes(bytes_to_send)
	
	bytes_recv := c.recv_bytes()
	response_code := c.decode(bytes_recv)
	c.conn.Close()

	if response_code == 1 {
		log.Infof("action: apuesta_enviada | result: success | client_id: %v | dni: %v | numero: %v",
			c.config.ID,
			c.clientInfo.Document,
			c.clientInfo.Number,
		)
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | client_id: %v | error: %v",
			c.config.ID,
			response_code,
		)
	}
}
