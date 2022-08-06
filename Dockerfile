FROM golang:latest
RUN mkdir src/app
WORKDIR /src/app
COPY . .
COPY cmd/orchestrator /cmd/orchestrator
RUN DEBIAN_FRONTEND=noninteractive; apt update && apt install -y ca-certificates openssl && rm -rf /var/lib/apt/lists/*
RUN go build -o orchestrator ./cmd/orchestrator

FROM ubuntu:latest
RUN DEBIAN_FRONTEND=noninteractive; apt update && apt install -y wget && rm -rf /var/lib/apt/lists/*

ARG digitalocean_access_token
ENV DIGITALOCEAN_ACCESS_TOKEN=$digitalocean_access_token
RUN wget https://github.com/digitalocean/doctl/releases/download/v1.78.0/doctl-1.78.0-linux-amd64.tar.gz
RUN tar xf doctl-1.78.0-linux-amd64.tar.gz
RUN mv doctl /usr/local/bin
RUN doctl serverless install; doctl serverless connect

ENV WAIT_VERSION 2.7.2
RUN wget https://github.com/ufoscout/docker-compose-wait/releases/download/$WAIT_VERSION/wait
RUN chmod +x /wait

COPY --from=0 /src/app/orchestrator .
CMD [ "./orchestrator", "1-10000000000" ]
