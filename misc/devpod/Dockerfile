FROM golang:1.22-bullseye AS build

WORKDIR /gno

# Download gno deps
ADD go.mod go.sum .
RUN go mod download

# Download gnodev deps
RUN mkdir -p ./contribs/gnodev
ADD contribs/gnodev/go.mod contribs/gnodev/go.sum ./contribs/gnodev
RUN cd ./contribs/gnodev && go mod download

ADD . ./

RUN mkdir -p /build

# Install gnokey & gno
RUN go build -o /build/gnokey ./gno.land/cmd/gnokey && \
    go build -o /build/gno ./gnovm/cmd/gno

# Install gnodev
RUN cd contribs/gnodev && go build -o /build/gnodev .

# Main image
FROM mcr.microsoft.com/devcontainers/base:debian

ENV PATH="/opt/bin:${PATH}"

WORKDIR /workspace/gno

COPY --from=build /gno /gno
RUN chmod -R 755 /gno

# Create Data folder
RUN mkdir -p /gno/data && chown root:vscode /gno/data && chmod 775 /gno/data


COPY --from=build /build /opt/bin

ENTRYPOINT ["/bin/bash"]
