import socket
import signal
import threading
import logger
import protocol


class Server:
    def __init__(self, server_host: str, server_port: int, lottery, quorum_min: int) -> None:
        self.server_host = server_host
        self.server_port = server_port
        self.lottery = lottery
        self.quorum_min = quorum_min
        self.finished_agencies = set()
        self.condition = threading.Condition()
        self._shutdown_event = threading.Event()
        self._server_socket = None

    def _handle_sigterm(self, signum, frame):
        logger.info("shutdown", logger.LogResult.in_progress, "signal", "SIGTERM")
        self._shutdown_event.set()
        with self.condition:
            self.condition.notify_all()
        if self._server_socket:
            self._server_socket.close()

    def _handle_client(self, client_socket):
        action = "handle-client"
        message_amount = 0
        agency_id = None
        with client_socket:
            try:
                logger.info(action, logger.LogResult.in_progress)
                while True:
                    msg_type, payload = protocol.recv_message(client_socket)
                    message_amount += 1

                    if msg_type == protocol.MSG_BET:
                        bets = protocol.parse_bets(payload)
                        agency_id = bets[0].agency_id
                        with self.condition:
                            self.lottery.store_bets(bets)
                    elif msg_type == protocol.MSG_FINISH:
                        with self.condition:
                            self.finished_agencies.add(agency_id)
                            self.condition.notify_all()
                            while (
                                len(self.finished_agencies) < self.quorum_min
                                and not self._shutdown_event.is_set()
                            ):
                                self.condition.wait()

                            if self._shutdown_event.is_set():
                                return

                            winners = [
                                bet
                                for bet in self.lottery.load_bets()
                                if bet.agency_id == agency_id and self.lottery.has_won(bet)
                            ]

                        response = protocol.encode_winners(winners)
                        protocol.send_message(client_socket, protocol.MSG_WINNERS, response)
                        logger.info(
                            action,
                            logger.LogResult.success,
                            "messages-amount", message_amount,
                            "winners-amount", len(winners),
                        )
                        return
            except Exception as e:
                if self._shutdown_event.is_set():
                    logger.info(action, logger.LogResult.success, "reason", "sigterm")
                    return
                logger.error(action, logger.LogResult.fail, "messages-amount", message_amount)
                raise e

    def run(self):
        action = "accept-connection"
        signal.signal(signal.SIGTERM, self._handle_sigterm)
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as server_socket:
            self._server_socket = server_socket
            server_socket.bind((self.server_host, self.server_port))
            server_socket.listen()
            while True:
                if self._shutdown_event.is_set():
                    logger.info(action, logger.LogResult.success, "reason", "sigterm")
                    return
                try:
                    logger.info(action, logger.LogResult.in_progress)
                    client_socket, _ = server_socket.accept()
                except OSError:
                    if self._shutdown_event.is_set():
                        logger.info(action, logger.LogResult.success, "reason", "sigterm")
                        return
                    raise
                logger.info(action, logger.LogResult.success)

                thread = threading.Thread(target=self._handle_client, args=(client_socket,))
                thread.daemon = True
                thread.start()
