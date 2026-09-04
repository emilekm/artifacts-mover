FROM golang:1.26 AS build

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY internal internal
COPY cmd cmd

RUN CGO_ENABLED=0 GOOS=linux go build -o /artifacts-mover ./cmd

FROM alpine:3.21

WORKDIR /app

COPY --from=build /artifacts-mover /artifacts-mover

ENTRYPOINT ["/artifacts-mover"]
CMD ["-config", "/app/config.yaml"]
