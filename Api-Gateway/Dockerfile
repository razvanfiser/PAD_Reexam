# Use the official Golang image as the base image
FROM golang:1.21-alpine

# Set the working directory inside the container
WORKDIR /go/src/app

# Copy the Go API gateway source code into the container
COPY . .

# Install the gorilla/mux package
RUN go get -u github.com/gorilla/mux
RUN go get -u github.com/go-redis/redis/v8
RUN go get -u github.com/afex/hystrix-go/hystrix

# Build the Go API gateway executable
RUN go build -o api-gateway

# Expose the port on which the API gateway will run
EXPOSE 8080

# Command to run the API gateway
CMD ["./api-gateway"]
