# Build context: repository root; copies only worker and shared contracts. The local cached compiler may be supplied explicitly.
ARG GO_BUILD_IMAGE=golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61
ARG LIBYUV_COMMIT=47ba8e2160b71a7cf95c0720cd8944ab771e533e
ARG COLOR_CONVERTER=libyuv
ARG GPU_BUILD_IMAGE=${GO_BUILD_IMAGE}
FROM ${GO_BUILD_IMAGE} AS native-dependencies
ARG LIBYUV_COMMIT
# A locally selected, verified SDK can reuse the exact phase-61 native archive.
# Clean builds fetch only the pinned libyuv source; ISPC is not a dependency.
RUN set -eu; \
 if test -f /opt/syntelle-simd/lib/libyuv.a; then \
   test "$(git -C /opt/libyuv rev-parse HEAD)" = "$LIBYUV_COMMIT"; \
   echo '206343214ec3df0d655abce040fd427a8a4c1d6752a5c121d0741e72f43fe704  /opt/syntelle-simd/lib/libyuv.a' | sha256sum -c -; \
 else \
   apt-get update; apt-get install -y --no-install-recommends cmake; rm -rf /var/lib/apt/lists/*; \
   git init /opt/libyuv; git -C /opt/libyuv remote add origin https://chromium.googlesource.com/libyuv/libyuv; \
   git -C /opt/libyuv fetch --depth 1 origin "$LIBYUV_COMMIT"; \
   git -C /opt/libyuv checkout --detach FETCH_HEAD; \
   test "$(git -C /opt/libyuv rev-parse HEAD)" = "$LIBYUV_COMMIT"; \
   cmake -S /opt/libyuv -B /opt/libyuv/build -DCMAKE_BUILD_TYPE=Release \
     -DCMAKE_DISABLE_FIND_PACKAGE_JPEG=TRUE -DCMAKE_INSTALL_PREFIX=/opt/syntelle-simd; \
   cmake --build /opt/libyuv/build --target yuv -j2; \
   mkdir -p /opt/syntelle-simd/lib /opt/syntelle-simd/include; \
   cp /opt/libyuv/build/libyuv.a /opt/syntelle-simd/lib/; \
   cp -r /opt/libyuv/include/* /opt/syntelle-simd/include/; \
 fi

FROM ${GO_BUILD_IMAGE} AS nvenc-dependencies
# Only libavcodec/libavutil and the NVENC encoder: no FFmpeg subprocess or CUDA toolkit.
ARG FFMPEG_COMMIT=8ae0b34901ba60a802f183ee75a250a9fc3e09a5
ARG NVCODEC_COMMIT=e844e5b26f46bb77479f063029595293aa8f812d
RUN set -eu; \
 git init /opt/nv-codec-headers; \
 git -C /opt/nv-codec-headers remote add origin https://github.com/FFmpeg/nv-codec-headers.git; \
 git -C /opt/nv-codec-headers fetch --depth 1 origin "$NVCODEC_COMMIT"; \
 git -C /opt/nv-codec-headers checkout --detach FETCH_HEAD; \
 test "$(git -C /opt/nv-codec-headers rev-parse HEAD)" = "$NVCODEC_COMMIT"; \
 make -C /opt/nv-codec-headers PREFIX=/opt/syntelle-nvenc install; \
 git init /opt/ffmpeg; \
 git -C /opt/ffmpeg remote add origin https://github.com/FFmpeg/FFmpeg.git; \
 git -C /opt/ffmpeg fetch --depth 1 origin "$FFMPEG_COMMIT"; \
 git -C /opt/ffmpeg checkout --detach FETCH_HEAD; \
 test "$(git -C /opt/ffmpeg rev-parse HEAD)" = "$FFMPEG_COMMIT"; \
 cd /opt/ffmpeg; \
 PKG_CONFIG_PATH=/opt/syntelle-nvenc/lib/pkgconfig ./configure \
 --prefix=/opt/syntelle-nvenc --disable-everything --disable-autodetect \
 --enable-ffnvcodec --enable-nvenc --enable-encoder=h264_nvenc \
 --enable-shared --disable-static --disable-programs --disable-doc --disable-x86asm \
 --disable-network --disable-avdevice --disable-avfilter --disable-avformat \
 --disable-swresample --disable-swscale; \
 make -j2; make install; \
 mkdir -p /opt/syntelle-nvenc/licenses; \
 cp COPYING.LGPLv2.1 /opt/syntelle-nvenc/licenses/FFmpeg.LICENSE; \
 cp /opt/nv-codec-headers/include/ffnvcodec/nvEncodeAPI.h /opt/syntelle-nvenc/licenses/nvEncodeAPI.h; \
 { printf 'ffmpeg=8.0.3\nffmpeg_commit=%s\nnvcodec=13.0.19.0\nnvcodec_commit=%s\n' "$FFMPEG_COMMIT" "$NVCODEC_COMMIT"; \
   sha256sum /opt/syntelle-nvenc/lib/libavcodec.so.* /opt/syntelle-nvenc/lib/libavutil.so.*; \
   cat ffbuild/config.mak; } > /opt/syntelle-nvenc/build.txt

FROM ${GPU_BUILD_IMAGE} AS gpu-kernel
RUN set -eu; \
 if ! command -v clang-14 >/dev/null; then \
  apt-get update; apt-get install -y --no-install-recommends clang-14=1:14.0.6-12; rm -rf /var/lib/apt/lists/*; \
 fi; \
 test "$(clang-14 -dumpversion)" = 14.0.6
WORKDIR /kernel
COPY services/render-worker/internal/nativerender/gpu_kernel.cu services/render-worker/internal/nativerender/gpu_types.h services/render-worker/internal/nativerender/gpu_patches.cuh ./
RUN clang-14 -x cuda --cuda-device-only --cuda-gpu-arch=sm_60 -nocudainc -nocudalib \
 -O2 -ffp-contract=off -fno-fast-math -S gpu_kernel.cu -o gpu_kernel.ptx \
 && { clang-14 --version; sha256sum gpu_kernel.cu gpu_types.h gpu_patches.cuh gpu_kernel.ptx; } > gpu-build.txt

FROM native-dependencies AS build
ARG GO_BUILD_IMAGE
ARG LIBYUV_COMMIT
ARG COLOR_CONVERTER
WORKDIR /src/services/render-worker
ENV GOTOOLCHAIN=local CGO_ENABLED=1 CGO_CFLAGS="" CGO_LDFLAGS="" GOMAXPROCS=2
COPY --from=nvenc-dependencies /opt/syntelle-nvenc /opt/syntelle-nvenc
COPY packages/render-contract /src/packages/render-contract
COPY services/render-worker/go.mod services/render-worker/go.sum ./
RUN go mod download \
 && go work init . ../../packages/render-contract && go mod verify && rm go.work \
 && test "$(go version | cut -d ' ' -f 3)" = go1.27.2 \
 && printf '%s\n' \
 '3f8eace5169e825e5914a40570c6de3162e167d98c53aa3a47b77cb55a978b98  /go/pkg/mod/github.com/pion/mediadevices@v0.10.1/pkg/codec/openh264/lib/libopenh264-linux-x64.a' \
 'ae9a65a8bb0df23e16cead390aea6a6b2d7b52fbc0705cd322ec952cb636baa5  /go/pkg/mod/github.com/pion/mediadevices@v0.10.1/pkg/codec/opus/lib/libopus-linux-x64.a' | sha256sum -c -
COPY services/render-worker/cmd/render-worker ./cmd/render-worker
COPY services/render-worker/internal ./internal
COPY --from=gpu-kernel /kernel/gpu_kernel.ptx ./internal/nativerender/gpu_kernel.ptx
RUN set -eu; \
 case "$COLOR_CONVERTER" in libyuv) tags=nativecodecs,libyuv,nvenc,nativegpu;; scalar) tags=nativecodecs,nvenc,nativegpu;; *) echo 'Unsupported color converter' >&2; exit 1;; esac; \
 go test -p 2 -tags "$tags" ./internal/nativeclock ./internal/nativeanimation ./internal/nativeadapter ./internal/nativerender ./internal/nativemedia; \
 go build -p 2 -tags "$tags" -trimpath -ldflags="-s -w" -o /syntelle-render-worker ./cmd/render-worker; \
 { printf 'builder=%s\nopenh264=2.1.1\nopus=1.3.1\ncolor_converter=%s\nlibyuv=%s\n' "$GO_BUILD_IMAGE" "$COLOR_CONVERTER" "$LIBYUV_COMMIT"; \
      go version -m /syntelle-render-worker; \
      sha256sum /go/pkg/mod/github.com/pion/mediadevices@v0.10.1/pkg/codec/openh264/lib/libopenh264-linux-x64.a \
                /go/pkg/mod/github.com/pion/mediadevices@v0.10.1/pkg/codec/opus/lib/libopus-linux-x64.a \
                /opt/syntelle-simd/lib/libyuv.a \
                /usr/lib/x86_64-linux-gnu/libstdc++.so.6 /lib/x86_64-linux-gnu/libgcc_s.so.1 /etc/ssl/certs/ca-certificates.crt; \
      dpkg-query -W ca-certificates libstdc++6 libgcc-s1 libc6; \
    } > /native-build.txt

FROM debian:bookworm-slim@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8
ARG LIBYUV_COMMIT
ARG COLOR_CONVERTER
# Copy the complete SONAME pair from the pinned Bookworm builder; no apt mirror drift.
COPY --from=build /usr/lib/x86_64-linux-gnu/libstdc++.so.6* /usr/lib/x86_64-linux-gnu/
COPY --from=build /lib/x86_64-linux-gnu/libgcc_s.so.1 /lib/x86_64-linux-gnu/libgcc_s.so.1
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=nvenc-dependencies /opt/syntelle-nvenc/lib/libavcodec.so* /opt/syntelle-nvenc/lib/
COPY --from=nvenc-dependencies /opt/syntelle-nvenc/lib/libavutil.so* /opt/syntelle-nvenc/lib/
COPY --from=nvenc-dependencies /opt/syntelle-nvenc/licenses /usr/local/share/syntelle/licenses/nvenc
COPY --from=nvenc-dependencies /opt/syntelle-nvenc/build.txt /usr/local/share/syntelle/nvenc-build.txt
COPY --from=gpu-kernel /kernel/gpu-build.txt /usr/local/share/syntelle/gpu-build.txt
RUN useradd --system --uid 10001 syntelle
COPY --from=build /syntelle-render-worker /usr/local/bin/syntelle-render-worker
COPY --from=build /native-build.txt /usr/local/share/syntelle/native-build.txt
COPY --from=build /opt/libyuv/LICENSE /usr/local/share/syntelle/licenses/libyuv.LICENSE
RUN dpkg-query -W libc6 >> /usr/local/share/syntelle/native-build.txt \
 && ldd /usr/local/bin/syntelle-render-worker \
 && /usr/local/bin/syntelle-render-worker --help
LABEL org.opencontainers.image.title="Syntelle native render worker" \
 io.syntelle.project="syntelle" io.syntelle.renderer="auto-cuda-cpu" \
 io.syntelle.mediadevices="0.10.1" io.syntelle.webrtc="4.2.25" \
 io.syntelle.openh264="2.1.1" io.syntelle.opus="1.3.1" io.syntelle.video-encoder="auto-nvenc-openh264" \
 io.syntelle.color-converter="${COLOR_CONVERTER}" io.syntelle.libyuv="${LIBYUV_COMMIT}" \
 io.syntelle.openh264.sha256="3f8eace5169e825e5914a40570c6de3162e167d98c53aa3a47b77cb55a978b98" \
 io.syntelle.opus.sha256="ae9a65a8bb0df23e16cead390aea6a6b2d7b52fbc0705cd322ec952cb636baa5"
USER syntelle
ENTRYPOINT ["syntelle-render-worker"]
