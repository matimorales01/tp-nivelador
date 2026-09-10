import safe_socket
from lottery import Bet

MSG_BET = 1
MSG_FINISH = 2
MSG_WINNERS = 3

MSG_TYPE_SIZE = 1
MSG_LENGTH_SIZE = 4
HEADER_SIZE = MSG_TYPE_SIZE + MSG_LENGTH_SIZE


def send_message(sock, msg_type, payload):
    header = bytes([msg_type]) + len(payload).to_bytes(MSG_LENGTH_SIZE, "big")
    message = header + payload
    safe_socket.send_all(sock, message)


def recv_message(sock):
    header = safe_socket.recv_all(sock, HEADER_SIZE)
    msg_type = header[0]
    length = int.from_bytes(header[MSG_TYPE_SIZE:], "big")
    payload = safe_socket.recv_all(sock, length)
    return msg_type, payload


def parse_bets(payload):
    text = payload.decode("utf-8")
    bets = []
    for line in text.split("\n"):
        agency_id, first_name, last_name, document, birthdate, number = line.split(",")
        bets.append(Bet(int(agency_id), first_name, last_name, int(document), birthdate, int(number)))
    return bets


def encode_winners(bets):
    documents = [str(bet.document) for bet in bets]
    return ",".join(documents).encode("utf-8")
