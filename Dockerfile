FROM golang:1.25-alpine3.23 AS build

WORKDIR /app
RUN apk add --no-cache git
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -mod=mod \
  -tags="netgo,osusergo" \
  -trimpath \
  -buildvcs=false \
  -ldflags="-s -w" \
  -o /socketing-socket-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /socketing-socket-server /app/socketing-socket-server
EXPOSE 3000
ENTRYPOINT ["/app/socketing-socket-server"]
