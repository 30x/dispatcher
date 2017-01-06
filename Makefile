GIT_COMMIT=$(shell git rev-parse HEAD)

all: build

check: test lint

clean:
	rm -f coverage.out dispatcher router/router.test kubernetes/kubernetes.test nginx/nginx.test utils/utils.test

lint:
	golint router
	golint kubernetes
	golint nginx

test:
	go test -cover $$(glide novendor)

test-full:
	go test -tags=integration -cover $$(glide novendor)

build: main.go
	go build

build-for-container: main.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o dispatcher .

build-image: build-for-container
	docker build -t dispatcher --build-arg GIT_COMMIT=$(GIT_COMMIT) .
