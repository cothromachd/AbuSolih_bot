FROM golang:latest

WORKDIR /AbuSolih_bot

COPY . .

RUN go mod download
RUN go build -o main .


CMD ["./main"]