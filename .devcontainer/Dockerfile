FROM mcr.microsoft.com/devcontainers/go:1.26-bookworm

ENV GOTOOLCHAIN=auto

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        libegl1-mesa-dev \
        libgles2-mesa-dev \
        libwayland-dev \
        libx11-xcb-dev \
        libxcursor-dev \
        libxfixes-dev \
        libxkbcommon-x11-dev \
        pkg-config \
    && rm -rf /var/lib/apt/lists/*
