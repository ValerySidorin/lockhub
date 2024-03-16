run:
	go run cmd/main.go \
	-addr :13796 \
	-tls.server-cert-path ./sandbox/certs/lockhub.io.pem \
	-tls.server-key-path ./sandbox/certs/lockhub.io-key.pem \
	-tls.next-protos lockhub \
	-raft.dir ./sandbox/raft \
	-raft.bind-addr 127.0.0.1:9867 \
	-raft.node-id node1 \
	-log.level DEBUG