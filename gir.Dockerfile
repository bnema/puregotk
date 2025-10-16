FROM docker.io/library/alpine:latest

RUN apk add --no-cache \
    cairo-dev \
    gtk4.0-dev \
    libadwaita-dev

WORKDIR /gir-files

RUN cp -r /usr/share/gir-1.0/*.gir .
