FROM gcr.io/distroless/base:latest
MAINTAINER Sebastien Spagnolo "sebastien.spagnolo@orange.com"
COPY /bin/galera-operator /bin/galera-operator
ENTRYPOINT ["/bin/galera-operator"]
CMD ["--help"]