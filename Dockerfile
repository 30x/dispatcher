FROM nginx:stable-alpine

MAINTAINER Adam Magaluk <AdamMagaluk@google.com>

ARG GIT_COMMIT

LABEL Description="A general purpose router for Kubernetes."
LABEL GitCommit=${GIT_COMMIT}

# Prepare the environment
RUN apk update \
  # Install the CA Certificates for SSL usage
  && apk add --no-cache ca-certificates \
  # Update the CA Certificates
  && update-ca-certificates \
  # Remove the nginx log symlinks to give more control over how logging is controlled
  && unlink /var/log/nginx/access.log \
  && unlink /var/log/nginx/error.log

COPY dispatcher /

CMD ["/dispatcher"]
