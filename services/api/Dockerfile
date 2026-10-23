FROM golang:1.27.2-bookworm@sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61 AS build
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl libasound2-dev && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL https://csspeechstorage.blob.core.windows.net/drop/1.52.0/SpeechSDK-Linux-1.52.0.tar.gz -o /tmp/speech.tar.gz \
 && echo '2573d5336cdba31759967b77b3596dcd450b802fa8c1a410245c541a5df0fb47  /tmp/speech.tar.gz' | sha256sum -c - \
 && mkdir /opt/speech-sdk && tar -xzf /tmp/speech.tar.gz -C /opt/speech-sdk --strip-components=1
WORKDIR /src/services/api
COPY packages/render-contract /src/packages/render-contract
COPY services/api/go.mod services/api/go.sum ./
RUN go mod download
COPY services/api/cmd ./cmd
COPY services/api/internal ./internal
ENV CGO_CFLAGS="-I/opt/speech-sdk/include/c_api" CGO_LDFLAGS="-L/opt/speech-sdk/lib/x64 -lMicrosoft.CognitiveServices.Speech.core" LD_LIBRARY_PATH=/opt/speech-sdk/lib/x64
RUN CGO_ENABLED=1 go test -tags speech ./... && CGO_ENABLED=1 go build -tags speech -trimpath -o /syntelle-api ./cmd/api

FROM debian:bookworm-slim@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8
LABEL io.syntelle.project="syntelle"
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libasound2 libssl3 libstdc++6 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --uid 10001 syntelle
COPY --from=build /syntelle-api /usr/local/bin/syntelle-api
COPY --from=build /opt/speech-sdk/lib/x64/ /opt/speech-sdk/lib/
COPY --from=build /opt/speech-sdk/LICENSE.md /opt/speech-sdk/LICENSE.md
USER syntelle
ENV LISTEN_ADDR=0.0.0.0:8080
ENV LD_LIBRARY_PATH=/opt/speech-sdk/lib
EXPOSE 8080
CMD ["syntelle-api"]
