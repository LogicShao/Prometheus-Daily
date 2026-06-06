FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/m-daily-news ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /workspace

COPY --from=build /out/m-daily-news /usr/local/bin/m-daily-news
COPY VERSION ./VERSION
COPY prompt.md ./prompt.md
COPY templates ./templates
COPY content ./content

ENV WORKSPACE=/workspace
ENV PORT=8080

EXPOSE 8080

CMD ["m-daily-news"]
