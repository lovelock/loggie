# Build the binary
FROM --platform=$BUILDPLATFORM golang:1.21 as builder

ARG TARGETARCH
ARG TARGETOS

# Copy in the go src
WORKDIR /
COPY . .
# Build
RUN make build

# Run
FROM debian:buster-slim
WORKDIR /
COPY --from=builder /loggie .

ENTRYPOINT ["/loggie"]
