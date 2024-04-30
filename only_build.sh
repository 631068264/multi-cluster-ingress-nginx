export TAG=1.0.0-dev
export REGISTRY=${REGISTRY:-ingress-controller}

DEV_IMAGE=${REGISTRY}/controller:${TAG}

make build image
docker tag "${REGISTRY}/controller:${TAG}" "${DEV_IMAGE}"

