FROM golang:1.24
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN go build -o db-service main.go
EXPOSE 8081
CMD ["./db-service"]