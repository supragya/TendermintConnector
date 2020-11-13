import socket
import sys

# Create a TCP/IP socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock2 = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

server_address = ('localhost', 15000)
sock.bind(server_address)
sock2.connect('localhost', 59002)
sock.listen(1)

while True:
    # Wait for a connection
    print('waiting for a connection')
    connection, client_address = sock.accept()
    try:
        print('connection from', client_address)

        # Receive the data in small chunks and retransmit it
        while True:
            data = connection.recv(100)
            print(str(len(data)), end="\t", flush=True)
            if data:
                connection.sendall(data)
                sock2.sendall(data)
            else:
                break
            
    finally:
        # Clean up the connection
        connection.close()


# import socket

# s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
# s.bind(("127.0.0.1", 15003))
# s.listen()

# print("Started TCP test server")
# while True:
#     # now our endpoint knows about the OTHER endpoint.
#     clientsocket, address = s.accept()
#     print(f"Connection from {address} has been established.")
#     with clientsocket:
#         while True:
#             data = clientsocket.recv(1024)
#             if not data:
#                 continue
#             print("DATA in" + str(data))
#             clientsocket.sendall(data)