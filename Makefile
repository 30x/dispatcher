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
	go test -cover ./kubernetes/... ./router/... ./nginx/... ./utils/... .

test-full:
	go test -tags=integration -cover ./kubernetes/... ./router/... ./nginx/... ./utils/... .

build: main.go
	go build

build-for-container: main.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o dispatcher .

build-image: build-for-container
	docker build -t thirtyx/dispatcher --build-arg GIT_COMMIT=$(GIT_COMMIT) .

coverage-router:
	go test -coverprofile=router-coverage.out ./router
	go tool cover -html=router-coverage.out
coverage-nginx:
	go test -coverprofile=nginx-coverage.out ./nginx
	go tool cover -html=nginx-coverage.out
coverage-kubernetes:
	go test -coverprofile=kubernetes-coverage.out ./kubernetes
	go tool cover -html=kubernetes-coverage.out
coverage-utils:
	go test -coverprofile=utils-coverage.out ./utils
	go tool cover -html=utils-coverage.out

coverage: coverage-router coverage-nginx coverage-kubernetes coverage-utils
