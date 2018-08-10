@echo off

echo Building binary for GNU/Linux (amd64)
set GOOS=linux
set GOARCH=amd64
go build -o bin/installer-linux-amd64

echo Building binary for GNU/Linux (386)
set GOOS=linux
set GOARCH=386
go build -o bin/installer-linux-386

echo Building binary for macOS (amd64)
set GOOS=darwin
set GOARCH=amd64
go build -o bin/installer-darwin-amd64

echo Building binary for Windows (amd64)
set GOOS=windows
set GOARCH=amd64
go build -o bin/installer-windows-amd64.exe

echo Building binary for Windows (386)
set GOOS=windows
set GOARCH=386
go build -o bin/installer-windows-386.exe
