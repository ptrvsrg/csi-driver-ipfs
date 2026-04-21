# Copyright 2026 ptrvsrg.
#
# Licensed under the Apache License, Version 2.0 (the License);
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an 'AS IS' BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ARG GOLANG_VERSION=1.26.2
ARG ALPINE_VERSION=3.23
ARG KUBO_VERSION=0.40.1
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Dependency stage
FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} AS deps

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Build stage
FROM deps AS builder

COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 \
    go build \
    -ldflags "-s -w \
    -X main.driverVersion=${VERSION} \
    -X main.gitCommit=${GIT_COMMIT} \
    -X main.buildDate=${BUILD_DATE}" \
    -o /bin/csi-driver-ipfs \
    ./cmd/csi-driver-ipfs/

FROM ipfs/kubo:v${KUBO_VERSION} AS kubo

# Runtime stage
FROM alpine:${ALPINE_VERSION} AS runtime

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="csi-driver-ipfs" \
      org.opencontainers.image.description="CSI driver for IPFS volumes" \
      org.opencontainers.image.source="https://github.com/ptrvsrg/csi-driver-ipfs" \
      org.opencontainers.image.commit="${GIT_COMMIT}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.build-date="${BUILD_DATE}"

RUN echo "http://mirror.yandex.ru/mirrors/alpine/v3.23/main" > /etc/apk/repositories \
    && echo "http://mirror.yandex.ru/mirrors/alpine/v3.23/community" >> /etc/apk/repositories \
    && apk add --no-cache \
        bash \
        ca-certificates \
        curl \
        e2fsprogs \
        make \
        xfsprogs

COPY --from=kubo /usr/local/bin/ipfs /usr/local/bin/ipfs
COPY --from=builder /bin/csi-driver-ipfs /bin/csi-driver-ipfs

# Define a non-root image user by default; Kubernetes manifests override to
# root for CSI driver workloads that need mount/umount privileges.
RUN addgroup -S -g 65532 csiipfs \
    && adduser -S -D -H -u 65532 -G csiipfs -s /sbin/nologin csiipfs

USER 65532:65532

ENTRYPOINT ["/bin/csi-driver-ipfs"]
