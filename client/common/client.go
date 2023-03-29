package common

import (
	"bufio"
	"fmt"
	"net"
	"time"
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

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop() {
	// autoincremental msgID to identify every message sent
	msgID := 1

	// Build message
	// Format 	<NAME>;<LASTNAME>;<DNI>;<BIRTHDAY>;<NUMBER>/
	content := fmt.Sprintf(
		"%s;%s;%d;%s;%d/", 
		c.clientInfo.Name, 
		c.clientInfo.Lastname,
		c.clientInfo.DNI,
		c.clientInfo.Birthday,
		c.clientInfo.Number,
	)

	c.createClientSocket()

loop:
	// Send messages if the loopLapse threshold has not been surpassed
	for timeout := time.NewTimer(c.config.LoopLapse); ; {
		select {
		case <-timeout.C:
	        log.Infof("action: timeout_detected | result: success | client_id: %v",
                c.config.ID,
            )
			break loop
		case <-c.shutdown:
			c.conn.Close()
			// Stop the timer from timeout to not leak goroutines
			if !timeout.Stop() {
				<-timeout.C
			} 
			
			log.Infof("action: shutdown | result: success | client_id: %v",
                c.config.ID,
            )
			return
		default:
		}

		// TODO: Modify the send to avoid short-write
		fmt.Fprintf(
			c.conn,
			"[CLIENT %v] Message N°%v\n",
			c.config.ID,
			msgID,
		)
		msg, err := bufio.NewReader(c.conn).ReadString('\n')
		msgID++

		if err != nil {
			log.Errorf("action: receive_message | result: fail | client_id: %v | error: %v",
                c.config.ID,
				err,
			)
			c.conn.Close()
			return
		}
		log.Infof("action: receive_message | result: success | client_id: %v | msg: %v",
            c.config.ID,
            msg,
        )

		// Wait a time between sending one message and the next one
		time.Sleep(c.config.LoopPeriod)
	}

	c.conn.Close()
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}
