FROM golang:1.23 AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY internal internal
COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -o /artifacts-mover ./

FROM alpine:3.21

WORKDIR /app

COPY --from=build /artifacts-mover /artifacts-mover

ENTRYPOINT ["/artifacts-mover"]
CMD ["-config", "/app/config.yaml"]
