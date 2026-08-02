# One command builds the library, the adapter binary, and the harness that runs
# decimal.js's own tests against it:
#
#   docker build -t decimaljs-go .
#   docker run --rm decimaljs-go            # parity table for the ported modules
#   docker run --rm decimaljs-go go test ./src/...
#
# The image carries Node only because the proof harness needs it. The library
# itself has no dependencies at all.
FROM golang:1.24-alpine

RUN apk add --no-cache nodejs

WORKDIR /app

COPY go.mod ./
COPY src/ ./src/
COPY adapter/ ./adapter/
COPY tests/ ./tests/

RUN go vet ./src/ ./adapter/... \
 && go test ./src/ \
 && go build -o /app/adapter/bin/decimald ./adapter/cmd/decimald \
 && node adapter/smoke.mjs

# The default command runs the vendored suite through the Go build and prints
# the per-module pass table.
CMD ["node", "adapter/run-parity.mjs", "--all"]
