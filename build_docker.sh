#!/bin/bash
# Build and push Docker images

set -e

VERSION="1.1"
REGISTRY="git.uuxo.net/uuxo"
IMAGE_NAME="dbbackup"

echo "=== Building Docker Image ==="
echo "Version: $VERSION"
echo "Registry: $REGISTRY"
echo ""

# Build image
echo "Building image..."
docker build -t ${IMAGE_NAME}:${VERSION} -t ${IMAGE_NAME}:latest .

# Tag for registry
echo "Tagging for registry..."
docker tag ${IMAGE_NAME}:${VERSION} ${REGISTRY}/${IMAGE_NAME}:${VERSION}
docker tag ${IMAGE_NAME}:latest ${REGISTRY}/${IMAGE_NAME}:latest

# Show images
echo ""
echo "Images built:"
docker images ${IMAGE_NAME}

echo ""
echo "✅ Build complete!"
echo ""
echo "To push to registry:"
echo "  docker push ${REGISTRY}/${IMAGE_NAME}:${VERSION}"
echo "  docker push ${REGISTRY}/${IMAGE_NAME}:latest"
echo ""
echo "To test locally:"
echo "  docker run --rm ${IMAGE_NAME}:latest --version"
echo "  docker run --rm -it ${IMAGE_NAME}:latest interactive"
