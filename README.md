# moecraft-client-installer

## Environment variables for testing

#### BASE_URL

Download the client from specified url instead of pre-defined ones.

Must end with `/`.

```bash
~/project $ BASE_URL=http://localhost:2015/ go run main.go
# or
~/project $ go build -o installer
~/project $ BASE_URL=http://localhost:2015/ ./installer
```

#### USE_WORK_DIR

Install the client to working directory.

```bash
~/project $ mkdir tmp && cd tmp
~/project/tmp $ USE_WORK_DIR=1 go run ../main.go
# or
~/project/tmp $ USE_WORK_DIR=true go run ../main.go
# Then the client will be installed to ~/project/tmp
```
