FROM golang:1.22 AS build
WORKDIR /src
COPY . .
ARG TARGET=gateway
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/app ./cmd/${TARGET}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
COPY config /config
ENV CONFIG=/config/routing.json
USER nonroot
ENTRYPOINT ["/app"]
