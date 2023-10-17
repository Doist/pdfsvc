FROM public.ecr.aws/docker/library/golang:alpine AS builder
WORKDIR /app
ENV GOFLAGS="-ldflags=-w -trimpath" CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN go version && go build

FROM debian:stretch-slim
ADD https://github.com/wkhtmltopdf/wkhtmltopdf/releases/download/0.12.5/wkhtmltox_0.12.5-1.stretch_amd64.deb /tmp/package.deb
RUN export DEBIAN_FRONTEND=noninteractive \
	&& apt-get update \
	&& dpkg --install /tmp/package.deb || apt-get -f -y --no-install-recommends install \
	&& apt-get install -y --no-install-recommends \
		fonts-arkpandora \
		fonts-dejavu \
		fonts-ipafont \
		fonts-liberation2 \
		fonts-unfonts-core \
		fonts-vlgothic \
		fonts-wqy-zenhei \
	&& apt-get clean \
	&& rm -rf /tmp/package.deb /var/cache/apt /var/lib/apt /var/log/* \
	&& dpkg -l wkhtmltox | grep ^ii
COPY --from=builder /app/pdfsvc /usr/local/bin/
ENV ADDR=:8080
EXPOSE 8080
CMD ["/usr/local/bin/pdfsvc"]
