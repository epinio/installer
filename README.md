# epinio-installer

A declarative installer for Kubernetes that can install Epinio and its dependencies.
This is intended to be used by Epinio developers and CI.

## Usage

    # edit manifest.yaml, then:
    epinio-installer install --trace-level 1 -m assets/examples/manifest.yaml

## Building

    go build -o epinio-installer cmd/epinio-installer/main.go
    # or
    goreleaser build --single-target --snapshot --rm-dist

## Releasing

Push a tag that is prefixed with 'v', to trigger releases.

Build the container image locally:

    docker build -t epinio-installer \
      --build-arg=HELM_VERSION=$HELM_VERSION \
      --build-arg=HELM_CHECKSUM=$HELM_CHECKSUM \
      --build-arg=KUBECTL_VERSION=$KUBECTL_VERSION \
      --build-arg=KUBECTL_CHECKSUM=$KUBECTL_CHECKSUM \
    .
