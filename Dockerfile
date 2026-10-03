# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/mockvendors ./cmd/mockvendors \
 && go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

# Distroless static: no shell or package manager, runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
