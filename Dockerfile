# Runtime image for edgex-foundry-mcp, built by GoReleaser (dockers_v2) from the
# release binaries. Speaks MCP over stdio; configure with EDGEX_* variables.
#   docker run -i --rm -e EDGEX_METADATA_URL=... ghcr.io/fabiohernandezru/edgex-foundry-mcp
FROM gcr.io/distroless/static-debian12:nonroot
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/edgex-foundry-mcp /usr/bin/edgex-foundry-mcp
USER nonroot:nonroot
ENTRYPOINT ["/usr/bin/edgex-foundry-mcp"]
