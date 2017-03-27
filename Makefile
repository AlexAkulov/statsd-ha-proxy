VERSION := $(shell git describe --always --tags --abbrev=0 | tail -c +2)
RELEASE := $(shell git describe --always --tags | awk -F- '{ if ($$2) dot="."} END { printf "1%s%s%s%s\n",dot,$$2,dot,$$3}')
GO_VERSION := $(shell go version | cut -d' ' -f3)
BUILD_DATE := $(shell date --iso-8601=second)
LDFLAGS := -ldflags "-X main.version=${VERSION}-${RELEASE} -X main.goVersion=${GO_VERSION} -X main.buildDate=${BUILD_DATE}"

default: build

test:
	echo "No tests"

build: clean
	mkdir -p build/root/usr/bin/
	go build  ${LDFLAGS} -o build/root/usr/bin/statsd-ha-proxy ./cmd/statsd-ha-proxy

rpm: build
	fpm -t rpm \
		-s "dir" \
		--description "statsd-ha-proxy" \
		-C ./build/root/ \
		--vendor "Alex Akulov" \
		--url "https://github.com/AlexAkulov/statsd-ha-proxy" \
		--name "statsd-ha-proxy" \
		--version "${VERSION}" \
		--iteration "${RELEASE}" \
		-p build

clean:
	rm -rf build

.PHONY: test
