FROM golang:alpine as builder
RUN CGO_ENABLED=0 GOOS=linux go install -a -installsuffix=nocgo std
WORKDIR /go/src/github.com/Doist/pdfsvc
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -installsuffix=nocgo -o=/tmp/pdfsvc
FROM debian:stable-slim
ADD https://github.com/wkhtmltopdf/wkhtmltopdf/releases/download/0.12.4/wkhtmltox-0.12.4_linux-generic-amd64.tar.xz /tmp/archive.tar.xz
RUN export DEBIAN_FRONTEND=noninteractive \
	&& apt-get update \
	&& apt-get install -y --no-install-recommends \
		xz-utils fontconfig libxrender1 libxext6 \
		fonts-dejavu fonts-arkpandora \
	&& tar xf /tmp/archive.tar.xz -C /tmp \
	&& install /tmp/wkhtmltox/bin/wkhtmltopdf /usr/local/bin \
	&& apt-get purge -y xz-utils && apt-get clean \
	&& rm -rf /tmp/archive.tar.xz /tmp/wkhtmltox /var/cache/apt /var/lib/apt
COPY --from=builder /tmp/pdfsvc /usr/local/bin/
ENV ADDR=:8080
EXPOSE 8080
CMD ["/usr/local/bin/pdfsvc"]
