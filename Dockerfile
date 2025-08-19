FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    file \
    fuse \
    libfuse2 \
    tcl \
    tcl-dev \
    tk \
    tk-dev \
    wget

RUN wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz \
    && rm go1.25.0.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:$PATH"

RUN wget https://github.com/linuxdeploy/linuxdeploy/releases/latest/download/linuxdeploy-x86_64.AppImage -O /usr/local/bin/linuxdeploy \
    && chmod +x /usr/local/bin/linuxdeploy

RUN wget https://github.com/upx/upx/releases/download/v5.0.2/upx-5.0.2-amd64_linux.tar.xz \
    && tar -xf upx-5.0.2-amd64_linux.tar.xz \
    && mv upx-5.0.2-amd64_linux/upx /usr/local/bin/upx \
    && rm upx-5.0.2-amd64_linux.tar.xz

RUN wget https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage -O /usr/local/bin/appimagetool \
    && chmod +x /usr/local/bin/appimagetool

WORKDIR /app

VOLUME ["/release"]

CMD ["./build-appimage.sh"]
