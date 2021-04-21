import socket
import sys

# Create a TCP/IP socket
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
gateway_side_address = ('localhost', 60000)
sock.bind(gateway_side_address)
sock.listen(1)

socksf = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sf_side_address = ('localhost', 60001)

while True:
    print('Establishing connection with spamfilter at ', sf_side_address)
    socksf.connect(sf_side_address)
    print('Connection with SF established')
    # Wait for a connection
    print('Waiting for a connection from gateway')
    conn_gw, client_address = sock.accept()
    try:
        print('Connection from gateway established. gateway address: ', client_address)
        print('------------------------------------------------------------------------')
        # Receive the data in small chunks and retransmit it
        while True:
            data = conn_gw.recv(100)
            # for i in range(len(data)):
            #     print(data[i], end=" ", flush=True)
            print('.', end="", flush=True)
            socksf.sendall(data)
            while True:
                try:
                    resp = socksf.recv(1)
                    if resp != b'':
                        print(resp)
                    else:
                        break
                except:
                    break
            # if data:
            #     connection.sendall(data)
            # else:
            #     break
            
    finally:
        # Clean up the connection
        conn_gw.close()
        conn_sf.close()

