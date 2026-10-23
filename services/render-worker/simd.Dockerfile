# Optional ISPC/scalar comparison. Standard native.Dockerfile selects libyuv.
FROM golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61 AS dependencies
RUN apt-get update && apt-get install -y --no-install-recommends cmake ca-certificates curl && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://github.com/ispc/ispc/releases/download/v1.32.0/ispc-v1.32.0-linux.tar.gz -o /tmp/ispc.tar.gz \
 && echo 'faf266bda2b702d68976ffd26bbd36c1ddba43424cc3c44f25b092cd1aa1f64a  /tmp/ispc.tar.gz' | sha256sum -c - \
 && mkdir /opt/ispc && tar xzf /tmp/ispc.tar.gz -C /opt/ispc --strip-components=1
ARG LIBYUV_COMMIT=47ba8e2160b71a7cf95c0720cd8944ab771e533e
RUN git clone https://chromium.googlesource.com/libyuv/libyuv /opt/libyuv \
 && cd /opt/libyuv && git checkout ${LIBYUV_COMMIT} \
 && cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DCMAKE_DISABLE_FIND_PACKAGE_JPEG=TRUE -DCMAKE_INSTALL_PREFIX=/opt/syntelle-simd \
 && cmake --build build --target yuv -j2 \
 && mkdir -p /opt/syntelle-simd/lib /opt/syntelle-simd/include \
 && cp build/libyuv.a /opt/syntelle-simd/lib/ && cp -r include/* /opt/syntelle-simd/include/
COPY services/render-worker/native/compose.ispc /tmp/compose.ispc
RUN /opt/ispc/bin/ispc /tmp/compose.ispc -O2 --target=sse2-i32x4,avx2-i32x8 --opt=disable-fma --pic -h /opt/syntelle-simd/include/compose.h -o /tmp/compose.o \
 && ar rcs /opt/syntelle-simd/lib/libsyntelle_ispc.a /tmp/compose*.o
WORKDIR /src/services/render-worker
ENV GOTOOLCHAIN=local CGO_ENABLED=1 GOMAXPROCS=2
COPY packages/render-contract /src/packages/render-contract
COPY services/render-worker/go.mod services/render-worker/go.sum ./
RUN go mod download
COPY services/render-worker/cmd ./cmd
COPY services/render-worker/internal ./internal
FROM dependencies AS build
ARG EXPERIMENT_TAGS=nativecodecs
RUN set -eu; mkdir /out; for command in render-worker render-probe render-frame; do \
 go build -p 2 -tags "$EXPERIMENT_TAGS" -trimpath -ldflags='-s -w' -o /out/$command ./cmd/$command; done
FROM debian:bookworm-slim@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8 AS runtime
COPY --from=build /usr/lib/x86_64-linux-gnu/libstdc++.so.6* /usr/lib/x86_64-linux-gnu/
COPY --from=build /lib/x86_64-linux-gnu/libgcc_s.so.1 /lib/x86_64-linux-gnu/libgcc_s.so.1
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /opt/libyuv/LICENSE /usr/local/share/syntelle/licenses/libyuv.LICENSE
RUN useradd --system --uid 10001 syntelle
COPY --from=build /out/ /native/
USER syntelle
ENTRYPOINT ["/native/render-worker"]
