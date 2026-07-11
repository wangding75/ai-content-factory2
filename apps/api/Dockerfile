FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
ARG TARGET=api
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${TARGET}

FROM alpine:3.22
RUN adduser -D -H -u 10001 appuser
USER appuser
COPY --from=build /out/app /app
ENTRYPOINT ["/app"]