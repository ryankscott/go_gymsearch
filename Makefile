ARTIFACT = gymsearch 

all: build

build: GOOS ?= darwin
build: GOARCH ?= amd64
build: clean
	cp -r ~/Code/gym_frontend/build/ ui/build/
	GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 go build -o ${ARTIFACT} -a .

clean:
	rm -rf ${ARTIFACT}

image: clean
	cp -r ~/Code/gym_frontend/build/ ui/build/
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ${ARTIFACT} -a .
push:
	docker build --no-cache -t ryankscott/go_gymsearch .
	docker push ryankscott/go_gymsearch

test:
	go test -v

run: build
	./${ARTIFACT}
