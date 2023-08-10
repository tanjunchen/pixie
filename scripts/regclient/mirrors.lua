local mirrors = {}

mirrors.sourceRegistry = "registry.baidubce.com/csm/pixie-prod"
mirrors.destinationRegistries = {
  "docker.io/pxio",
  "ghcr.io/pixie-io",
  "quay.io/pixie",
}

return mirrors
