#!/bin/bash

echo CA subject?
read -r CASub

echo Server subject?
read -r ServerSub

echo Client subject?
read -r ClientSub

# Generate CA
openssl genrsa -out ca.key 2048
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt -subj "/CN=$CASub"

# Generate Server Cert
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr -subj "/CN=$ServerSub"

# Create the extension file on the fly
echo "subjectAltName = DNS:localhost, IP:127.0.0.1" >server-ext.cnf
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out server.crt -days 365 -sha256 -extfile server-ext.cnf

# Generate Client Cert
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr -subj "/CN=$ClientSub"
openssl x509 -req -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -out client.crt -days 365 -sha256

echo "Certificates generated successfully!"
