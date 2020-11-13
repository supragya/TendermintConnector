import socket
import sys

# Create a TCP/IP socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock2 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

server_address_tcp_bridge = ('localhost', 15004)
sock.bind(server_address_tcp_bridge)
sock.listen(2)

while True:
    # Wait for a connection
    print('connwait TCP Bridge')
    connection, client_address = sock.accept()
    print('connwait TCP Bridge')
    connection, client_address = sock.accept()
    try:

        # Receive the data in small chunks and retransmit it
        while True:
            data = connection.recv(200)
            print(str(len(data)), end="\t", flush=True)
            if data:
                connection.sendall(data)
                # connection2.sendall(data)
            else:
                break
            
    finally:
        # Clean up the connection
        connection.close()