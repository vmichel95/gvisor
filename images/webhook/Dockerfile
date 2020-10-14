# Copyright 2020 The gVisor Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.15-alpine

# Setup the environment.
ENV MOD=gvisor.dev/images/webhook
ENV CGO_ENABLED=0
RUN apk --no-cache add git

# Install the webhook.
COPY ./ $GOPATH/$MOD
WORKDIR $GOPATH/$MOD
RUN go build -o /gvisor-injection-admission-webhook ./cmd

USER nobody:nobody
ENTRYPOINT ["/gvisor-injection-admission-webhook", "--pod-namespace-labels", "e2e-framework"]
