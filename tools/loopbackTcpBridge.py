import socket
import sys

# Create a TCP/IP socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)

server_address = ('localhost', 21901)
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
            # for i in range(len(data)):
            #     print(data[i], end=" ", flush=True)
            if data:
                connection.sendall(data)
            else:
                break
            
    finally:
        # Clean up the connection
        connection.close()

# Updates:
# - MarlinCTL: updated for IRISHUB support
# - Iris Full node: shifted from GCP to AWS (syncing)
# - Cosmos Full node: created on AWS (syncing)
# - TendermintConnector: Basic interfacing and handshake with cosmoshub-3 node successful, waiting for node to catch up to check for message deliveries
# - Iris stakedrop: To start today
# - Build-Automation: needs a few changes to support keyfiles