FROM debian:bookworm-slim

RUN dpkg --add-architecture arm64
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    file \
    fuse \
    gcc-aarch64-linux-gnu \
    gcc-mingw-w64 \
    git \
    libfuse2 \
    libc6-dev-arm64-cross \
    tcl \
    tcl-dev \
    tk \
    tk-dev \
    wget \
    tcl-dev:arm64 \
    tk-dev:arm64 \
    libx11-dev:arm64 \
    zlib1g-dev:arm64 \
    libc6-dev:arm64

RUN wget --quiet https://go.dev/dl/go1.26.2.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.*.*.linux-amd64.tar.gz \
    && rm go1.*.*.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:$PATH"

RUN wget --quiet https://github.com/linuxdeploy/linuxdeploy/releases/latest/download/linuxdeploy-x86_64.AppImage -O /usr/local/bin/linuxdeploy \
    && chmod +x /usr/local/bin/linuxdeploy

RUN wget --quiet https://github.com/upx/upx/releases/download/v5.0.2/upx-5.0.2-amd64_linux.tar.xz \
    && tar -xf upx-5.0.2-amd64_linux.tar.xz \
    && mv upx-5.0.2-amd64_linux/upx /usr/local/bin/upx \
    && rm upx-5.0.2-amd64_linux.tar.xz

RUN wget --quiet https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage -O /usr/local/bin/appimagetool \
    && chmod +x /usr/local/bin/appimagetool

ARG BUILD_UID=2000
ARG BUILD_GID=2000
RUN groupadd -g "$BUILD_GID" builder \
    && useradd -m -u "$BUILD_UID" -g "$BUILD_GID" -s /bin/bash builder

WORKDIR /app

ENV IS_DOCKER 1

VOLUME ["/release"]

USER builder
CMD ["./release.sh"]
