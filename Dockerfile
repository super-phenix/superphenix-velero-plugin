FROM golang:1.25.5-trixie AS build

WORKDIR $GOPATH/src/$PROJECT/
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags '-w -s' -o /bin/main cmd/main.go

FROM busybox:1.37.0 AS busybox
FROM scratch

COPY --from=build /bin/main /plugins/kubeovn-velero-plugin
COPY --from=busybox /bin/cp /bin/cp

USER 65532:65532
ENTRYPOINT ["cp", "/plugins/kubeovn-velero-plugin", "/target/."]