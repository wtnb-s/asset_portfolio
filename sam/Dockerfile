FROM golang:1.15 as build-image
ARG BUILD_TARGET
WORKDIR /go/src/code/api/${BUILD_TARGET}
COPY code/ /go/src/code/
RUN go build -o /go/bin/buildfile

FROM public.ecr.aws/lambda/go:1

COPY --from=build-image /go/bin/ /var/task/

# Command can be overwritten by providing a different command in the template directly.
CMD ["buildfile"]
