FROM public.ecr.aws/docker/library/golang:alpine AS builder
WORKDIR /app
ENV GOFLAGS="-ldflags=-w -trimpath" CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go version && go build

FROM public.ecr.aws/docker/library/alpine:latest
RUN apk add font-noto font-noto-cjk font-noto-extra weasyprint

COPY --from=builder /app/pdfsvc /usr/bin/
ENV ADDR=:8080
EXPOSE 8080
CMD ["/usr/bin/pdfsvc"]
