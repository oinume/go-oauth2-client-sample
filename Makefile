APP = oauth2-client-sample
PID = $(APP).pid
GO_TEST ?= go test -v -race

all: build

.PHONY: build
build:
	mkdir -p bin
	go build -o bin/$(APP) ./cmd/$(APP)

clean:
	${RM} $(APP)

run: build
	bin/$(APP)

kill:
	kill `cat $(PID)` 2> /dev/null || true

restart: kill clean build
	bin/$(APP) & echo $$! > $(PID)

watch: restart
	fswatch -o -e ".*" -e vendor -i "\\.go$$" . | xargs -n1 -I{} make restart || make kill

test:
	$(GO_TEST) ./...
