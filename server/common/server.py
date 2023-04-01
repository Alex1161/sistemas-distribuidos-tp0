import socket
import logging
import signal
from multiprocessing import Process, Value, Array, Lock, Manager
from common.utils import Bet
from common.utils import store_bets
from common.utils import load_bets
from common.utils import has_won

MAX_LEN = 8192
MAX_BYTES_MSG = 2
RESPONSE_BYTES = 2
RESPONSE = 1
CONTINUE = 1
END = 0
RECV_BETS = 1
SEND_WINNERS = 2
MAX_CONNECTIONS = 5


class Connection:
    def __init__(self, conn):
        self.conn = conn
        self.agency = None
        self.state = RECV_BETS

class Server:
    def __init__(self, port, listen_backlog):
        # Initialize server socket
        self._shutdown = False
        self._server_socket = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._process = []
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
        with Manager() as manager:
            connections = manager.list()
            database_lock = Lock()
            while not self._shutdown and len(self._process) < MAX_CONNECTIONS:
                conn = self.__accept_new_connection()
                p = Process(
                    target=self.__handle_client_connection,
                    args=(conn, connections, database_lock)
                )
                p.start()
                self._process.append(p)

            for p in self._process:
                p.join()

            self._process = []
            logging.info(f'action: sorteo | result: success')
            database_lock.acquire()
            all_bets = load_bets()
            database_lock.release()
            winners = list(filter(lambda b: has_won(b), all_bets))

            while not self._shutdown and len(self._process) < MAX_CONNECTIONS:
                conn = connections.pop(0)
                p = Process(
                    target=self.__handle_client_get_winners,
                    args=(conn, winners,)
                )
                p.start()
                self._process.append(p)

            for p in self._process:
                p.join()

        logging.info(f'action: server_done | result: success')

    def __recv(self, connection):
        msg = b''
        bytes_recv = 0
        try:
            while not bytes_recv >= MAX_BYTES_MSG:
                msg += connection.conn.recv(MAX_LEN)
                bytes_recv += len(msg)

            size = int.from_bytes(msg[0:MAX_BYTES_MSG], 'big')
            bytes_recv -= MAX_BYTES_MSG
            while not bytes_recv >= size:
                msg += connection.conn.recv(size)
                bytes_recv += len(msg)

            if bool(int.from_bytes(msg[-2:], 'big')) == CONTINUE:
                connection.state = RECV_BETS
            else:
                connection.state = SEND_WINNERS

        except OSError as e:
            logging.error(f"action: receive_message | result: fail | error: {e}")
            connection.conn.close()

        return msg[2:-2]

    def __send(self, connection, bytes_msg, size):
        try:
            sent = 0
            while not sent >= size:
                sent += connection.conn.send(bytes_msg)

        except OSError as e:
            connection.conn.close()
            logging.error(f"action: send_message | result: fail | error: {e}")
        return

    def __recv_agency(self, connection):
        msg = b''
        bytes_recv = 0
        try:
            while not bytes_recv >= MAX_BYTES_MSG:
                msg += connection.conn.recv(MAX_BYTES_MSG)
                bytes_recv += len(msg)

            agency = int.from_bytes(msg[0:MAX_BYTES_MSG], 'big')

        except OSError as e:
            logging.error(f"action: receive_message | result: fail | error: {e}")
            connection.conn.close()

        logging.info(f'action: request_winner_recv | result: in:progress | client_id: {agency}')
        return agency

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

        return agency, bets

    def __encode(self, msg):
        bytes_msg = msg.to_bytes(RESPONSE_BYTES, byteorder='big')
        return bytes_msg

    def __handle_client_connection(self, connection, connections, database_lock):
        """
        Read message from a specific client socket and closes the socket

        If a problem arises in the communication with the client, the
        client socket will also be closed
        """
        while connection.state == RECV_BETS:
            if self._shutdown:
                return

            bytes_msg = self.__recv(connection)
            if connection.state == RECV_BETS:
                connection.agency, bets = self.__decode(bytes_msg)
                database_lock.acquire()
                store_bets(bets)
                database_lock.release()
            else:
                connections.append(connection)

            response = self.__encode(RESPONSE)
            self.__send(connection, response, RESPONSE_BYTES)

        logging.info(f'action: apuestas_almacenada | result: success | client_id: {connection.agency}')

    def __handle_client_get_winners(self, connection, winners):
        if self._shutdown:
            return

        agency = self.__recv_agency(connection)
        winners_agency = filter(lambda w: w.agency == agency, winners)
        winners_document = list(map(lambda x: x.document, winners_agency))

        content = ";".join(winners_document)
        size = len(content)
        if size == 0:
            content = ";"
        else:
            content = str(size) + ";" + content

        winners_to_send = bytes(content, 'utf-8')
        self.__send(connection, winners_to_send, len(winners_to_send))
        logging.info(f'action: send_winners | result: success | client_id: {agency}')

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
        return Connection(c)

    def __del__(self):
        self._shutdown = True
        self._server_socket.close()
