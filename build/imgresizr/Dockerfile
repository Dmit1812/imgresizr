# Build stage
FROM docker.io/library/golang:1.21-alpine3.18 AS build-env

#Build deps
RUN apk --no-cache add \
    build-base \
    git \
    vips-dev \
    && rm -rf /var/cache/apk/*

# Setup repo
COPY . /app/imgresizr
WORKDIR /app/imgresizr

# Checkout version if set
RUN make build


# Set permissions
RUN chmod 755 /app/imgresizr/bin/imgresizr
RUN chmod 644 /app/imgresizr/assets/error.png

FROM docker.io/library/alpine:3.18

EXPOSE 9000

RUN apk --no-cache add \
    bash \
    ca-certificates \
    dumb-init \
    gettext \
    curl \
    vips \ 
    && rm -rf /var/cache/apk/*

RUN addgroup \
    -S -g 1000 \
    imgresizr && \
  adduser \
    -S -H -D \
    -h /app/imgresizr \
    -s /bin/bash \
    -u 1000 \
    -G imgresizr \
    imgresizr

RUN mkdir -p /app/imgresizr
RUN chown imgresizr:imgresizr /app/imgresizr

COPY --from=build-env --chown=root:root /app/imgresizr/bin/imgresizr /app/imgresizr/imgresizr
COPY --from=build-env --chown=root:root /app/imgresizr/assets  /app/imgresizr/assets

# imgresizr:imgresizr
USER 1000:1000

WORKDIR /app/imgresizr

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/app/imgresizr/imgresizr"]
CMD []
