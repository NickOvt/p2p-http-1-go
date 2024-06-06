FROM golang:latest

ENV port=8080

ENV ip=0.0.0.0

ENV GO111MODULE=off

WORKDIR /app

COPY node.go ./
COPY main_nodes_docker.json ./main_nodes.json

RUN touch "serverlog.log"

RUN CGO_ENABLED=0 GOOS=linux go build -o /node

EXPOSE $port

CMD ["sh", "-c", "/node ${port} ${ip}"]