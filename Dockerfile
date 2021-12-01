# Image used by the Epinio Wrapper Helm Chart
# Helm and kubectl must be installed in, that's why it differs from epinio-server image
ARG SERVER_IMAGE=registry.opensuse.org/opensuse/busybox
ARG DIST_BINARY=dist/epinio-installer_linux_amd64/epinio-installer


FROM opensuse/leap AS downloader
ARG HELM_VERSION
ARG HELM_CHECKSUM
ARG KUBECTL_VERSION
ARG KUBECTL_CHECKSUM

RUN zypper ref 
RUN zypper install --no-recommends -y wget tar gzip && rm -fr /var/cache/*
RUN wget https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz
RUN sh -c 'echo "${HELM_CHECKSUM} helm-v${HELM_VERSION}-linux-amd64.tar.gz" | sha256sum -w -c'
RUN tar xfz helm-v${HELM_VERSION}-linux-amd64.tar.gz linux-amd64/helm
RUN mv linux-amd64/helm /
RUN chmod +x /helm

RUN wget https://dl.k8s.io/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl
RUN sh -c 'echo "${KUBECTL_CHECKSUM} /kubectl" | sha256sum -w -c'
RUN chmod +x /kubectl


FROM $SERVER_IMAGE
COPY --from=downloader /helm /usr/local/bin/helm
COPY --from=downloader /kubectl /usr/local/bin/kubectl
# This works, because the image is built by goreleaser
COPY ${DIST_BINARY} /usr/local/bin/epinio-installer
COPY assets/installer /
