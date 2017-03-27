NAME := statsd-ha-proxy
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
	go build  ${LDFLAGS} -o build/root/usr/bin/${NAME} ./cmd/${NAME}

tar: build
	mkdir -p build/root/etc/${NAME}
	./build/root/usr/bin/${NAME} --print-default-config > build/root/etc/${NAME}/config.yml
	mkdir -p build/root/usr/lib/systemd/system
	cp pkg/${NAME}.service build/root/usr/lib/systemd/system/${NAME}.service
	mkdir -p build/root/etc/logrotate.d
	cp pkg/logrotate build/root/etc/logrotate.d/${NAME}

	tar -czvPf build/${NAME}-${VERSION}-${RELEASE}.tar.gz -C build/root .

rpm:
	fpm -t rpm \
		-s "tar" \
		--description "statsd-ha-proxy" \
		--vendor "Alex Akulov" \
		--url "https://github.com/AlexAkulov/statsd-ha-proxy" \
		--license "GPLv3" \
		--name "${NAME}" \
		--version "${VERSION}" \
		--iteration "${RELEASE}" \
		--after-install "./pkg/postinst.sh" \
		--depends logrotate \
		--config-files "/etc/statsd-ha-proxy/config.yml" \
		-p build \
		build/${NAME}-${VERSION}-${RELEASE}.tar.gz

clean:
	rm -rf build

.PHONY: test
