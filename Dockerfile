ARG BASE_IMAGE
ARG BASE_IMAGE_VERSION
FROM --platform=${TARGETPLATFORM} ${BASE_IMAGE}:${BASE_IMAGE_VERSION} AS builder
WORKDIR /go/src/admission-webhook-example
COPY . .

FROM --platform=${TARGETPLATFORM} alpine:3.17.2
COPY --from=builder /go/src/admission-webhook-example/bin/admission-webhook-example /usr/bin/admission-webhook-example
CMD ["/usr/bin/admission-webhook-example"]