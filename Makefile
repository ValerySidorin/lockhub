run:
	go run cmd/main.go \
	-addr :13796 \
	-tls.server-cert-path ./certs/lockhub.io.pem \
	-tls.server-key-path ./certs/lockhub.io-key.pem \
	-raft.dir ./raft \
	-raft.bind-addr 127.0.0.1:9867 \
	-raft.node-id node1 \
	-log.level DEBUG