ARG GO_VERSION=1.12.1

# Workaround to utilize the global GO_VERSION argument
# since "COPY --from" doesn't support variables.
FROM golang:${GO_VERSION}-stretch as golang-ignore


####### Dev Image ########
# This is an image to facilitate development of the agent.  It installs all of
# the build tools for building collectd and the go agent, along with some other
# useful utilities.  The agent image is copied from the final-image stage to
# the /bundle dir in here and the SIGNALFX_BUNDLE_DIR is set to point to that.
FROM ubuntu:18.04 as dev-extras

ARG TARGET_ARCH

RUN apt update &&\
    apt install -y \
      curl \
      git \
      inotify-tools \
      iproute2 \
      jq \
      net-tools \
      python3-pip \
      socat \
      vim \
      wget


ENV SIGNALFX_BUNDLE_DIR=/bundle \
    TEST_SERVICES_DIR=/usr/src/signalfx-agent/test-services \
    AGENT_BIN=/usr/src/signalfx-agent/signalfx-agent \
    PYTHONPATH=/usr/src/signalfx-agent/python \
    AGENT_VERSION=latest \
    BUILD_TIME=2017-01-25T13:17:17-0500 \
    GOOS=linux \
    LC_ALL=C.UTF-8 \
    LANG=C.UTF-8

RUN pip3 install ipython ipdb

# Install helm
ARG HELM_VERSION=v2.13.0
WORKDIR /tmp
RUN wget -O helm.tar.gz https://storage.googleapis.com/kubernetes-helm/helm-${HELM_VERSION}-linux-${TARGET_ARCH}.tar.gz && \
    tar -zxvf /tmp/helm.tar.gz && \
    mv linux-${TARGET_ARCH}/helm /usr/local/bin/helm && \
    chmod a+x /usr/local/bin/helm

WORKDIR /usr/src/signalfx-agent
CMD ["/bin/bash"]
ENV PATH=$PATH:/usr/local/go/bin:/go/bin GOPATH=/go

COPY --from=golang-ignore /usr/local/go/ /usr/local/go

RUN curl -fsSL get.docker.com -o /tmp/get-docker.sh &&\
    sh /tmp/get-docker.sh

RUN go get -u golang.org/x/lint/golint &&\
    if [ `uname -m` != "aarch64" ]; then go get github.com/derekparker/delve/cmd/dlv; fi &&\
    go get github.com/tebeka/go2xunit &&\
    curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin v1.16.0

# Get integration test deps in here
COPY python/setup.py /tmp/
RUN pip3 install -e /tmp/
COPY tests/requirements.txt /tmp/
RUN pip3 install --upgrade pip==9.0.1 && pip3 install -r /tmp/requirements.txt
RUN wget -O /usr/bin/gomplate https://github.com/hairyhenderson/gomplate/releases/download/v2.4.0/gomplate_linux-${TARGET_ARCH}-slim &&\
    chmod +x /usr/bin/gomplate
RUN ln -s /usr/bin/python3 /usr/bin/python &&\
    ln -s /usr/bin/pip3 /usr/bin/pip

# COPY --from=final-image /bin/signalfx-agent ./signalfx-agent

# COPY --from=final-image / /bundle/
# COPY ./ ./

# RUN /bundle/bin/patch-interpreter /bundle