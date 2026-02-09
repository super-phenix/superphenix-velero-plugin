FROM golang:1.25.7-trixie AS build

WORKDIR /go/src/github.com/super-phenix/superphenix-velero-plugin
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-w -s' -o /main cmd/main.go

FROM busybox:1.37.0 AS busybox
FROM scratch

COPY --from=build /main /superphenix-velero-plugin
COPY --from=busybox /bin/sh /bin/sh

USER 65532:65532
ENTRYPOINT ["/bin/sh", "-c", "ls / && cp /superphenix-velero-plugin /target/."]
