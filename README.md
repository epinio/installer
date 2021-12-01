# epinio-installer

A declarative installer for Kubernetes that can install Epinio and its dependencies.
This is intended to be used by Epinio developers and CI.

## Usage

    # edit manifest.yaml, then:
    epinio-installer install --trace-level 1 -m assets/installer/manifest.yaml 
  
## Building

    go build -o epinio-installer cmd/epinio-installer/main.go
    # or
    goreleaser build --single-target --snapshot --rm-dist
  
## Releasing

Push a tag that is prefixed with 'v', to trigger releases.
