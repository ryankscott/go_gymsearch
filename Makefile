ARTIFACT = go_gymsearch 

all: build

build: GOOS ?= darwin
build: GOARCH ?= amd64
build: clean
		GOOS=${GOOS} GOARCH=${GOARCH} go build -o ${ARTIFACT} -a .

clean:
		rm -f ${ARTIFACT}

image: clean
		GOOS=linux GOARCH=amd64 go build -o ${ARTIFACT} -a .

test:
		go test -v

run: build
	/${ARTIFACT}
