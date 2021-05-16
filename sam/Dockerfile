FROM golang:1.15 as build-image
ARG BUILD_TARGET
WORKDIR /go/src
COPY code/go.mod code/${BUILD_TARGET}/main.go ./
RUN go build -o ../bin/buildfile

FROM public.ecr.aws/lambda/go:1

COPY --from=build-image /go/bin/ /var/task/

# Command can be overwritten by providing a different command in the template directly.
CMD ["buildfile"]
