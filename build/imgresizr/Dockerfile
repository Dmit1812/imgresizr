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

# Begin env-to-ini build
#RUN go build contrib/environment-to-ini/environment-to-ini.go

# Copy produced files
#COPY ./bin/imgresizr /tmp/local

# Set permissions
RUN chmod 755 /app/imgresizr/bin/imgresizr
RUN chmod 644 /app/imgresizr/assets/error.png
# RUN chmod 644 /go/src/code.gitea.io/gitea/contrib/autocompletion/bash_autocomplete

FROM docker.io/library/alpine:3.18
#LABEL maintainer="maintainers@gitea.io"

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
#ENV GITEA_WORK_DIR /var/lib/gitea
#ENV GITEA_CUSTOM /var/lib/gitea/custom
#ENV GITEA_TEMP /tmp/gitea
#ENV TMPDIR /tmp/gitea

# TODO add to docs the ability to define the ini to load (useful to test and revert a config)
#ENV GITEA_APP_INI /etc/gitea/app.ini
#ENV HOME "/var/lib/gitea/git"
#VOLUME ["/var/lib/gitea", "/etc/gitea"]
WORKDIR /app/imgresizr

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/app/imgresizr/imgresizr"]
CMD []
