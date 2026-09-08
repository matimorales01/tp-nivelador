import socket


def recv_all(socket: socket.socket, size):
    parts = []
    total_received = 0
    while total_received < size:
        chunk = socket.recv(size - total_received)
        if not chunk:
            raise ConnectionError("conexión cerrada")
        parts.append(chunk)
        total_received += len(chunk)
    return b"".join(parts)


def send_all(socket: socket.socket, bytes):
    total_sent = 0
    while total_sent < len(bytes):
        sent = socket.send(bytes[total_sent:])
        total_sent += sent
