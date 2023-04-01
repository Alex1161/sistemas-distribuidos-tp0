package common

import (
	"fmt"
	"net"
	"time"
	"encoding/binary"
	"bytes"
	"strconv"
	"strings"
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
	ChunkSize	  uint
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
	shutdown chan os.Signal 
	chunk	[]byte
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
func NewClient(config ClientConfig) *Client {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	client := &Client{
		config: config,
		shutdown: sigs,
		chunk: nil,
	}

	client.createClientSocket()

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

func (c *Client) add_header(content []byte) []byte {
	id := []byte(c.config.ID)
	content_bytes := bytes.Join(
		[][]byte{id, content}, 
		[]byte(";"),
	)
	size := len(content_bytes)
	size_bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(size_bytes[0:], uint16(size))
	msg_to_send := bytes.Join(
		[][]byte{size_bytes, content_bytes}, 
		[]byte(""),
	)

	return msg_to_send
}

func (c *Client) encode_content(clientInfo ClientInfo) []byte {
	content := fmt.Sprintf(
		"%s;%s;%d;%s;%d", 
		clientInfo.Name, 
		clientInfo.Lastname,
		clientInfo.Document,
		clientInfo.Birthday,
		clientInfo.Number,
	)
	content_bytes := []byte(content)
	
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, content_bytes)
	if err != nil {
		log.Fatalf("action: encode | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
	}

	return buf.Bytes()
}

func (c *Client) decode(msg []byte) uint16 {
	response := binary.BigEndian.Uint32(msg[:])
	return uint16(response)
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

	return nil
}

func (c *Client) recv_bytes() []byte {
	RESPONSE_SIZE := 2
	msg := make([]byte, RESPONSE_SIZE)
	response := make([]byte, RESPONSE_SIZE)
	bytes_recv := 0

	for ; bytes_recv < RESPONSE_SIZE; {
		aux, err := c.conn.Read(msg)
		if err != nil {
			c.conn.Close()
			log.Fatalf("action: recv_bytes | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
		}

		bytes_recv += aux
		response = append(response, msg[:aux]...)
	}

	return response
}

func (c *Client) add_continue_suffix(chunk []byte, continue_sending uint16) []byte {
	var chunk_to_send []byte
	continue_msg := make([]byte, 2)
	binary.BigEndian.PutUint16(continue_msg[0:], continue_sending)
	if chunk == nil {
		chunk_to_send = continue_msg
	} else {
		chunk_to_send = bytes.Join(
			[][]byte{chunk, continue_msg}, 
			[]byte(""),
		)
	}

	return chunk_to_send
}

func (c *Client) flush(continue_sending uint16) {
	chunk_to_send := c.add_continue_suffix(c.chunk, continue_sending)

	bytes_to_send := c.add_header(chunk_to_send)
	c.send_bytes(bytes_to_send)
	
	bytes_recv := c.recv_bytes()
	response_code := c.decode(bytes_recv)

	if response_code != 1 {
		c.conn.Close()
		log.Errorf("action: chunk_sent | result: fail | client_id: %v | error: %v",
			c.config.ID,
			response_code,
		)
	}

	c.chunk = nil
}

func (c *Client) add_client(info_encoded []byte) {
	if c.chunk == nil {
		c.chunk = info_encoded
	} else {
		c.chunk = bytes.Join (
			[][]byte{c.chunk, info_encoded}, 
			[]byte(";"),
		)
	}
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) Send_number(clientInfo ClientInfo) {
	CONTINUE := 1
	HEADER_SIZE := 7
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

	client_encoded := c.encode_content(clientInfo)
	if (uint(len(c.chunk) + len(client_encoded) + HEADER_SIZE) >= c.config.ChunkSize) {
		c.flush(uint16(CONTINUE))
	}

	c.add_client(client_encoded)
}

func (c *Client) EndConnection() {
	CONTINUE := 0
	c.flush(uint16(CONTINUE))
	log.Infof("action: apuestas_enviadas | result: success | client_id: %v",
		c.config.ID,
	)
	c.send_agency()
	winners := c.recv_winners()
	log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %v | client_id: %v",
		len(winners),
		c.config.ID,
	)
	c.conn.Close()
}

func contains(s []byte, str byte) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}

	return false
}

func (c *Client) recv_winners() []string {
	MAX_LEN := 8192
	msg := make([]byte, MAX_LEN)
	buf := make([]byte, MAX_LEN)
	bytes_recv := 0
	DELIMITER := ';'
	
	for ; !bytes.ContainsRune(msg, DELIMITER); {
		aux, err := c.conn.Read(buf)
		if err != nil {
			c.conn.Close()
			log.Fatalf("action: recv_bytes | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
		}

		bytes_recv += aux
		msg = append(msg, buf[:aux]...)
	}

	size_bytes := msg[0:bytes.IndexRune(msg, DELIMITER)] 
	size, _ := strconv.ParseUint(string(size_bytes), 10, 32)
	new_buff := make([]byte, size)
	for ; bytes_recv < int(size); {
		aux, err := c.conn.Read(new_buff)
		if err != nil {
			c.conn.Close()
			log.Fatalf("action: recv_bytes | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
		}
		log.Infof("asdfadf")

		bytes_recv += aux
		msg = append(msg, new_buff[:aux]...)
	}
	return strings.Split(string(msg[size:]), ";")
}

func (c *Client) send_agency() error {
	msg, _ := strconv.ParseUint(c.config.ID, 10, 16)
	msg_to_send := uint16(msg)
	msg_bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(msg_bytes[0:], msg_to_send)

	for n:=0; n < len(msg_bytes);  {
		bytes_sent, err := c.conn.Write(
			msg_bytes[n:len(msg_bytes[:])-n],
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

	return nil
}
