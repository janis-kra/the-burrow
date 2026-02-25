#!/bin/sh

GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o burrow ./cmd/burrow

ssh jk@aragorn.local "sudo systemctl stop burrow"
scp burrow jk@aragorn.local:~/burrow/
scp config.yaml jk@aragorn.local:~/burrow/
scp -r templates/ jk@aragorn.local:~/burrow/
ssh jk@aragorn.local "sudo systemctl start burrow"

