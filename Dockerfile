# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25
FROM golang:${GO_VERSION} AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE
RUN test -n "${SERVICE}"
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/service ./services/${SERVICE}/cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=build /out/service /app/service

EXPOSE 8080
ENTRYPOINT ["/app/service"]
