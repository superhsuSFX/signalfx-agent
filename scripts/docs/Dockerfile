FROM ubuntu:18.04

RUN apt update -q &&\
    apt install -yq \
	  python3-pip \
	  git

RUN pip3 install \
      nltk==3.4.3 \
      pyyaml==5.1.2

WORKDIR /opt/agent
# Expected context path is the root of the agent repo
COPY docs/ ./docs/
COPY scripts/docs/ ./scripts/docs/
COPY internal/ ./internal/
