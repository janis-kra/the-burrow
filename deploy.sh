#!/bin/sh

GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o burrow ./cmd/burrow
USER=jk
HOST=aragorn.tail40acc0.ts.net

ssh $USER@$HOST "sudo systemctl stop burrow"
scp burrow $USER@$HOST:~/burrow/
scp config.yaml $USER@$HOST:~/burrow/
scp -r templates/ $USER@$HOST:~/burrow/
ssh $USER@$HOST "sudo systemctl start burrow"
