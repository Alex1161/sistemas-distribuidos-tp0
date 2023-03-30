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
	DNI 		uint
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

// Build message in binary with format <SIZE>;<NAME>;<LASTNAME>;<DNI>;<BIRTHDAY>;<NUMBER>
func (c *Client) encode(clientInfo ClientInfo) []byte {
	content := fmt.Sprintf(
		";%s;%s;%d;%s;%d", 
		clientInfo.Name, 
		clientInfo.Lastname,
		clientInfo.DNI,
		clientInfo.Birthday,
		clientInfo.Number,
	)
	size := len([]byte(content))

	size_bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(size_bytes[0:], uint16(size))
	
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, content)
	if err != nil {
		log.Fatalf("action: encoding | result: fail | client_id: %v | error: %v",
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

// func (c *Client) decode(msg []byte) string {
// 	content := new(bytes.Buffer)
//     binary.Read(bytes.NewBuffer(msg[2:]), binary.LittleEndian, &content)
// 	response := content.String()

// 	log.Infof("action: decoding | result: success | client_id: %v | msg: %v -> %v",
// 		c.config.ID,
// 		msg,
// 		response,
// 	)

// 	return response
// }

func (c *Client) decode(msg []byte) uint16 {
	response := binary.LittleEndian.Uint16(msg[:])

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
	bytes_sent := 0

	for n:=0; n < len(msg);  {
		bytes_sent, err := c.conn.Write(
			msg[n:min(MAX_LEN - 1, len(msg) - n - 1)],
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

	log.Infof("action: send_bytes | result: success | client_id: %v | msg: %v bytes sent",
		c.config.ID,
		bytes_sent,
	)
	return nil
}

// func (c *Client) recv_bytes() []byte {
// 	MAX_LEN := 8192
// 	msg := make([]byte, MAX_LEN)
// 	bytes_recv := 0

// 	for ; bytes_recv < 2; {
// 		bytes_recv, err := c.conn.Read(msg)
// 		if err != nil {
// 			c.conn.Close()
// 			log.Fatalf("action: recv_bytes | result: fail | client_id: %v | error: %v",
// 				c.config.ID,
// 				err,
// 			)
// 		}
// 	}

//     size := binary.BigEndian.Uint16(msg[0:2])
// 	content := make([]byte, size)
// 	aux := 0
// 	for ; bytes_recv < int(size) + 2; bytes_recv += aux {
// 		aux, err := c.conn.Read(msg)
// 		if err != nil {
// 			c.conn.Close()
// 			log.Fatalf("action: recv_bytes | result: fail | client_id: %v | error: %v",
// 				c.config.ID,
// 				err,
// 			)
// 		}

// 		content = append(content, msg[:aux]...)
// 	}

// 	log.Infof("action: send_bytes | result: success | client_id: %v | msg: %v bytes received",
// 		c.config.ID,
// 		bytes_recv,
// 	)

// 	return msg
// }

func (c *Client) recv_bytes() []byte {
	RESPONSE_SIZE := 2
	msg := make([]byte, RESPONSE_SIZE)
	response := make([]byte, RESPONSE_SIZE)
	bytes_recv := 0
	

	for aux := 0; bytes_recv < RESPONSE_SIZE; bytes_recv += aux {
		aux, err := c.conn.Read(msg)
		if err != nil {
			c.conn.Close()
			log.Fatalf("action: recv_bytes | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
		}

		response = append(response, msg[:aux]...)
	}

	return msg
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop() {
	c.createClientSocket()

	// Catchig sigterm to shutdown gracefully
	select {
	case <-c.shutdown:
		c.conn.Close()
		
		log.Infof("action: shutdown | result: success | client_id: %v",
			c.config.ID,
		)
		return
	default:
	}

	bytes_to_send := c.encode(c.clientInfo)
	c.send_bytes(bytes_to_send)
	
	bytes_recv := c.recv_bytes()
	response_code := c.decode(bytes_recv)
	
	if response_code == 1 {
		log.Infof("action: apuesta_enviada | result: success | client_id: %v | dni: %v | numero: %v",
			c.config.ID,
			c.clientInfo.DNI,
			c.clientInfo.Number,
		)
	} else {
		log.Errorf("action: apuesta_enviada | result: fail | client_id: %v | error: %v",
			c.config.ID,
			response_code,
		)
	}

	c.conn.Close()
}
