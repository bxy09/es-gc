#!bash
docker build -t builder -f Dockerfile.build . && docker run builder | docker build -t bxy09/es-gc:$TRAVIS_TAG -
mkdir -p ~/.docker
'echo "{\"auths\":{\"https://index.docker.io/v1/\": {\"auth\": \"$DOCKER_HUB_AUTH\"}}}" > ~/.docker/config.json'
docker push bxy09/es-gc:$TRAVIS_TAG
docker rmi -f builder
