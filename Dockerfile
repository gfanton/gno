# build
FROM        golang:1.21-alpine3.18 AS build
RUN         mkdir -p /opt/gno/src /opt/build
WORKDIR     /opt/build
ADD         go.mod go.sum ./
RUN         go mod download
ADD         . ./
RUN         go build -o ./build/gnoland   ./gno.land/cmd/gnoland
RUN         go build -o ./build/gnokey    ./gno.land/cmd/gnokey
RUN         go build -o ./build/gnofaucet ./gno.land/cmd/gnofaucet
RUN         go build -o ./build/gnoweb    ./gno.land/cmd/gnoweb
RUN         go build -o ./build/gnotxsync ./gno.land/cmd/gnotxsync
RUN         go build -o ./build/gno       ./gnovm/cmd/gno
RUN         ls -la ./build
ADD         . /opt/gno/src/
RUN         rm -rf /opt/gno/src/.git

# setup upx base
FROM        alpine:3.18 AS upx-build
ENV         UPX_VERSION=7f9d381c7b594ecf7d52cc59a50f3c0f16527599
ENV         LDFLAGS=-static
RUN         apk add --no-cache build-base ucl-dev zlib-dev git cmake
RUN         git clone --depth 1 --recursive -b devel https://github.com/upx/upx.git /upx
RUN         cd /upx/src && make -j2 release CHECK_WHITESPACE=
RUN         /upx/build/release/upx --lzma -o /usr/bin/upx /upx/build/release/upx

# pack every binary
FROM        upx-build AS packer
COPY        --from=build /opt/build/build /out
#           gofmt is required by `gnokey maketx addpkg`
COPY        --from=build /usr/local/go/bin/gofmt /out/gofmt 
RUN         upx --lzma /out/*

# runtime-base + runtime-tls
FROM        alpine:3.18 AS runtime-base
ENV         PATH="${PATH}:/opt/gno/bin"
WORKDIR     /opt/gno/src

FROM        runtime-base AS runtime-tls
RUN         apk update && apk add --no-cache expect ca-certificates && update-ca-certificates && \
            rm -rf /var/cache/apk/* /tmp/* /var/tmp/* # cleanup

# slim images
FROM        runtime-base AS gnoland-slim
WORKDIR     /opt/gno/src/gno.land/
COPY        --from=packer /out/gnoland /opt/gno/bin/

ENTRYPOINT  ["gnoland"]
EXPOSE      26657 36657

FROM        runtime-base AS gnokey-slim
COPY        --from=packer /out/gnokey /opt/gno/bin/
ENTRYPOINT  ["gnokey"]

FROM        runtime-base AS gno-slim
COPY        --from=packer /out/gno /opt/gno/bin/
ENTRYPOINT  ["gno"]

FROM        runtime-tls AS gnofaucet-slim
COPY        --from=packer /out/gnofaucet /opt/gno/bin/
ENTRYPOINT  ["gnofaucet"]
EXPOSE      5050

FROM        runtime-tls AS gnotxsync-slim
COPY        --from=packer /out/gnotxsync /opt/gno/bin/
ENTRYPOINT  ["gnotxsync"]

FROM        runtime-tls AS gnoweb-slim
COPY        --from=packer /out/gnoweb /opt/gno/bin/
COPY        --from=build /opt/gno/src/gno.land/cmd/gnoweb /opt/gno/src/gnoweb
ENTRYPOINT  ["gnoweb"]
EXPOSE      8888

# all, contains everything.
FROM        runtime-tls AS all
COPY        --from=packer /out/* /opt/gno/bin/
COPY        --from=build /opt/gno/src /opt/gno/src
