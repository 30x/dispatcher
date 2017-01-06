GIT_COMMIT=$(shell git rev-parse HEAD)

all: build

check: test lint

clean:
	rm -f coverage.out dispatcher router/router.test kubernetes/kubernetes.test nginx/nginx.test utils/utils.test

lint:
	golint -set_exit_status router
	golint -set_exit_status kubernetes
	golint -set_exit_status nginx

test:
	go test -cover ./kubernetes/... ./router/... ./utils/... .

test-full:
	go test -tags=integration -cover ./kubernetes/... ./router/... ./utils/... .

build: main.go
	go build

build-for-container: main.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o dispatcher .

build-image: build-for-container
	docker build -t dispatcher --build-arg GIT_COMMIT=$(GIT_COMMIT) .
