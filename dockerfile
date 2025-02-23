    # Use the official Go image to build the bot
    FROM golang:1.24 AS builder

    # Set the working directory
    WORKDIR /app

    # Copy go module files and download dependencies
    COPY go.mod go.sum ./
    RUN go mod download

    # Copy the source code
    COPY . .

    # Build the Go application and explicitly specify the output path
    RUN go build -o app 


    # Command to run the bot
    CMD ["./app"]
