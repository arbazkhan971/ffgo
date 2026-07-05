# Build stage
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
	-ldflags "-s -w \
	-X github.com/arbazkhan971/ffgo/internal/buildinfo.Version=${VERSION} \
	-X github.com/arbazkhan971/ffgo/internal/buildinfo.Commit=${COMMIT} \
	-X github.com/arbazkhan971/ffgo/internal/buildinfo.Date=${DATE}" \
	-o /out/ffgo .

# Runtime stage — includes ffmpeg so the image is self-contained
FROM alpine:3.20
RUN apk add --no-cache ffmpeg
COPY --from=build /out/ffgo /usr/local/bin/ffgo
# Mount your media at /work
WORKDIR /work
ENTRYPOINT ["ffgo"]
CMD ["--help"]
