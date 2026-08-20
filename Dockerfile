# Dependabot tracks the version number here
ARG FFMPEG_VERSION=7
# GitHub Actions injects the variant (scratch, nvidia, etc.)
ARG VARIANT=scratch

# Combine them for the final base image
FROM jrottenberg/ffmpeg:${FFMPEG_VERSION}-${VARIANT}

COPY --chmod=755 gompeg /bin/

ENTRYPOINT ["/bin/gompeg"]
