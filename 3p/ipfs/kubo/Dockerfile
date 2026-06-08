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

ARG GOLANG_VERSION=1.26.4
ARG ALPINE_VERSION=3.23
ARG KUBO_VERSION=0.41.0
ARG TARGETOS=linux
ARG TARGETARCH=amd64

FROM golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} AS builder

ARG KUBO_VERSION
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

RUN apk add --no-cache git

RUN git clone --branch "v${KUBO_VERSION}" --depth 1 https://github.com/ipfs/kubo.git kubo

WORKDIR /src/kubo

COPY patches/v${KUBO_VERSION} /tmp/patches/

RUN git apply /tmp/patches/*.patch --stat --verbose

RUN go mod download

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 GOFLAGS=-buildvcs=false \
    go build -trimpath -ldflags "-s -w" -o /usr/local/bin/ipfs ./cmd/ipfs

FROM scratch

COPY --from=builder /usr/local/bin/ipfs /usr/local/bin/ipfs
