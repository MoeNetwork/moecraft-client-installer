#!/bin/sh

echo 'Building binary for GNU/Linux (amd64)'
GOOS=linux GOARCH=amd64 go build -o bin/installer-linux-amd64
echo 'Building binary for GNU/Linux (386)'
GOOS=linux GOARCH=386 go build -o bin/installer-linux-386

echo 'Building binary for macOS (amd64)'
GOOS=darwin GOARCH=amd64 go build -o bin/installer-darwin-amd64

echo 'Building binary for Windows (amd64)'
GOOS=windows GOARCH=amd64 go build -o bin/installer-windows-amd64.exe
echo 'Building binary for Windows (386)'
GOOS=windows GOARCH=386 go build -o bin/installer-windows-386.exe
