variable "IMAGE" {
  default = "sub2api-custom:local-arm64"
}

variable "VERSION" {
  default = ""
}

variable "COMMIT" {
  default = "local"
}

variable "DATE" {
  default = ""
}

group "default" {
  targets = ["app"]
}

target "app" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/arm64"]
  tags       = [IMAGE]
  output     = ["type=docker"]
  args = {
    VERSION = VERSION
    COMMIT  = COMMIT
    DATE    = DATE
  }
  labels = {
    "org.opencontainers.image.revision" = COMMIT
    "org.opencontainers.image.version"  = VERSION
    "org.opencontainers.image.created"  = DATE
  }
}
