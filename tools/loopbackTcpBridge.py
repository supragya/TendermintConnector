import socket
import sys

# Create a TCP/IP socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

server_address = ('localhost', 21906)
sock.bind(server_address)

sock.listen(1)

while True:
    # Wait for a connection
    print('waiting for a connection')
    connection, client_address = sock.accept()
    try:
        print('connection from', client_address)

        # Receive the data in small chunks and retransmit it
        while True:
            data = connection.recv(10)
            for i in range(len(data)):
                print(data[i], end=" ", flush=True)
            # if data:
            #     connection.sendall(data)
            # else:
            #     break
            
    finally:
        # Clean up the connection
        connection.close()
