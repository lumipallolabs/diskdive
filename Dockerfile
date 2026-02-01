FROM golang:latest

WORKDIR /app
COPY . .
RUN go build -o diskdive .

ENTRYPOINT ["./diskdive"]
CMD ["/app"]
