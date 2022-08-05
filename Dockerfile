FROM golang:latest
COPY cmd/orchestrator /cmd/orchestrator
RUN DEBIAN_FRONTEND=noninteractive; apt update && apt install -y ca-certificates openssl && rm -rf /var/lib/apt/lists/*
RUN go build -o orchestrator ./cmd/orchestrator

FROM digitalocean/doctl:latest
ENV DIGITALOCEAN_ACCESS_TOKEN=$DIGITALOCEAN_ACCESS_TOKEN
RUN doctl serverless install && doctl serverless connect
COPY --from=0 /orchestrator .
RUN ./orchestrator
