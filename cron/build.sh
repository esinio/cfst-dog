podman build \
  --build-arg http_proxy=http://192.168.1.146:1087 \
  --build-arg https_proxy=http://192.168.1.146:1087 \
  -t localhost/cfst-cron .
