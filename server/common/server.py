import socket
import logging
import signal
from common.utils import Bet
from common.utils import store_bets

MAX_LEN = 8192
MAX_BYTES_MSG = 2
RESPONSE_BYTES = 2
RESPONSE = 1
CONTINUE = 1
END = 0


class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._shutdown = False
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._server_socket.bind(('', port))
        self._server_socket.listen(listen_backlog)
        signal.signal(signal.SIGTERM, self.__exit_gracefully)

    def __exit_gracefully(self, *args):
        self.__del__()
        logging.info('action: shutdown | result: success')

    def run(self):
        """
        Dummy Server loop

        Server that accept a new connections and establishes a
        communication with a client. After client with communucation
        finishes, servers starts to accept new connections again
        """

        # TODO: Modify this program to handle signal to graceful shutdown
        # the server
        while not self._shutdown:
            client_sock = self.__accept_new_connection()
            self.__handle_client_connection(client_sock)
            if client_sock is not None:
                addr = client_sock.getpeername()
                client_sock.shutdown(socket.SHUT_RDWR)
                client_sock.close()
                logging.info(f'action: apuestas_almacenada | result: success | client: {addr[0]}')

    def __recv(self, client_sock):
        msg = b''
        bytes_recv = 0
        try:
            while not bytes_recv >= MAX_BYTES_MSG:
                msg += client_sock.recv(MAX_LEN)
                bytes_recv += len(msg)

            size = int.from_bytes(msg[0:MAX_BYTES_MSG], 'big')
            bytes_recv -= MAX_BYTES_MSG
            while not bytes_recv >= size:
                msg += client_sock.recv(size)
                bytes_recv += len(msg)

            self._continue_conn = bool(int.from_bytes(msg[-2:], 'big'))

        except OSError as e:
            logging.error(f"action: receive_message | result: fail | error: {e}")
            client_sock.close()

        return msg[2:-2]

    def __send(self, client_sock, bytes_msg):
        try:
            sent = 0
            while not sent >= RESPONSE_BYTES:
                sent += client_sock.send(bytes_msg)

        except OSError as e:
            client_sock.close()
            logging.error(f"action: send_message | result: fail | error: {e}")
        return

    def __decode(self, bytes_msg):
        content = bytes_msg.decode('UTF-8')
        info = content.split(';')
        agency = int(info[0])
        i = 1
        bets = []
        while i < len(info):
            bet = Bet(
                agency,
                info[i],
                info[i+1],
                info[i+2],
                info[i+3],
                info[i+4]
            )
            bets.append(bet)
            i = i + 5

        return bets

    def __encode(self, msg):
        bytes_msg = msg.to_bytes(RESPONSE_BYTES, byteorder='big')
        return bytes_msg

    def __handle_client_connection(self, client_sock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        while self._continue_conn:
            if self._shutdown:
                return

            bytes_msg = self.__recv(client_sock)
            if self._continue_conn:
                bets = self.__decode(bytes_msg)
                for bet in bets:
                    store_bets([bet])

            response = self.__encode(RESPONSE)
            self.__send(client_sock, response)

    def __accept_new_connection(self):
        """
        Accept new connections

        Function blocks until a connection to a client is made.
        Then connection created is printed and returned
        """

        # Connection arrived
        logging.info('action: accept_connections | result: in_progress')
        try:
            c, addr = self._server_socket.accept()
        except OSError:
            return

        logging.info(f'action: accept_connections | result: success | ip: {addr[0]}')
        self._continue_conn = True
        return c

    def __del__(self):
        self._shutdown = True
        self._server_socket.close()
