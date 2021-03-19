FROM golang:1.16-alpine
LABEL maintainer="Leigh MacDonald <leigh.macdonald@gmail.com>"
WORKDIR /build
RUN apk add make git build-base yarn
COPY go.mod go.sum ./
COPY frontend/package.json frontend/package.json
COPY frontend/yarn.lock frontend/yarn.lock
RUN cd frontend && yarn
# Download all dependencies. Dependencies will be cached if the
# go.mod and go.sum files are not changed
RUN go mod download
COPY docker/docker_init.sh .
COPY . .
ENTRYPOINT ["sh", "/build/docker_init.sh"]
CMD ["make", "test"]
