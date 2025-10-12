# Build stage
FROM golang:latest AS build-stage

WORKDIR /app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -v -o metarlive .

# Run stage
FROM alpine:latest

WORKDIR /root/

COPY --from=build-stage /app/metarlive .

CMD ["./metarlive"]
