import socket
import sys

# Create a TCP/IP socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

server_address = ('localhost', 15005)
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
            data = connection.recv(100)
            print("Recv: ", end="")
            for i in range(len(data)):
                print(data[i], end=" ")
            if data:
                connection.sendall(data)
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